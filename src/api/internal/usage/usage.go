package usage

import (
        "log"
        "runtime"
        "time"

        "api/internal/metrics"
)

func MonitorMemory(reg *metrics.Registry) {
        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()

        iter := 0
        for range ticker.C {
                iter++
                var m runtime.MemStats
                runtime.ReadMemStats(&m)

                log.Printf("[USAGE] Memory check iteration #%d", iter)
                log.Printf("[USAGE] Memory stats - Alloc: %d bytes (%.2f MB)", m.Alloc, float64(m.Alloc)/1024/1024)
                log.Printf("[USAGE] Memory stats - TotalAlloc: %d bytes (%.2f MB)", m.TotalAlloc, float64(m.TotalAlloc)/1024/1024)
                log.Printf("[USAGE] Memory stats - Sys: %d bytes (%.2f MB)", m.Sys, float64(m.Sys)/1024/1024)
                log.Printf("[USAGE] Memory stats - HeapAlloc: %d bytes (%.2f MB)", m.HeapAlloc, float64(m.HeapAlloc)/1024/1024)
                log.Printf("[USAGE] Memory stats - HeapInuse: %d bytes (%.2f MB)", m.HeapInuse, float64(m.HeapInuse)/1024/1024)
                log.Printf("[USAGE] Memory stats - NumGC: %d", m.NumGC)
                log.Printf("[USAGE] Memory stats - NumGoroutine: %d", runtime.NumGoroutine())

                if reg != nil {
                        reg.SetGauge("app_memory_usage_bytes", float64(m.Alloc), map[string]string{"type": "alloc"})
                        reg.SetGauge("app_memory_usage_bytes", float64(m.TotalAlloc), map[string]string{"type": "total_alloc"})
                        reg.SetGauge("app_memory_usage_bytes", float64(m.Sys), map[string]string{"type": "sys"})
                        reg.SetGauge("app_memory_usage_bytes", float64(m.HeapAlloc), map[string]string{"type": "heap_alloc"})
                        reg.SetGauge("app_memory_usage_bytes", float64(m.HeapInuse), map[string]string{"type": "heap_inuse"})
                        reg.SetGauge("app_gc_runs_total", float64(m.NumGC), map[string]string{})
                }
        }
}

#
