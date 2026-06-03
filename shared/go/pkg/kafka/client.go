package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	TopicRawDetections  = "raw-detections"
	TopicStoreEvents    = "store-events"
	TopicSessionEvents  = "session-events"
	TopicMetricsEvents  = "metrics-events"
	TopicAnomalyEvents  = "anomaly-events"
	TopicDashboardEvents = "dashboard-events"
)

type Client struct {
	brokers []string
}

func NewClient(brokers string) *Client {
	return &Client{brokers: []string{brokers}}
}

func (c *Client) Writer(topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(c.brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
	}
}

func (c *Client) Reader(topic, groupID string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        c.brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        500 * time.Millisecond,
		CommitInterval: time.Second,
	})
}

func Publish(ctx context.Context, w *kafka.Writer, key string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return w.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: data,
		Time:  time.Now().UTC(),
	})
}

func AllTopics() []string {
	return []string{
		TopicRawDetections, TopicStoreEvents, TopicSessionEvents,
		TopicMetricsEvents, TopicAnomalyEvents, TopicDashboardEvents,
	}
}
