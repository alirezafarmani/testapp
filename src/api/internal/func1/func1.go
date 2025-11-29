package func1

import (
        "fmt"
        "log"
        "math/rand"
        "time"

        "api/internal/redis_gateway"
)

type Stats struct {
        SuccessfulKeys  int
        FailedKeys      int
        DurationSeconds float64
        KeysPerSecond   float64
        TotalBytes      int64
        Keys            []string
        Values          []string
}

func Func1Run(client *redis_gateway.Client) (*Stats, error) {
        log.Printf("[FUNC1] Starting stress test on Redis")

        const totalKeys = 5000
        const valueSize = 4096 // bytes

        stats := &Stats{}

        start := time.Now()
        keys := make([]string, 0, totalKeys)
        values := make([]string, 0, totalKeys)

        valueTemplate := make([]byte, valueSize)
        for i := range valueTemplate {
                valueTemplate[i] = byte('A' + rand.Intn(26))
        }
        baseValue := string(valueTemplate)

        for i := 0; i < totalKeys; i++ {
                key := fmt.Sprintf("func1:key:%d", i)
                val := fmt.Sprintf("%s-%d", baseValue, i)

                if err := client.Set(key, val); err != nil {
                        stats.FailedKeys++
                        log.Printf("[FUNC1] ERROR setting key %s: %v", key, err)
                } else {
                        stats.SuccessfulKeys++
                        stats.TotalBytes += int64(len(val))
                        keys = append(keys, key)
                        values = append(values, val)
                }
        }

        elapsed := time.Since(start).Seconds()
        stats.DurationSeconds = elapsed
        if elapsed > 0 {
                stats.KeysPerSecond = float64(stats.SuccessfulKeys) / elapsed
        }
        stats.Keys = keys
        stats.Values = values

        log.Printf("[FUNC1] Completed. success=%d failed=%d duration=%.2fs throughput=%.2f keys/s totalBytes=%d",
                stats.SuccessfulKeys, stats.FailedKeys, stats.DurationSeconds, stats.KeysPerSecond, stats.TotalBytes)

        return stats, nil
}
