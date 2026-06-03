package observability

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	EventsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "retail_events_processed_total",
		Help: "Total events processed",
	}, []string{"service", "topic", "status"})

	KafkaLag = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "retail_kafka_consumer_lag",
		Help: "Kafka consumer lag estimate",
	}, []string{"service", "topic"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "retail_http_request_duration_seconds",
		Help:    "HTTP request duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "method", "path", "status"})

	WebSocketConnections = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "retail_websocket_connections",
		Help: "Active WebSocket connections",
	}, []string{"service"})
)

func InitLogger(service string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})).With("service", service)
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
