package main

import (
	"context"
	"encoding/json"
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
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	cfg    config.ServiceConfig
	log    *slog.Logger
	kafka  *kafka.Client
	redis  *redispkg.Client
	db     *pgxpool.Pool
	seen   sync.Map
}

func main() {
	cfg := config.LoadService("ingestion-service")
	cfg.HTTPPort = config.Get("HTTP_PORT", "8082")
	log := observability.InitLogger("ingestion-service")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Warn("postgres deferred", "err", err)
	}

	rdb, _ := redispkg.New(cfg.RedisURL)
	svc := &Service{cfg: cfg, log: log, kafka: kafka.NewClient(cfg.KafkaBrokers), redis: rdb, db: pool}

	mux := http.NewServeMux()
	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	mux.Handle("/metrics", observability.MetricsHandler())
	go func() { _ = http.ListenAndServe(":"+cfg.HTTPPort, mux) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.consume(ctx)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	cancel()
}

func (s *Service) consume(ctx context.Context) {
	topics := []string{kafka.TopicStoreEvents, kafka.TopicSessionEvents, kafka.TopicAnomalyEvents}
	var wg sync.WaitGroup
	for _, topic := range topics {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			s.consumeTopic(ctx, t)
		}(topic)
	}
	wg.Wait()
}

func (s *Service) consumeTopic(ctx context.Context, topic string) {
	reader := s.kafka.Reader(topic, "ingestion-service-"+topic)
	defer reader.Close()

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

		if _, dup := s.seen.LoadOrStore(ev.EventID, true); dup {
			_ = reader.CommitMessages(ctx, msg)
			continue
		}

		if !events.ValidEventTypes()[ev.EventType] {
			observability.EventsProcessed.WithLabelValues("ingestion-service", topic, "invalid").Inc()
			_ = reader.CommitMessages(ctx, msg)
			continue
		}

		s.persist(ctx, &ev)
		s.updateRedis(ctx, &ev)
		observability.EventsProcessed.WithLabelValues("ingestion-service", topic, "ok").Inc()
		_ = reader.CommitMessages(ctx, msg)
	}
}

func (s *Service) persist(ctx context.Context, ev *events.StoreEvent) {
	if s.db == nil {
		return
	}
	meta, _ := json.Marshal(ev.Metadata)
	_, err := s.db.Exec(ctx, `
		INSERT INTO store_events (event_id, store_id, camera_id, visitor_id, event_type, timestamp, zone_id, dwell_ms, is_staff, confidence, metadata)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb)
		ON CONFLICT (event_id) DO NOTHING`,
		ev.EventID, ev.StoreID, ev.CameraID, ev.VisitorID, ev.EventType, ev.Timestamp,
		ev.ZoneID, ev.DwellMs, ev.IsStaff, ev.Confidence, meta,
	)
	if err != nil {
		s.log.Error("persist event", "err", err, "event_id", ev.EventID)
	}

	if ev.EventType == events.EventAnomaly {
		_, _ = s.db.Exec(ctx, `
			INSERT INTO anomalies (store_id, anomaly_type, severity, description, metadata, detected_at)
			VALUES ($1, $2, $3, $4, $5::jsonb, $6)`,
			ev.StoreID, ev.Metadata.AnomalyType, ev.Metadata.Severity,
			ev.Metadata.MovementPattern, meta, ev.Timestamp,
		)
	}
}

func (s *Service) updateRedis(ctx context.Context, ev *events.StoreEvent) {
	if s.redis == nil {
		return
	}
	_ = s.redis.Publish(ctx, redispkg.PubSubDashboard, ev)
}
