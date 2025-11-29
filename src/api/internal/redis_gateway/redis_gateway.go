package redis_gateway

import (
        "context"
        "log"
        "time"

        "api/internal/metrics"

        "github.com/redis/go-redis/v9"
)

type Client struct {
        rc      *redis.Client
        ctx     context.Context
        metrics *metrics.Registry
}

func NewRedisClient(addr string) *Client {
        log.Printf("[REDIS] Creating client for address %s", addr)

        rc := redis.NewClient(&redis.Options{
                Addr: addr,
        })

        ctx := context.Background()
        if err := rc.Ping(ctx).Err(); err != nil {
                log.Fatalf("[REDIS] Failed to ping Redis: %v", err)
        }

        return &Client{
                rc:  rc,
                ctx: ctx,
        }
}

func (c *Client) SetMetricsRegistry(reg *metrics.Registry) {
        c.metrics = reg
}

func (c *Client) Set(key, value string) error {
        start := time.Now()
        err := c.rc.Set(c.ctx, key, value, 0).Err()

        if c.metrics != nil {
                status := "success"
                if err != nil {
                        status = "error"
                }
                c.metrics.IncrementCounter("redis_set_total", map[string]string{
                        "status": status,
                })
                c.metrics.SetGauge("redis_set_duration_seconds", time.Since(start).Seconds(), map[string]string{})
        }

        return err
}

func (c *Client) Get(key string) (string, error) {
        start := time.Now()
        val, err := c.rc.Get(c.ctx, key).Result()

        if c.metrics != nil {
                status := "success"
                if err != nil {
                        status = "error"
                }
                c.metrics.IncrementCounter("redis_get_total", map[string]string{
                        "status": status,
                })
                c.metrics.SetGauge("redis_get_duration_seconds", time.Since(start).Seconds(), map[string]string{})
        }

        return val, err
}

func (c *Client) Close() {
        if err := c.rc.Close(); err != nil {
                log.Printf("[REDIS] ERROR closing client: %v", err)
        }
}
