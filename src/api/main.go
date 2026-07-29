package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api/internal/metrics"
	"api/internal/pg_gateway"
	"api/internal/redis_gateway"
	"api/internal/users"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)
type StructuredLog struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Component string                 `json:"component"`
	Duration  float64                `json:"duration_ms,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

type UserRequest struct {
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Age           int    `json:"age"`
	MaritalStatus bool   `json:"marital_status"`
}

func main() {
	log.SetFlags(0)
	writeLog("INFO", "Application startup initiated", "system", nil)

	// 1. Configs
	rabbitURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	queueName := getEnv("RABBITMQ_QUEUE", "user_tasks")
	metricsPort := getEnv("METRICS_PORT", "9090")

	// 2. Initializing Core Services
	reg := metrics.NewRegistry()
	redisClient := redis_gateway.NewRedisClient(getEnv("REDIS_HOST", "localhost") + ":6379")
	pgClient := pg_gateway.NewPGClient(
		getEnv("POSTGRES_HOST", "localhost"),
		getEnv("POSTGRES_PORT", "5432"),
		getEnv("POSTGRES_USER", "appuser"),
		getEnv("POSTGRES_PASSWORD", "apppass"),
		getEnv("POSTGRES_DB", "appdb"),
	)
	
	userManager := users.NewUsersManager(redisClient, pgClient, reg)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		writeLog("INFO", fmt.Sprintf("Prometheus exporter started on port %s", metricsPort), "monitoring", nil)
		if err := http.ListenAndServe(":"+metricsPort, nil); err != nil {
			writeLog("ERROR", "Metrics server failed", "monitoring", map[string]interface{}{"error": err.Error()})
		}
	}()

	// 4. RabbitMQ Connection
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		writeLog("FATAL", "Failed to connect to RabbitMQ", "rabbitmq", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		writeLog("FATAL", "Failed to open RabbitMQ channel", "rabbitmq", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(queueName, true, false, false, false, amqp.Table{"x-queue-type": "quorum"})
	if err != nil {
		writeLog("FATAL", "Failed to declare queue", "rabbitmq", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		writeLog("FATAL", "Failed to register consumer", "rabbitmq", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	// 5. Graceful Shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	writeLog("INFO", "Worker is ready and consuming messages", "worker", map[string]interface{}{"queue": queueName})

	go func() {
		for d := range msgs {
			start := time.Now()
			var req UserRequest
			
			if err := json.Unmarshal(d.Body, &req); err != nil {
				writeLog("ERROR", "Invalid JSON payload", "worker", map[string]interface{}{"payload": string(d.Body)})
				d.Ack(false) 
				continue
			}

			userID, err := userManager.CreateUser(req.FirstName, req.LastName, req.Age, req.MaritalStatus)
			duration := float64(time.Since(start).Milliseconds())

			if err != nil {
				writeLog("ERROR", "Failed to process user", "worker", map[string]interface{}{
					"error": err.Error(),
					"user":  req.LastName,
				})
				d.Nack(false, true) 
			} else {
				writeLog("INFO", "User processed successfully", "worker", map[string]interface{}{
					"user_id":     userID,
					"duration_ms": duration,
				})
				d.Ack(false)
				reg.IncrementCounter("processed_users_total", nil)
			}
		}
	}()

	<-sigChan
	writeLog("INFO", "Shutting down worker gracefully...", "system", nil)
}
func writeLog(level, message, component string, ctx map[string]interface{}) {
	entry := StructuredLog{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		Message:   message,
		Component: component,
		Context:   ctx,
	}
	jsonBytes, _ := json.Marshal(entry)
	fmt.Println(string(jsonBytes))
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
