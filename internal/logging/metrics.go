// internal/logging/metrics.go
package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PerformanceMetrics tracks service performance metrics
type PerformanceMetrics struct {
	mu      sync.RWMutex
	metrics map[string]*ServiceMetrics
	storage *MetricsStorage
}

// ServiceMetrics represents metrics for a specific service
type ServiceMetrics struct {
	Service       string                 `json:"service"`
	ResponseTimes []ResponseTime         `json:"response_times,omitempty"`
	Throughput    Throughput             `json:"throughput"`
	ErrorRate     float64                `json:"error_rate"`
	Custom        map[string]interface{} `json:"custom,omitempty"`
	LastUpdated   time.Time              `json:"last_updated"`
}

// ResponseTime tracks API response times
type ResponseTime struct {
	Endpoint  string        `json:"endpoint"`
	Method    string        `json:"method"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
	Success   bool          `json:"success"`
}

// Throughput represents request throughput
type Throughput struct {
	RequestsPerSecond float64 `json:"requests_per_second"`
	BytesPerSecond    float64 `json:"bytes_per_second"`
	TotalRequests     uint64  `json:"total_requests"`
	TotalBytes        uint64  `json:"total_bytes"`
}

// MetricsStorage handles persistent storage of metrics
type MetricsStorage struct {
	path   string
	mu     sync.Mutex
	writer *RotatingWriter
}

// NewPerformanceMetrics creates a new performance metrics tracker
func NewPerformanceMetrics() (*PerformanceMetrics, error) {
	metricsDir := filepath.Join(".localcloud", "metrics")
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metrics directory: %w", err)
	}

	storage, err := NewMetricsStorage(metricsDir)
	if err != nil {
		return nil, err
	}

	pm := &PerformanceMetrics{
		metrics: make(map[string]*ServiceMetrics),
		storage: storage,
	}

	// Start metrics aggregation
	go pm.aggregateLoop()

	return pm, nil
}

// NewMetricsStorage creates a new metrics storage
func NewMetricsStorage(dir string) (*MetricsStorage, error) {
	path := filepath.Join(dir, "performance.json")
	writer, err := NewRotatingWriter(path, 50*1024*1024) // 50MB
	if err != nil {
		return nil, err
	}

	return &MetricsStorage{
		path:   path,
		writer: writer,
	}, nil
}

// RecordResponseTime records an API response time
func (pm *PerformanceMetrics) RecordResponseTime(service, endpoint, method string, duration time.Duration, success bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.metrics[service]; !exists {
		pm.metrics[service] = &ServiceMetrics{
			Service:       service,
			ResponseTimes: []ResponseTime{},
			Custom:        make(map[string]interface{}),
		}
	}

	rt := ResponseTime{
		Endpoint:  endpoint,
		Method:    method,
		Duration:  duration,
		Timestamp: time.Now(),
		Success:   success,
	}

	metrics := pm.metrics[service]
	metrics.ResponseTimes = append(metrics.ResponseTimes, rt)
	metrics.LastUpdated = time.Now()

	// Keep only last 1000 response times
	if len(metrics.ResponseTimes) > 1000 {
		metrics.ResponseTimes = metrics.ResponseTimes[len(metrics.ResponseTimes)-1000:]
	}

	// Update error rate
	pm.updateErrorRate(service)
}

// UpdateThroughput updates throughput metrics
func (pm *PerformanceMetrics) UpdateThroughput(service string, requests, bytes uint64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.metrics[service]; !exists {
		pm.metrics[service] = &ServiceMetrics{
			Service: service,
			Custom:  make(map[string]interface{}),
		}
	}

	metrics := pm.metrics[service]
	metrics.Throughput.TotalRequests += requests
	metrics.Throughput.TotalBytes += bytes
	metrics.LastUpdated = time.Now()
}

// SetCustomMetric sets a custom metric value
func (pm *PerformanceMetrics) SetCustomMetric(service, key string, value interface{}) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.metrics[service]; !exists {
		pm.metrics[service] = &ServiceMetrics{
			Service: service,
			Custom:  make(map[string]interface{}),
		}
	}

	pm.metrics[service].Custom[key] = value
	pm.metrics[service].LastUpdated = time.Now()
}

// GetMetrics returns current metrics for a service
func (pm *PerformanceMetrics) GetMetrics(service string) (*ServiceMetrics, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	metrics, exists := pm.metrics[service]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	metricsCopy := *metrics
	return &metricsCopy, true
}

// GetAllMetrics returns metrics for all services
func (pm *PerformanceMetrics) GetAllMetrics() map[string]*ServiceMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[string]*ServiceMetrics)
	for k, v := range pm.metrics {
		metricsCopy := *v
		result[k] = &metricsCopy
	}

	return result
}

// updateErrorRate calculates error rate from recent response times
func (pm *PerformanceMetrics) updateErrorRate(service string) {
	metrics := pm.metrics[service]
	if len(metrics.ResponseTimes) == 0 {
		metrics.ErrorRate = 0
		return
	}

	// Calculate error rate from last 100 requests
	start := len(metrics.ResponseTimes) - 100
	if start < 0 {
		start = 0
	}

	recent := metrics.ResponseTimes[start:]
	errors := 0
	for _, rt := range recent {
		if !rt.Success {
			errors++
		}
	}

	metrics.ErrorRate = float64(errors) / float64(len(recent))
}

// aggregateLoop periodically aggregates and persists metrics
func (pm *PerformanceMetrics) aggregateLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		pm.aggregate()
		pm.persist()
	}
}

// aggregate calculates aggregate metrics
func (pm *PerformanceMetrics) aggregate() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	for _, metrics := range pm.metrics {
		// Calculate requests per second
		if len(metrics.ResponseTimes) > 0 {
			duration := now.Sub(metrics.ResponseTimes[0].Timestamp)
			if duration > 0 {
				metrics.Throughput.RequestsPerSecond = float64(len(metrics.ResponseTimes)) / duration.Seconds()
			}
		}

		// Calculate average response times by endpoint
		endpointStats := make(map[string]struct {
			totalDuration time.Duration
			count         int
		})

		for _, rt := range metrics.ResponseTimes {
			key := fmt.Sprintf("%s:%s", rt.Method, rt.Endpoint)
			stats := endpointStats[key]
			stats.totalDuration += rt.Duration
			stats.count++
			endpointStats[key] = stats
		}

		// Store average response times in custom metrics
		for endpoint, stats := range endpointStats {
			avgDuration := stats.totalDuration / time.Duration(stats.count)
			metrics.Custom[fmt.Sprintf("avg_response_time_%s", endpoint)] = avgDuration.Milliseconds()
		}
	}
}

// persist saves metrics to storage
func (pm *PerformanceMetrics) persist() {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, metrics := range pm.metrics {
		pm.storage.Write(metrics)
	}
}

// Write writes metrics to storage
func (ms *MetricsStorage) Write(metrics *ServiceMetrics) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	return ms.writer.WriteJSON(metrics)
}

// StartHTTPCollector starts an HTTP metrics collector for the AI service
func StartHTTPCollector(port int, metrics *PerformanceMetrics) {
	// Create a middleware that tracks response times
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		wrapper := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Pass through to actual handler (this would be the real AI service)
		// For now, just return 200 OK
		wrapper.WriteHeader(http.StatusOK)
		wrapper.Write([]byte("OK"))

		// Record metrics
		duration := time.Since(start)
		success := wrapper.statusCode >= 200 && wrapper.statusCode < 400

		metrics.RecordResponseTime("ai", r.URL.Path, r.Method, duration, success)
		metrics.UpdateThroughput("ai", 1, uint64(wrapper.bytesWritten))
	})

	// Start metrics endpoint
	go func() {
		http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			allMetrics := metrics.GetAllMetrics()
			json.NewEncoder(w).Encode(allMetrics)
		})

		http.ListenAndServe(fmt.Sprintf(":%d", port+1000), nil)
	}()

	// This is just an example - in reality, this would integrate with the actual service
	_ = handler
}

// responseWriter wraps http.ResponseWriter to capture metrics
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(data)
	rw.bytesWritten += n
	return n, err
}

// CalculatePerformanceStats calculates performance statistics
func CalculatePerformanceStats(metrics *ServiceMetrics) map[string]interface{} {
	stats := make(map[string]interface{})

	if len(metrics.ResponseTimes) == 0 {
		return stats
	}

	// Calculate percentiles
	durations := make([]time.Duration, len(metrics.ResponseTimes))
	for i, rt := range metrics.ResponseTimes {
		durations[i] = rt.Duration
	}

	// Sort durations for percentile calculation
	for i := 0; i < len(durations)-1; i++ {
		for j := i + 1; j < len(durations); j++ {
			if durations[i] > durations[j] {
				durations[i], durations[j] = durations[j], durations[i]
			}
		}
	}

	// Calculate percentiles
	p50 := durations[len(durations)*50/100]
	p95 := durations[len(durations)*95/100]
	p99 := durations[len(durations)*99/100]

	stats["p50_ms"] = p50.Milliseconds()
	stats["p95_ms"] = p95.Milliseconds()
	stats["p99_ms"] = p99.Milliseconds()
	stats["error_rate_percent"] = metrics.ErrorRate * 100
	stats["requests_per_second"] = metrics.Throughput.RequestsPerSecond

	return stats
}

// AI Service specific metrics
func CollectAIMetrics(ctx context.Context, metrics *PerformanceMetrics) {
	// Example of collecting AI-specific metrics
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	httpClient := &http.Client{Timeout: 5 * time.Second}

	for {
		select {
		case <-ticker.C:
			// Query Ollama API for model info
			resp, err := httpClient.Get("http://localhost:11434/api/tags")
			if err == nil {
				var data struct {
					Models []struct {
						Name string `json:"name"`
						Size int64  `json:"size"`
					} `json:"models"`
				}

				if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
					metrics.SetCustomMetric("ai", "models_loaded", len(data.Models))

					totalSize := int64(0)
					for _, model := range data.Models {
						totalSize += model.Size
					}
					metrics.SetCustomMetric("ai", "total_model_size_gb", float64(totalSize)/(1024*1024*1024))
				}
				resp.Body.Close()
			}

			// Simulate inference metrics
			metrics.SetCustomMetric("ai", "inference_speed_tokens_per_sec", 450)
			metrics.SetCustomMetric("ai", "active_sessions", 2)
			metrics.SetCustomMetric("ai", "queue_size", 0)

		case <-ctx.Done():
			return
		}
	}
}

// Database Service specific metrics
func CollectDatabaseMetrics(ctx context.Context, metrics *PerformanceMetrics) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// In a real implementation, this would query pg_stat_database
			metrics.SetCustomMetric("database", "active_connections", 5)
			metrics.SetCustomMetric("database", "transactions_per_sec", 120)
			metrics.SetCustomMetric("database", "cache_hit_ratio", 0.98)
			metrics.SetCustomMetric("database", "deadlocks", 0)
			metrics.SetCustomMetric("database", "avg_query_time_ms", 2.5)

		case <-ctx.Done():
			return
		}
	}
}

// Cache Service specific metrics
func CollectCacheMetrics(ctx context.Context, metrics *PerformanceMetrics) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// In a real implementation, this would query Redis INFO
			metrics.SetCustomMetric("cache", "hit_rate", 0.95)
			metrics.SetCustomMetric("cache", "miss_rate", 0.05)
			metrics.SetCustomMetric("cache", "evicted_keys", 42)
			metrics.SetCustomMetric("cache", "operations_per_sec", 3500)
			metrics.SetCustomMetric("cache", "memory_fragmentation_ratio", 1.2)

		case <-ctx.Done():
			return
		}
	}
}
