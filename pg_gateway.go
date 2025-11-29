package pg_gateway

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"api/internal/metrics"

	_ "github.com/lib/pq"
)

type Client struct {
	db      *sql.DB
	metrics *metrics.Registry
}

func NewPGClient(host, port, user, pass, dbName string) *Client {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, pass, dbName)

	log.Printf("[POSTGRES] Opening connection with DSN: %s", dsn)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("[POSTGRES] Failed to open DB: %v", err)
	}

	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatalf("[POSTGRES] Failed to ping DB: %v", err)
	}

	return &Client{db: db}
}

func (c *Client) SetMetricsRegistry(reg *metrics.Registry) {
	c.metrics = reg
}

func (c *Client) Close() {
	if err := c.db.Close(); err != nil {
		log.Printf("[POSTGRES] ERROR closing DB: %v", err)
	}
}

func (c *Client) CreateTable() error {
	q := `
CREATE TABLE IF NOT EXISTS users (
    user_id    TEXT PRIMARY KEY,
    data       JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`
	_, err := c.db.Exec(q)
	if err != nil {
		log.Printf("[POSTGRES] ERROR creating table: %v", err)
	}
	return err
}

func (c *Client) SaveUser(userID string, jsonData string) error {
	start := time.Now()

	_, err := c.db.Exec(`
INSERT INTO users (user_id, data) VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET data = EXCLUDED.data
`, userID, jsonData)

	if c.metrics != nil {
		status := "success"
		if err != nil {
			status = "error"
		}
		c.metrics.IncrementCounter("pg_save_user_total", map[string]string{"status": status})
		c.metrics.SetGauge("pg_save_user_duration_seconds", time.Since(start).Seconds(), map[string]string{})
	}
	return err
}

type StoredUser struct {
	UserID string `json:"user_id"`
	Data   string `json:"data"`
}

func (c *Client) GetUsers() ([]StoredUser, error) {
	start := time.Now()
	rows, err := c.db.Query(`SELECT user_id, data::text FROM users ORDER BY created_at DESC LIMIT 1000`)
	if err != nil {
		if c.metrics != nil {
			c.metrics.IncrementCounter("pg_get_users_total", map[string]string{"status": "error"})
		}
		return nil, err
	}
	defer rows.Close()

	var users []StoredUser
	for rows.Next() {
		var u StoredUser
		if err := rows.Scan(&u.UserID, &u.Data); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	if c.metrics != nil {
		c.metrics.IncrementCounter("pg_get_users_total", map[string]string{"status": "success"})
		c.metrics.SetGauge("pg_get_users_duration_seconds", time.Since(start).Seconds(), map[string]string{})
	}

	return users, nil
}

