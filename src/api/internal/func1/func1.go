package func1

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"api/internal/redis_gateway"
)
type Stats struct {
	SuccessfulKeys  int           `json:"successful_keys"`
	FailedKeys      int           `json:"failed_keys"`
	DurationSeconds float64       `json:"duration_seconds"`
	KeysPerSecond   float64       `json:"keys_per_second"`
	TotalBytes      int64         `json:"total_bytes"`
	Keys            []string      `json:"keys,omitempty"`
	Values          []string      `json:"values,omitempty"`
}
type Func1Config struct {
	TotalKeys      int
	ValueSize      int
	KeyTTL         time.Duration 
	KeepValuesInRAM bool          
}
func Func1Run(client *redis_gateway.Client, cfg Func1Config) (*Stats, error) {
	log.Printf("[FUNC1] Starting optimized stress test on Redis (Keys: %d, TTL: %v)", cfg.TotalKeys, cfg.KeyTTL)

	
	if cfg.TotalKeys <= 0 {
		cfg.TotalKeys = 5000
	}
	if cfg.ValueSize <= 0 {
		cfg.ValueSize = 4096
	}
	if cfg.KeyTTL == 0 {
		cfg.KeyTTL = 5 * time.Minute
	}

	stats := &Stats{}
	start := time.Now()
	
	keys := make([]string, 0, cfg.TotalKeys)
	var values []string
	if cfg.KeepValuesInRAM {
		values = make([]string, 0, cfg.TotalKeys)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	
	valueTemplate := make([]byte, cfg.ValueSize)
	for i := range valueTemplate {
		valueTemplate[i] = byte('A' + r.Intn(26))
	}
	baseValue := string(valueTemplate)

	ctx := context.Background()

	for i := 0; i < cfg.TotalKeys; i++ {
		key := fmt.Sprintf("func1:key:%d", i)
		val := fmt.Sprintf("%s-%d", baseValue, i)
		var err error
		if err != nil {
			stats.FailedKeys++
			if stats.FailedKeys <= 5 {
				log.Printf("[FUNC1] ERROR setting key %s: %v", key, err)
			} else if stats.FailedKeys == 6 {
				log.Printf("[FUNC1] ERROR: Too many errors. Suppressing further error logs.")
			}
		} else {
			stats.SuccessfulKeys++
			stats.TotalBytes += int64(len(val))
			keys = append(keys, key)
			
			if cfg.KeepValuesInRAM {
				values = append(values, val)
			}
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
