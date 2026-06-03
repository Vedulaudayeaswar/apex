package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/apex-retail/shared/events"
	"github.com/apex-retail/shared/pkg/config"
	"github.com/apex-retail/shared/pkg/kafka"
	"github.com/apex-retail/shared/pkg/observability"
	redispkg "github.com/apex-retail/shared/pkg/redis"
	"github.com/google/uuid"
)

type LiveMetrics struct {
	StoreID        string             `json:"store_id"`
	ActiveVisitors int                `json:"active_visitors"`
	QueueDepth     int                `json:"queue_depth"`
	ConversionRate float64            `json:"conversion_rate"`
	ZonePopularity map[string]int     `json:"zone_popularity"`
	Leaderboard    []LeaderboardEntry `json:"leaderboard"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

type LeaderboardEntry struct {
	ZoneID   string  `json:"zone_id"`
	Visits   int     `json:"visits"`
	AvgDwell float64 `json:"avg_dwell_ms"`
	Score    float64 `json:"score"`
}

type Service struct {
	cfg     config.ServiceConfig
	log     *slog.Logger
	kafka   *kafka.Client
	redis   *redispkg.Client
	metrics sync.Map
}

func main() {
	cfg := config.LoadService("metrics-service")
	cfg.HTTPPort = config.Get("HTTP_PORT", "8083")
	log := observability.InitLogger("metrics-service")

	rdb, _ := redispkg.New(cfg.RedisURL)
	svc := &Service{cfg: cfg, log: log, kafka: kafka.NewClient(cfg.KafkaBrokers), redis: rdb}

	mux := http.NewServeMux()
	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	mux.Handle("/metrics", observability.MetricsHandler())
	mux.HandleFunc("/stores/{id}/metrics", svc.httpMetrics)
	go func() { _ = http.ListenAndServe(":"+cfg.HTTPPort, mux) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.consume(ctx)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	cancel()
}

func (s *Service) httpMetrics(w http.ResponseWriter, r *http.Request) {
	storeID := r.URL.Path[len("/stores/"):]
	if i := len(storeID) - len("/metrics"); i > 0 {
		storeID = storeID[:i]
	}
	if v, ok := s.metrics.Load(storeID); ok {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func (s *Service) consume(ctx context.Context) {
	reader := s.kafka.Reader(kafka.TopicSessionEvents, "metrics-service")
	defer reader.Close()
	writer := s.kafka.Writer(kafka.TopicMetricsEvents)
	defer writer.Close()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			time.Sleep(time.Second)
			continue
		}

		var ev events.StoreEvent
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			_ = reader.CommitMessages(ctx, msg)
			continue
		}

		lm := s.aggregate(&ev)
		s.metrics.Store(ev.StoreID, lm)

		if s.redis != nil {
			_ = s.redis.SetJSON(ctx, redispkg.LiveMetricsKey(ev.StoreID), lm, 30*time.Second)
			_ = s.redis.SetJSON(ctx, redispkg.LeaderboardKey(ev.StoreID), lm.Leaderboard, 30*time.Second)
			_ = s.redis.Publish(ctx, redispkg.PubSubDashboard, lm)
		}

		metricEv := &events.StoreEvent{
			EventID:   uuid.New().String(),
			StoreID:   ev.StoreID,
			EventType: "METRICS_SNAPSHOT",
			Timestamp: time.Now().UTC(),
			Metadata: events.Metadata{
				ActiveVisitors: lm.ActiveVisitors,
				ConversionRate: lm.ConversionRate,
				QueueDepth:     lm.QueueDepth,
			},
		}
		if ev.EventType == events.EventReset {
			metricEv.EventType = events.EventReset
		}
		_ = kafka.Publish(ctx, writer, ev.StoreID, metricEv)
		observability.EventsProcessed.WithLabelValues("metrics-service", kafka.TopicSessionEvents, "ok").Inc()
		_ = reader.CommitMessages(ctx, msg)
	}
}

func (s *Service) aggregate(ev *events.StoreEvent) *LiveMetrics {
	var lm *LiveMetrics
	if v, ok := s.metrics.Load(ev.StoreID); ok {
		lm = v.(*LiveMetrics)
	} else {
		lm = &LiveMetrics{
			StoreID:        ev.StoreID,
			ZonePopularity: make(map[string]int),
		}
	}

	switch ev.EventType {
	case events.EventReset:
		lm = &LiveMetrics{
			StoreID:        ev.StoreID,
			ZonePopularity: make(map[string]int),
		}
	case events.EventEntry:
		lm.ActiveVisitors++
	case events.EventExit:
		if lm.ActiveVisitors > 0 {
			lm.ActiveVisitors--
		}
	case events.EventZoneEnter:
		lm.ZonePopularity[ev.ZoneID]++
	case events.EventQueueJoin:
		lm.QueueDepth = ev.Metadata.QueueDepth
	case events.EventDetection:
		lm.QueueDepth = ev.Metadata.QueueDepth
	}

	entries := 0
	total := 0
	lb := make([]LeaderboardEntry, 0, len(lm.ZonePopularity))
	for zone, count := range lm.ZonePopularity {
		total += count
		entries++
		lb = append(lb, LeaderboardEntry{
			ZoneID: zone,
			Visits: count,
			Score:  float64(count),
		})
	}
	lm.Leaderboard = lb
	sort.Slice(lm.Leaderboard, func(i, j int) bool {
		return lm.Leaderboard[i].Score > lm.Leaderboard[j].Score
	})
	if entries > 0 && lm.ActiveVisitors > 0 {
		lm.ConversionRate = math.Min(float64(total)/float64(lm.ActiveVisitors+1)*0.15, 0.99)
	}
	lm.UpdatedAt = time.Now().UTC()
	return lm
}
