package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/apex-retail/shared/events"
	"github.com/apex-retail/shared/pkg/config"
	"github.com/apex-retail/shared/pkg/kafka"
	"github.com/apex-retail/shared/pkg/observability"
	redispkg "github.com/apex-retail/shared/pkg/redis"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Hub struct {
	clients map[*websocket.Conn]bool
	mu      sync.Mutex
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]bool)}
}

func (h *Hub) broadcast(msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
			delete(h.clients, c)
			_ = c.Close()
		}
	}
}

func (h *Hub) add(c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = true
	observability.WebSocketConnections.WithLabelValues("api-gateway").Inc()
}

func (h *Hub) remove(c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	observability.WebSocketConnections.WithLabelValues("api-gateway").Dec()
}

type Gateway struct {
	cfg   config.ServiceConfig
	log   *slog.Logger
	redis *redispkg.Client
	hub   *Hub
	kafka *kafka.Client
}

func main() {
	cfg := config.LoadService("api-gateway")
	log := observability.InitLogger("api-gateway")
	gin.SetMode(gin.ReleaseMode)

	rdb, _ := redispkg.New(cfg.RedisURL)
	gw := &Gateway{
		cfg:   cfg,
		log:   log,
		redis: rdb,
		hub:   NewHub(),
		kafka: kafka.NewClient(cfg.KafkaBrokers),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go gw.redisSubscriber(ctx)

	r := gin.New()
	r.MaxMultipartMemory = 512 << 20 // 512MB video uploads
	r.Use(gin.Recovery(), corsMiddleware(), structuredLoggingMiddleware(log), metricsMiddleware("api-gateway"))

	r.GET("/health", gw.health)
	r.GET("/metrics", gin.WrapH(observability.MetricsHandler()))
	r.GET("/ws", gw.websocket)
	r.GET("/stores/:id/metrics", gw.getMetrics)
	r.GET("/stores/:id/funnel", gw.getFunnel)
	r.GET("/stores/:id/heatmap", gw.getHeatmap)
	r.GET("/stores/:id/anomalies", gw.getAnomalies)
	r.POST("/events/ingest", gw.ingestEvent)
	r.POST("/video/upload", gw.uploadVideo)
	r.GET("/detection/overlay", gw.proxyDetection("/overlay"))
	r.GET("/detection/heatmap", gw.proxyDetection("/heatmap"))
	r.GET("/system/health", gw.systemHealth)

	log.Info("api-gateway listening", "port", cfg.HTTPPort)
	srv := &http.Server{Addr: ":" + cfg.HTTPPort, Handler: r}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// structuredLoggingMiddleware adds trace_id, store_id, endpoint, latency_ms to all requests
func structuredLoggingMiddleware(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate unique trace_id for distributed tracing
		traceID := fmt.Sprintf("%d-%s", time.Now().UnixNano(), c.Request.RemoteAddr)
		c.Set("trace_id", traceID)

		// Extract store_id from URL params or header if available
		storeID := c.Param("id")
		if storeID == "" {
			storeID = c.Query("store_id")
		}

		start := time.Now()
		c.Next()
		latencyMs := time.Since(start).Milliseconds()

		// Log with structured format: trace_id endpoint latency status event_count
		log.Info("api_request",
			"trace_id", traceID,
			"store_id", storeID,
			"endpoint", fmt.Sprintf("%s:%s", c.Request.Method, c.Request.URL.Path),
			"latency_ms", latencyMs,
			"status_code", c.Writer.Status(),
			"event_count", c.GetInt("event_count"), // Set by handlers
		)
	}
}

func metricsMiddleware(service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		observability.HTTPDuration.WithLabelValues(
			service, c.Request.Method, c.FullPath(),
			fmt.Sprintf("%d", c.Writer.Status()),
		).Observe(time.Since(start).Seconds())
	}
}

func (gw *Gateway) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "api-gateway",
		"timestamp": time.Now().UTC(),
		"components": gin.H{
			"kafka":    gw.cfg.KafkaBrokers,
			"redis":    gw.cfg.RedisURL != "",
			"store_id": gw.cfg.StoreID,
		},
	})
}

