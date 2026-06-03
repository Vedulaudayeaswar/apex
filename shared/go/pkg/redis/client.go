package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *goredis.Client
}

func New(url string) (*Client, error) {
	opt, err := goredis.ParseURL(url)
	if err != nil {
		opt = &goredis.Options{Addr: "localhost:6379"}
	}
	rdb := goredis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return &Client{rdb: rdb}, nil // allow lazy connect in dev
	}
	return &Client{rdb: rdb}, nil
}

func (c *Client) RDB() *goredis.Client { return c.rdb }

func (c *Client) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, data, ttl).Err()
}

func (c *Client) GetJSON(ctx context.Context, key string, dest any) error {
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (c *Client) Publish(ctx context.Context, channel string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.rdb.Publish(ctx, channel, data).Err()
}

func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, key).Result()
}

func SessionKey(storeID, visitorID string) string {
	return fmt.Sprintf("session:%s:%s", storeID, visitorID)
}

func LeaderboardKey(storeID string) string {
	return fmt.Sprintf("leaderboard:%s", storeID)
}

func LiveMetricsKey(storeID string) string {
	return fmt.Sprintf("metrics:live:%s", storeID)
}

func QueueStateKey(storeID string) string {
	return fmt.Sprintf("queue:%s", storeID)
}

const PubSubDashboard = "dashboard:updates"
