package usage
import (
	"context"
	"log"
	"runtime"
	"time"
	"api/internal/metrics"
)
func MonitorMemory(ctx context.Context, reg *metrics.Registry, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Printf("[USAGE] Starting runtime memory metrics monitor (interval: %v)", interval)
	for {
		select {
		case <-ctx.Done():
			log.Println("[USAGE] Stopping memory metrics monitor due to context cancellation")
			return
		case <-ticker.C:
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			numGoroutines := runtime.NumGoroutine()
			if reg != nil {
				reg.SetGauge("app_memory_usage_bytes", float64(m.Alloc), map[string]string{"type": "alloc"})
				reg.SetGauge("app_memory_usage_bytes", float64(m.TotalAlloc), map[string]string{"type": "total_alloc"})
				reg.SetGauge("app_memory_usage_bytes", float64(m.Sys), map[string]string{"type": "sys"})
				reg.SetGauge("app_memory_usage_bytes", float64(m.HeapAlloc), map[string]string{"type": "heap_alloc"})
				reg.SetGauge("app_memory_usage_bytes", float64(m.HeapInuse), map[string]string{"type": "heap_inuse"})
				reg.SetGauge("app_gc_runs_total", float64(m.NumGC), map[string]string{})
				reg.SetGauge("app_goroutines_total", float64(numGoroutines), map[string]string{})
			}
		}
	}
}
