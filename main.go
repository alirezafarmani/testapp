package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"api/internal/func1"
	"api/internal/func2"
	"api/internal/metrics"
	"api/internal/pg_gateway"
	"api/internal/redis_gateway"
	"api/internal/usage"
	"api/internal/users"
)

type SetRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type UserRequest struct {
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Age           int    `json:"age"`
	MaritalStatus bool   `json:"marital_status"`
}

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

var (
	metricsRegistry   *metrics.Registry
	loadedKeys        []string
	loadedKeysMutex   sync.RWMutex
	loadedValues      []string
	loadedValuesMutex sync.RWMutex
	usersManager      *users.UsersManager
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	log.Println("========================================")
	log.Println("APPLICATION STARTUP INITIATED")
	log.Println("========================================")

	log.Println("[INIT] Reading environment variables...")
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	log.Printf("[INIT] Redis configuration: host=%s, port=%s", redisHost, redisPort)

	pgHost := getEnv("POSTGRES_HOST", "localhost")
	pgPort := getEnv("POSTGRES_PORT", "5432")
	pgUser := getEnv("POSTGRES_USER", "appuser")
	pgPass := getEnv("POSTGRES_PASSWORD", "apppass")
	pgDB := getEnv("POSTGRES_DB", "appdb")
	log.Printf("[INIT] PostgreSQL configuration: host=%s, port=%s, user=%s, db=%s", pgHost, pgPort, pgUser, pgDB)

	log.Println("[INIT] Initializing metrics registry...")
	metricsRegistry = metrics.NewRegistry()
	log.Println("[INIT] Metrics registry initialized successfully")

	log.Printf("[REDIS] Attempting to connect to Redis at %s:%s...", redisHost, redisPort)
	startTime := time.Now()
	redisClient := redis_gateway.NewRedisClient(redisHost + ":" + redisPort)
	defer redisClient.Close()
	log.Printf("[REDIS] Connected successfully in %v", time.Since(startTime))
	redisClient.SetMetricsRegistry(metricsRegistry)
	log.Println("[REDIS] Metrics registry attached to Redis client")

	log.Printf("[POSTGRES] Attempting to connect to PostgreSQL at %s:%s...", pgHost, pgPort)
	startTime = time.Now()
	pgClient := pg_gateway.NewPGClient(pgHost, pgPort, pgUser, pgPass, pgDB)
	defer pgClient.Close()
	log.Printf("[POSTGRES] Connected successfully in %v", time.Since(startTime))
	pgClient.SetMetricsRegistry(metricsRegistry)
	log.Println("[POSTGRES] Metrics registry attached to PostgreSQL client")

	log.Println("[POSTGRES] Creating database table if not exists...")
	if err := pgClient.CreateTable(); err != nil {
		log.Printf("[POSTGRES] WARNING: Could not create table: %v", err)
	} else {
		log.Println("[POSTGRES] Table created/verified successfully")
	}

	log.Println("[MONITOR] Starting memory monitoring goroutine...")
	go usage.MonitorMemory(metricsRegistry)
	log.Println("[MONITOR] Memory monitoring started")

	log.Println("[MONITOR] Starting array keeper goroutine to prevent GC...")
	go keepArraysAlive()
	log.Println("[MONITOR] Array keeper started")

	log.Println("[MONITOR] Starting database connections keeper goroutine...")
	go func2.KeepConnectionsAlive()
	log.Println("[MONITOR] Database connections keeper started")

	log.Println("[MONITOR] Starting database connection keeper goroutine...")
	go func2.KeepConnectionsAlive()
	log.Println("[MONITOR] Database connection keeper started")

	metricsRegistry.SetGauge("redis_connection_status", 1, map[string]string{})
	metricsRegistry.SetGauge("postgres_connection_status", 1, map[string]string{})

	log.Println("[INIT] Creating UsersManager...")
	usersManager = users.NewUsersManager(redisClient, pgClient, metricsRegistry)
	log.Println("[INIT] UsersManager created successfully")

	// /api/user
	log.Println("[HTTP] Registering /api/user endpoint...")
	http.HandleFunc("/api/user", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		requestID := fmt.Sprintf("%d", time.Now().UnixNano())
		requestStart := time.Now()

		log.Printf("[USER:%s] Incoming %s request to /api/user from %s", requestID, r.Method, r.RemoteAddr)
		log.Printf("[USER:%s] Headers: %v", requestID, r.Header)

		if r.Method != http.MethodPost {
			log.Printf("[USER:%s] ERROR: Method not allowed: %s", requestID, r.Method)
			metricsRegistry.IncrementCounter("api_requests_total", map[string]string{
				"method": r.Method, "endpoint": "/api/user", "status": "405",
			})
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req UserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[USER:%s] ERROR: Failed to decode JSON body: %v", requestID, err)
			metricsRegistry.IncrementCounter("api_requests_total", map[string]string{
				"method": r.Method, "endpoint": "/api/user", "status": "400",
			})
			json.NewEncoder(w).Encode(Response{Success: false, Message: "Invalid request"})
			return
		}

		log.Printf("[USER:%s] Decoded payload: first_name='%s', last_name='%s', age=%d, marital_status=%t",
			requestID, req.FirstName, req.LastName, req.Age, req.MaritalStatus)

		userID, err := usersManager.CreateUser(req.FirstName, req.LastName, req.Age, req.MaritalStatus)
		if err != nil {
			log.Printf("[USER:%s] ERROR: Failed to create user: %v", requestID, err)
			metricsRegistry.IncrementCounter("api_requests_total", map[string]string{
				"method": r.Method, "endpoint": "/api/user", "status": "500",
			})
			json.NewEncoder(w).Encode(Response{Success: false, Message: err.Error()})
			return
		}

		metricsRegistry.IncrementCounter("api_requests_total", map[string]string{
			"method": r.Method, "endpoint": "/api/user", "status": "200",
		})
		metricsRegistry.IncrementCounter("user_created_total", map[string]string{})
		metricsRegistry.SetGauge("http_request_duration_seconds", time.Since(requestStart).Seconds(), map[string]string{
			"endpoint": "/api/user",
		})
		metricsRegistry.SetGauge("app_goroutines", float64(runtime.NumGoroutine()), map[string]string{})

		log.Printf("[USER:%s] SUCCESS: User created successfully (user_id: %s)", requestID, userID)
		log.Printf("[USER:%s] Sending response to client", requestID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "User created successfully",
			"user_id": userID,
		})
		log.Printf("[USER:%s] Request completed successfully", requestID)
	}))
	log.Println("[HTTP] /api/user endpoint registered")

	// /api/users
	log.Println("[HTTP] Registering /api/users endpoint...")
	http.HandleFunc("/api/users", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		requestID := fmt.Sprintf("%d", time.Now().UnixNano())
		requestStart := time.Now()

		log.Printf("[USERS:%s] Incoming %s request to /api/users from %s", requestID, r.Method, r.RemoteAddr)
		log.Printf("[USERS:%s] Headers: %v", requestID, r.Header)

		if r.Method != http.MethodGet {
			log.Printf("[USERS:%s] ERROR: Method not allowed: %s", requestID, r.Method)
			metricsRegistry.IncrementCounter("api_requests_total", map[string]string{
				"method": r.Method, "endpoint": "/api/users", "status": "405",
			})
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		log.Printf("[USERS:%s] Fetching all users...", requestID)
		usersList, err := usersManager.GetUsers()
		if err != nil {
			log.Printf("[USERS:%s] ERROR: Failed to get users: %v", requestID, err)
			metricsRegistry.IncrementCounter("api_requests_total", map[string]string{
				"method": r.Method, "endpoint": "/api/users", "status": "500",
			})
			json.NewEncoder(w).Encode(Response{Success: false, Message: err.Error()})
			return
		}

		metricsRegistry.IncrementCounter("api_requests_total", map[string]string{
			"method": r.Method, "endpoint": "/api/users", "status": "200",
		})
		metricsRegistry.SetGauge("http_request_duration_seconds", time.Since(requestStart).Seconds(), map[string]string{
			"endpoint": "/api/users",
		})
		metricsRegistry.SetGauge("app_goroutines", float64(runtime.NumGoroutine()), map[string]string{})

		log.Printf("[USERS:%s] SUCCESS: Retrieved %d users", requestID, len(usersList))
		log.Printf("[USERS:%s] Sending response to client", requestID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Retrieved %d users", len(usersList)),
			"count":   len(usersList),
			"users":   usersList,
		})
		log.Printf("[USERS:%s] Request completed successfully", requestID)
	}))
	log.Println("[HTTP] /api/users endpoint registered")

	// /api/set
	log.Println("[HTTP] Registering /api/set endpoint...")
	http.HandleFunc("/api/set", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		requestID := fmt.Sprintf("%d", time.Now().UnixNano())
		requestStart := time.Now()

		log.Printf("[REQUEST:%s] Incoming %s request to /api/set from %s", requestID, r.Method, r.RemoteAddr)
		log.Printf("[REQUEST:%s] Headers: %v", requestID, r.Header)

		if r.Method != http.MethodPost {
			log.Printf("[REQUEST:%s] ERROR: Method not allowed: %s", requestID, r.Method)
			metricsRegistry.IncrementCounter("api_requests_total", map[string]string{
				"method": r.Method, "endpoint": "/api/set", "status": "405",
			})
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req SetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[REQUEST:%s] ERROR: Failed to decode JSON body: %v", requestID, err)
			metricsRegistry.IncrementCounter("api_requests_total", map[string]stri
