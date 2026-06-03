package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
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

type SessionState struct {
	VisitorID  string    `json:"visitor_id"`
	StoreID    string    `json:"store_id"`
	SessionSeq int       `json:"session_seq"`
	EntryAt    time.Time `json:"entry_at"`
	LastZone   string    `json:"last_zone"`
	DwellMs    int64     `json:"dwell_ms"`
	Active     bool      `json:"active"`
	LastSeen   time.Time `json:"last_seen"`
}

type Service struct {
	cfg    config.ServiceConfig
	log    *slog.Logger
	kafka  *kafka.Client
	redis  *redispkg.Client
	states sync.Map
}

func main() {
	cfg := config.LoadService("session-service")
	cfg.HTTPPort = config.Get("HTTP_PORT", "8081")
	log := observability.InitLogger("session-service")

	rdb, err := redispkg.New(cfg.RedisURL)
	if err != nil {
		log.Error("redis connect", "err", err)
	}
	svc := &Service{
		cfg:   cfg,
		log:   log,
		kafka: kafka.NewClient(cfg.KafkaBrokers),
		redis: rdb,
	}

	mux := http.NewServeMux()
	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	mux.Handle("/metrics", observability.MetricsHandler())

	go func() {
		log.Info("session-service listening", "port", cfg.HTTPPort)
		_ = http.ListenAndServe(":"+cfg.HTTPPort, mux)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go svc.consume(ctx)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	cancel()
}

func (s *Service) consume(ctx context.Context) {
	reader := s.kafka.Reader(kafka.TopicStoreEvents, "session-service")
	defer reader.Close()

	writer := s.kafka.Writer(kafka.TopicSessionEvents)
	defer writer.Close()

	dashWriter := s.kafka.Writer(kafka.TopicDashboardEvents)
	defer dashWriter.Close()

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
			observability.EventsProcessed.WithLabelValues("session-service", kafka.TopicStoreEvents, "invalid").Inc()
			_ = reader.CommitMessages(ctx, msg)
			continue
		}

		out := s.processSession(ctx, &ev)
		for _, e := range out {
			_ = kafka.Publish(ctx, writer, e.VisitorID, e)
			_ = kafka.Publish(ctx, dashWriter, e.StoreID, e)
			if s.redis != nil {
				_ = s.redis.Publish(ctx, redispkg.PubSubDashboard, e)
			}
		}

		observability.EventsProcessed.WithLabelValues("session-service", kafka.TopicStoreEvents, "ok").Inc()
		_ = reader.CommitMessages(ctx, msg)
	}
}

func (s *Service) processSession(ctx context.Context, ev *events.StoreEvent) []*events.StoreEvent {
	var results []*events.StoreEvent
	if ev.EventType == events.EventReset {
		s.states.Range(func(key, _ any) bool {
			if strings.HasPrefix(key.(string), ev.StoreID+":") {
				s.states.Delete(key)
			}
			return true
		})
		return []*events.StoreEvent{cloneEvent(ev, events.EventReset)}
	}
	key := ev.StoreID + ":" + ev.VisitorID

	val, loaded := s.states.LoadOrStore(key, &SessionState{
		VisitorID:  ev.VisitorID,
		StoreID:    ev.StoreID,
		SessionSeq: 1,
	})
	state := val.(*SessionState)

	switch ev.EventType {
	case events.EventEntry:
		if !state.Active {
			state.Active = true
			state.EntryAt = ev.Timestamp
			state.SessionSeq++
			if loaded {
				// Re-entry detection: exited within REENTRY_WINDOW
				if time.Since(state.LastSeen) < 30*time.Minute && state.SessionSeq > 1 {
					re := cloneEvent(ev, events.EventReentry)
					re.Metadata.ReentryDetected = true
					re.Metadata.SessionSeq = state.SessionSeq
					re.Metadata.MovementPattern = "REENTRY"
					results = append(results, re)
				}
			}
			entry := cloneEvent(ev, events.EventEntry)
			entry.Metadata.SessionSeq = state.SessionSeq
			results = append(results, entry)
		}
		state.LastZone = ev.ZoneID
		state.LastSeen = ev.Timestamp
		state.DwellMs += ev.DwellMs

	case events.EventZoneEnter:
		if !state.Active {
			state.Active = true
			state.EntryAt = ev.Timestamp
			state.SessionSeq++
			entry := cloneEvent(ev, events.EventEntry)
			entry.Metadata.SessionSeq = state.SessionSeq
			results = append(results, entry)
		}
		state.LastZone = ev.ZoneID
		state.LastSeen = ev.Timestamp
		state.DwellMs += ev.DwellMs
		results = append(results, cloneEvent(ev, events.EventZoneEnter))

	case events.EventExit:
		state.Active = false
		state.LastSeen = ev.Timestamp
		state.DwellMs += ev.DwellMs
		exit := cloneEvent(ev, events.EventExit)
		exit.DwellMs = state.DwellMs
		exit.Metadata.SessionSeq = state.SessionSeq
		results = append(results, exit)

	case events.EventZoneExit:
		state.LastSeen = ev.Timestamp
		state.DwellMs += ev.DwellMs
		results = append(results, cloneEvent(ev, events.EventZoneExit))

	default:
		state.LastSeen = ev.Timestamp
		if ev.ZoneID != "" {
			state.LastZone = ev.ZoneID
		}
		results = append(results, cloneEvent(ev, ev.EventType))
	}

	s.states.Store(key, state)
	if s.redis != nil {
		_ = s.redis.SetJSON(ctx, redispkg.SessionKey(ev.StoreID, ev.VisitorID), state, 2*time.Hour)
	}
	return results
}

func cloneEvent(src *events.StoreEvent, typ string) *events.StoreEvent {
	return &events.StoreEvent{
		EventID:    uuid.New().String(),
		StoreID:    src.StoreID,
		CameraID:   src.CameraID,
		VisitorID:  src.VisitorID,
		EventType:  typ,
		Timestamp:  src.Timestamp,
		ZoneID:     src.ZoneID,
		DwellMs:    src.DwellMs,
		IsStaff:    src.IsStaff,
		Confidence: src.Confidence,
		Metadata:   src.Metadata,
	}
}
