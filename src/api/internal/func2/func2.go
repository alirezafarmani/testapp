package func2

import (
        "database/sql"
        "fmt"
        "log"
        "sync"
        "time"

        _ "github.com/lib/pq"
)

type Stats struct {
        SuccessfulConnections int
        DurationSeconds       float64
        AverageLatencySeconds float64
}

var (
        activeMu          sync.Mutex
        activeConnections int
)

func GetActiveConnectionsCount() int {
        activeMu.Lock()
        defer activeMu.Unlock()
        return activeConnections
}

func KeepConnectionsAlive() {
        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()

        iter := 0
        for range ticker.C {
                iter++
                log.Printf("[Func-2-KEEPER] Iteration #%d - Keeping %d database connections alive",
                        iter, GetActiveConnectionsCount())
        }
}

func Func2Run(host, port, user, pass, dbName string) (*Stats, error) {
        const connCount = 50

        log.Printf("[FUNC2] Starting PostgreSQL connection storm (%d connections)...", connCount)

        stats := &Stats{}
        start := time.Now()

        var wg sync.WaitGroup
        var mu sync.Mutex
        var latencies []float64

        dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
                host, port, user, pass, dbName)

        for i := 0; i < connCount; i++ {
                wg.Add(1)
                go func(idx int) {
                        defer wg.Done()

                        connStart := time.Now()
                        db, err := sql.Open("postgres", dsn)
                        if err != nil {
                                log.Printf("[FUNC2] ERROR opening connection #%d: %v", idx, err)
                                return
                        }
                        defer db.Close()

                        if err := db.Ping(); err != nil {
                                log.Printf("[FUNC2] ERROR pinging connection #%d: %v", idx, err)
                                return
                        }

                        latency := time.Since(connStart).Seconds()

                        activeMu.Lock()
                        activeConnections++
                        activeMu.Unlock()

                        mu.Lock()
                        latencies = append(latencies, latency)
                        stats.SuccessfulConnections++
                        mu.Unlock()

                        time.Sleep(2 * time.Second)

                        activeMu.Lock()
                        activeConnections--
                        activeMu.Unlock()
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

        log.Printf("[FUNC2] Completed. connections=%d duration=%.2fs avgLatency=%.4fs",
                stats.SuccessfulConnections, stats.DurationSeconds, stats.AverageLatencySeconds)

        return stats, nil
}
