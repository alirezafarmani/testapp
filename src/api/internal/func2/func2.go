package func2

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/lib/pq"
)

type Stats struct {
	SuccessfulConnections int     `json:"successful_connections"`
	DurationSeconds       float64 `json:"duration_seconds"`
	AverageLatencySeconds float64 `json:"average_latency_seconds"`
}

var activeConnections int32

func GetActiveConnectionsCount() int32 {
	return atomic.LoadInt32(&activeConnections)
}

func KeepConnectionsAlive(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	iter := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("[Func-2-KEEPER] Terminating connection keeper loop")
			return
		case <-ticker.C:
			iter++
			log.Printf("[Func-2-KEEPER] Iteration #%d - Active database connections count: %d",
				iter, GetActiveConnectionsCount())
		}
	}
}
func Func2Run(host, port, user, pass, dbName string, connCount int) (*Stats, error) {
	if connCount <= 0 {
		connCount = 50
	}

	log.Printf("[FUNC2] Starting optimized PostgreSQL connection storm (%d concurrent attempts)...", connCount)

	stats := &Stats{}
	start := time.Now()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var latencies []float64

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable connect_timeout=5",
		host, port, user, pass, dbName)

	for i := 0; i < connCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			connStart := time.Now()
			db, err := sql.Open("postgres", dsn)
			if err != nil {
				if idx < 5 { 
					log.Printf("[FUNC2] ERROR opening connection #%d: %v", idx, err)
				}
				return
			}
			defer db.Close()
			if err := db.PingContext(ctx); err != nil {
				if idx < 5 {
					log.Printf("[FUNC2] ERROR pinging connection #%d: %v", idx, err)
				}
				return
			}

			latency := time.Since(connStart).Seconds()
			atomic.AddInt32(&activeConnections, 1)
			mu.Lock()
			latencies = append(latencies, latency)
			stats.SuccessfulConnections++
			mu.Unlock()
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
			}
			atomic.AddInt32(&activeConnections, -1)
		}(i)
	}

	wg.Wait()

	total := time.Since(start).Seconds()
	stats.DurationSeconds = total

	if len(latencies) > 0 {
		sum := 0.0
		for _, l := range latencies {
			sum += l
		}
		stats.AverageLatencySeconds = sum / float64(len(latencies))
	}

	log.Printf("[FUNC2] Completed connection storm. successful=%d/%d duration=%.2fs avgLatency=%.4fs",
		stats.SuccessfulConnections, connCount, stats.DurationSeconds, stats.AverageLatencySeconds)

	return stats, nil
}
