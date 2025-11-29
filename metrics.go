package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type metricType string

const (
	typeCounter metricType = "counter"
	typeGauge   metricType = "gauge"
)

type sample struct {
	value  float64
	labels map[string]string
}

type metric struct {
	name string
	mtyp metricType
	help string
	mu   sync.RWMutex
	data map[string]float64
}

type Registry struct {
	mu      sync.RWMutex
	metrics map[string]*metric
}

func NewRegistry() *Registry {
	return &Registry{
		metrics: make(map[string]*metric),
	}
}

func (r *Registry) getOrCreate(name string, mtyp metricType) *metric {
	r.mu.Lock()
	defer r.mu.Unlock()

	if m, ok := r.metrics[name]; ok {
		return m
	}

	m := &metric{
		name: name,
		mtyp: mtyp,
		data: make(map[string]float64),
	}
	r.metrics[name] = m
	return m
}

func labelsKey(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, labels[k]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func (r *Registry) IncrementCounter(name string, labels map[string]string) {
	m := r.getOrCreate(name, typeCounter)
	key := labelsKey(labels)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] += 1
}

func (r *Registry) SetGauge(name string, value float64, labels map[string]string) {
	m := r.getOrCreate(name, typeGauge)
	key := labelsKey(labels)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

func (r *Registry) Export() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder

	for _, m := range r.metrics {
		for labelKey, v := range m.data {
			line := fmt.Sprintf("%s%s %f\n", m.name, labelKey, v)
			sb.WriteString(line)
		}
	}

	return sb.String()
}