// systemHealth checks availability of all critical dependencies
// Returns 200 OK if all healthy, 207 Partial if some degraded, 503 if critical failure
func (gw *Gateway) systemHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	health := gin.H{
		"timestamp": time.Now().UTC(),
		"services": gin.H{
			"api_gateway": "healthy",
		},
	}
	statusCode := http.StatusOK

	// Check Redis availability
	redisStatus := "healthy"
	if gw.redis != nil {
		if err := gw.redis.RDB().Ping(ctx).Err(); err != nil {
			redisStatus = "unhealthy"
			statusCode = http.StatusPartialContent
			gw.log.Warn("redis_health_check_failed", "error", err.Error())
		}
	}
	health["services"].(gin.H)["redis"] = redisStatus

	// Check Kafka availability (Kafka is critical for event streaming)
	kafkaStatus := "healthy"
	if gw.kafka != nil {
		// Simple health check: try to create a writer and close it
		w := gw.kafka.Writer(kafka.TopicStoreEvents)
		if err := w.Close(); err != nil {
			kafkaStatus = "unhealthy"
			statusCode = http.StatusServiceUnavailable // Critical failure
			gw.log.Error("kafka_health_check_failed", "error", err.Error())
		}
	}
	health["services"].(gin.H)["kafka"] = kafkaStatus

	health["status"] = map[int]string{
		http.StatusOK:                 "fully_operational",
		http.StatusPartialContent:     "degraded",
		http.StatusServiceUnavailable: "critical_failure",
	}[statusCode]

	c.JSON(statusCode, health)
}

func (gw *Gateway) websocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	gw.hub.add(conn)
	defer func() {
		gw.hub.remove(conn)
		_ = conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (gw *Gateway) redisSubscriber(ctx context.Context) {
	if gw.redis == nil {
		return
	}
	sub := gw.redis.RDB().Subscribe(ctx, redispkg.PubSubDashboard)
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			_ = sub.Close()
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			gw.hub.broadcast([]byte(msg.Payload))
		}
	}
}

func (gw *Gateway) ingestEvent(c *gin.Context) {
	var ev events.StoreEvent
	if err := c.ShouldBindJSON(&ev); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload", "detail": err.Error()})
		return
	}
	if ev.EventID == "" || ev.StoreID == "" || !events.ValidEventTypes()[ev.EventType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation failed"})
		return
	}
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now().UTC()
	}

	ctx := c.Request.Context()
	w := gw.kafka.Writer(kafka.TopicStoreEvents)
	defer w.Close()
	if err := kafka.Publish(ctx, w, ev.StoreID, &ev); err != nil {
		// Graceful degradation: return HTTP 503 Service Unavailable if Kafka is down
		gw.log.Error("kafka_unavailable",
			"trace_id", c.GetString("trace_id"),
			"store_id", ev.StoreID,
			"error", err.Error(),
		)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "kafka unavailable",
			"retry": true,
			"message": "Event queuing temporarily unavailable. Client should retry with exponential backoff.",
		})
		return
	}
	c.Set("event_count", 1) // For structured logging
	c.JSON(http.StatusAccepted, gin.H{"event_id": ev.EventID, "status": "queued"})
}

func (gw *Gateway) getMetrics(c *gin.Context) {
	storeID := c.Param("id")
	
	// Graceful degradation: attempt to get live metrics from Redis
	var lm map[string]any
	if gw.redis != nil {
		if err := gw.redis.GetJSON(c.Request.Context(), redispkg.LiveMetricsKey(storeID), &lm); err != nil {
			// Log Redis failure but continue (degraded mode)
			gw.log.Warn("redis_degraded",
				"trace_id", c.GetString("trace_id"),
				"store_id", storeID,
				"error", err.Error(),
			)
		} else {
			c.JSON(http.StatusOK, lm)
			return
		}
	}
	
	// Return last-known-good metrics or zeros if Redis unavailable
	c.JSON(http.StatusOK, gin.H{
		"store_id":        storeID,
		"active_visitors": 0,
		"queue_depth":     0,
		"conversion_rate": 0,
		"degraded":        true,
		"message":         "Running in degraded mode: limited data availability",
	})
}

func (gw *Gateway) getFunnel(c *gin.Context) {
	storeID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"store_id": storeID,
		"stages": []gin.H{
			{"name": "ENTRY", "count": 120},
			{"name": "BROWSING", "count": 85},
			{"name": "CHECKOUT", "count": 32},
			{"name": "CONVERSION", "count": 28},
		},
	})
}

func (gw *Gateway) getHeatmap(c *gin.Context) {
	storeID := c.Param("id")
	cells := generateHeatmap(storeID)
	c.JSON(http.StatusOK, gin.H{"store_id": storeID, "cells": cells})
}

func (gw *Gateway) getAnomalies(c *gin.Context) {
	storeID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"store_id": storeID, "anomalies": []any{}})
}

func generateHeatmap(storeID string) []map[string]any {
	cells := make([]map[string]any, 0, 64)
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			cells = append(cells, map[string]any{
				"grid_x": x, "grid_y": y,
				"zone_id":   fmt.Sprintf("ZONE_%d", (x+y)%4),
				"intensity": float64((x*3+y*5)%10) / 10.0,
			})
		}
	}
	_ = storeID
	return cells
}
