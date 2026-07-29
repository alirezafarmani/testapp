package redis_gateway

import (
	"context"
	"fmt"
	"log"
	"time"

	"api/internal/metrics"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Addr         string
	Password     string
	DB           int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PingTimeout  time.Duration
	OpTimeout    time.Duration
	DefaultTTL   time.Duration
}

type Client struct {
	rc      *redis.Client
	metrics *metrics.Registry
	cfg     Config
}

func NewRedisClient(cfg Config) (*Client, error) {
	if cfg.PingTimeout == 0 {
		cfg.PingTimeout = 3 * time.Second
	}
	if cfg.OpTimeout == 0 {
		cfg.OpTimeout = 2 * time.Second
	}

	log.Printf("[REDIS] Creating client for address %s", cfg.Addr)

	rc := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), cfg.PingTimeout)
	defer cancel()

	if err := rc.Ping(ctx).Err(); err != nil {
		_ = rc.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &Client{
		rc:  rc,
		cfg: cfg,
	}, nil
}

func (c *Client) SetMetricsRegistry(reg *metrics.Registry) {
	c.metrics = reg
}

func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	start := time.Now()
	ctx, cancel := withTimeoutIfNone(ctx, c.cfg.OpTimeout)
	defer cancel()

	if ttl == 0 {
		ttl = c.cfg.DefaultTTL
	}

	err := c.rc.Set(ctx, key, value, ttl).Err()
	c.observeSet(err, time.Since(start))
	return err
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	start := time.Now()
	ctx, cancel := withTimeoutIfNone(ctx, c.cfg.OpTimeout)
	defer cancel()

	val, err := c.rc.Get(ctx, key).Result()
	c.observeGet(err, time.Since(start))
	return val, err
}

func (c *Client) Close() error {
	if err := c.rc.Close(); err != nil {
		log.Printf("[REDIS] ERROR closing client: %v", err)
		return err
	}
	return nil
}

func (c *Client) observeSet(err error, d time.Duration) {
	if c.metrics == nil {
		return
	}

	status := "success"
	if err != nil {
		status = "error"
	}

	c.metrics.IncrementCounter("redis_set_total", map[string]string{
		"status": status,
	})
	c.metrics.SetGauge("redis_set_duration_seconds", d.Seconds(), map[string]string{
		"stat": "last",
	})
}

func (c *Client) observeGet(err error, d time.Duration) {
	if c.metrics == nil {
		return
	}

	status := "hit"
	switch {
	case err == nil:
		status = "hit"
	case err == redis.Nil:
		status = "miss"
	default:
		status = "error"
	}

	c.metrics.IncrementCounter("redis_get_total", map[string]string{
		"status": status,
	})
	c.metrics.SetGauge("redis_get_duration_seconds", d.Seconds(), map[string]string{
		"stat": "last",
	})
}

func withTimeoutIfNone(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), d)
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}
