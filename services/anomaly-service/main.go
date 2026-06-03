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
	"github.com/google/uuid"
)

type StoreWindow struct {
	QueueDepths    []int
	ZoneHits       map[string]int
	VisitorZones   map[string][]string
	LastEventAt    time.Time
	ConversionHist []float64
	mu             sync.Mutex
}

type Service struct {
	cfg     config.ServiceConfig
	log     *slog.Logger
	kafka   *kafka.Client
	redis   *redispkg.Client
	windows sync.Map
}

const (
	queueSpikeThreshold = 8
	deadZoneMinutes     = 15
	staleFeedSeconds    = 120
	repeatPatternCount  = 5
	conversionDropPct   = 0.4
)

func main() {
	cfg := config.LoadService("anomaly-service")
	cfg.HTTPPort = config.Get("HTTP_PORT", "8084")
	log := observability.InitLogger("anomaly-service")

	rdb, _ := redispkg.New(cfg.RedisURL)
	svc := &Service{cfg: cfg, log: log, kafka: kafka.NewClient(cfg.KafkaBrokers), redis: rdb}

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
	go svc.periodicChecks(ctx)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	cancel()
}

func (s *Service) consume(ctx context.Context) {
	reader := s.kafka.Reader(kafka.TopicMetricsEvents, "anomaly-service")
	defer reader.Close()
	writer := s.kafka.Writer(kafka.TopicAnomalyEvents)
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

		anomalies := s.detect(&ev)
		for _, a := range anomalies {
			_ = kafka.Publish(ctx, writer, a.StoreID, a)
			if s.redis != nil {
				_ = s.redis.Publish(ctx, redispkg.PubSubDashboard, a)
			}
		}
		observability.EventsProcessed.WithLabelValues("anomaly-service", kafka.TopicMetricsEvents, "ok").Inc()
		_ = reader.CommitMessages(ctx, msg)
	}
}

func (s *Service) getWindow(storeID string) *StoreWindow {
	v, _ := s.windows.LoadOrStore(storeID, &StoreWindow{
		ZoneHits:     make(map[string]int),
		VisitorZones: make(map[string][]string),
	})
	return v.(*StoreWindow)
}

func (s *Service) detect(ev *events.StoreEvent) []*events.StoreEvent {
	if ev.EventType == events.EventReset {
		s.windows.Delete(ev.StoreID)
		return nil
	}
	w := s.getWindow(ev.StoreID)
	w.mu.Lock()
	defer w.mu.Unlock()

	w.LastEventAt = time.Now()
	var out []*events.StoreEvent

	if ev.Metadata.QueueDepth >= queueSpikeThreshold {
		w.QueueDepths = append(w.QueueDepths, ev.Metadata.QueueDepth)
		out = append(out, s.anomaly(ev, "QUEUE_SPIKE", "HIGH", "Billing queue depth exceeded threshold"))
	}

	if ev.VisitorID != "" && ev.ZoneID != "" {
		zones := w.VisitorZones[ev.VisitorID]
		zones = append(zones, ev.ZoneID)
		if len(zones) > 20 {
			zones = zones[len(zones)-20:]
		}
		w.VisitorZones[ev.VisitorID] = zones
		if suspiciousPattern(zones) {
			out = append(out, s.anomaly(ev, "SUSPICIOUS_REPEAT", "MEDIUM", "Repeated zone oscillation detected"))
		}
	}

	if ev.Metadata.ConversionRate > 0 {
		w.ConversionHist = append(w.ConversionHist, ev.Metadata.ConversionRate)
		if len(w.ConversionHist) > 10 {
			w.ConversionHist = w.ConversionHist[1:]
		}
		if len(w.ConversionHist) >= 5 {
			avg := mean(w.ConversionHist[:len(w.ConversionHist)-1])
			last := w.ConversionHist[len(w.ConversionHist)-1]
			if avg > 0 && last/avg < (1-conversionDropPct) {
				out = append(out, s.anomaly(ev, "CONVERSION_DROP", "HIGH", "Conversion rate dropped significantly"))
			}
		}
	}

	return out
}

func (s *Service) periodicChecks(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	writer := s.kafka.Writer(kafka.TopicAnomalyEvents)
	defer writer.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.windows.Range(func(key, val any) bool {
				storeID := key.(string)
				w := val.(*StoreWindow)
				w.mu.Lock()
				defer w.mu.Unlock()

				if time.Since(w.LastEventAt) > staleFeedSeconds*time.Second {
					ev := s.anomaly(&events.StoreEvent{StoreID: storeID}, "STALE_FEED", "CRITICAL", "No events received — possible camera outage")
					_ = kafka.Publish(ctx, writer, storeID, ev)
				}

				totalHits := 0
				for _, c := range w.ZoneHits {
					totalHits += c
				}
				if totalHits == 0 && time.Since(w.LastEventAt) > deadZoneMinutes*time.Minute {
					ev := s.anomaly(&events.StoreEvent{StoreID: storeID}, "DEAD_ZONE", "LOW", "Store activity unusually low")
					_ = kafka.Publish(ctx, writer, storeID, ev)
				}
				return true
			})
		}
	}
}

func suspiciousPattern(zones []string) bool {
	if len(zones) < repeatPatternCount {
		return false
	}
	osc := 0
	for i := 1; i < len(zones); i++ {
		if zones[i] != zones[i-1] {
			osc++
		}
	}
	return osc >= repeatPatternCount-1
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range vals {
		s += v
	}
	return s / float64(len(vals))
}

func (s *Service) anomaly(src *events.StoreEvent, typ, severity, desc string) *events.StoreEvent {
	return &events.StoreEvent{
		EventID:    uuid.New().String(),
		StoreID:    src.StoreID,
		CameraID:   src.CameraID,
		VisitorID:  src.VisitorID,
		EventType:  events.EventAnomaly,
		Timestamp:  time.Now().UTC(),
		ZoneID:     src.ZoneID,
		Confidence: 0.95,
		Metadata: events.Metadata{
			AnomalyType:     typ,
			Severity:        severity,
			MovementPattern: desc,
		},
	}
}
