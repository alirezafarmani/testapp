package pg_gateway

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"api/internal/metrics"

	_ "github.com/lib/pq"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string

	SSLMode string // 
	
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration

	PingTimeout  time.Duration
	QueryTimeout time.Duration
	ExecTimeout  time.Duration
}

type Client struct {
	db      *sql.DB
	metrics *metrics.Registry
	cfg     Config
}

func NewPGClient(cfg Config) (*Client, error) {
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 50
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 10
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = 5 * time.Minute
	}
	if cfg.PingTimeout == 0 {
		cfg.PingTimeout = 5 * time.Second
	}
	if cfg.QueryTimeout == 0 {
		cfg.QueryTimeout = 5 * time.Second
	}
	if cfg.ExecTimeout == 0 {
		cfg.ExecTimeout = 5 * time.Second
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)
	log.Printf("[POSTGRES] Opening connection (host=%s port=%s dbname=%s user=%s sslmode=%s)",
		cfg.Host, cfg.Port, cfg.DBName, cfg.User, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres open: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.PingTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	return &Client{db: db, cfg: cfg}, nil
}

func (c *Client) SetMetricsRegistry(reg *metrics.Registry) {
	c.metrics = reg
}

func (c *Client) Close() error {
	if c.db == nil {
		return nil
	}
	if err := c.db.Close(); err != nil {
		log.Printf("[POSTGRES] ERROR closing DB: %v", err)
		return err
	}
	return nil
}

func (c *Client) CreateTable(ctx context.Context) error {
	q := `
CREATE TABLE IF NOT EXISTS users (
    user_id    TEXT PRIMARY KEY,
    data       JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`
	ctx, cancel := withTimeoutIfNone(ctx, c.cfg.ExecTimeout)
	defer cancel()

	_, err := c.db.ExecContext(ctx, q)
	if err != nil {
		log.Printf("[POSTGRES] ERROR creating table: %v", err)
	}
	return err
}

func (c *Client) SaveUser(ctx context.Context, userID string, jsonData string) error {
	start := time.Now()
	ctx, cancel := withTimeoutIfNone(ctx, c.cfg.ExecTimeout)
	defer cancel()

	_, err := c.db.ExecContext(ctx, `
INSERT INTO users (user_id, data) VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET data = EXCLUDED.data
`, userID, jsonData)

	c.observe("pg_save_user", err, time.Since(start))
	return err
}

type StoredUser struct {
	UserID string `json:"user_id"`
	Data   string `json:"data"`
}

func (c *Client) GetUsers(ctx context.Context) ([]StoredUser, error) {
	start := time.Now()
	ctx, cancel := withTimeoutIfNone(ctx, c.cfg.QueryTimeout)
	defer cancel()

	rows, err := c.db.QueryContext(ctx, `SELECT user_id, data::text FROM users ORDER BY created_at DESC LIMIT 1000`)
	if err != nil {
		c.observe("pg_get_users", err, time.Since(start))
		return nil, err
	}
	defer rows.Close()

	users := make([]StoredUser, 0, 128)
	for rows.Next() {
		var u StoredUser
		if err := rows.Scan(&u.UserID, &u.Data); err != nil {
			c.observe("pg_get_users", err, time.Since(start))
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		c.observe("pg_get_users", err, time.Since(start))
		return nil, err
	}

	c.observe("pg_get_users", nil, time.Since(start))
	return users, nil
}
func (c *Client) observe(op string, err error, d time.Duration) {
	if c.metrics == nil {
		return
	}
	status := "success"
	if err != nil {
		status = "error"
	}
	c.metrics.IncrementCounter(op+"_total", map[string]string{"status": status})
	c.metrics.SetGauge(op+"_duration_seconds", d.Seconds(), map[string]string{"stat": "last"})
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
