// internal/logging/aggregator.go
package logging

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/localcloud-sh/localcloud/internal/config"
)

// LogLevel represents log severity level
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	Service     string                 `json:"service"`
	Container   string                 `json:"container"`
	Level       LogLevel               `json:"level"`
	Message     string                 `json:"message"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Source      string                 `json:"source"` // stdout or stderr
	ContainerID string                 `json:"container_id,omitempty"`
}

// Aggregator manages log collection from all containers
type Aggregator struct {
	client       *client.Client
	config       *config.Config
	logDir       string
	mu           sync.RWMutex
	writers      map[string]*RotatingWriter
	collectors   map[string]context.CancelFunc
	outputStream chan LogEntry
}

// NewAggregator creates a new log aggregator
func NewAggregator(cfg *config.Config) (*Aggregator, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	logDir := filepath.Join(".localcloud", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	return &Aggregator{
		client:       cli,
		config:       cfg,
		logDir:       logDir,
		writers:      make(map[string]*RotatingWriter),
		collectors:   make(map[string]context.CancelFunc),
		outputStream: make(chan LogEntry, 1000),
	}, nil
}

// Start begins log collection from all containers
func (a *Aggregator) Start(ctx context.Context) error {
	// List all LocalCloud containers
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("com.localcloud.project=%s", a.config.Project.Name))

	containers, err := a.client.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Start collecting logs from each container
	for _, container := range containers {
		if container.State == "running" {
			serviceName := a.getServiceName(container)
			if err := a.startCollector(ctx, container.ID, serviceName); err != nil {
				// Log error but continue with other containers
				fmt.Printf("Failed to start collector for %s: %v\n", serviceName, err)
			}
		}
	}

	// Start the aggregator writer
	go a.writeLoop(ctx)

	return nil
}

// Stop stops all log collectors
func (a *Aggregator) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Cancel all collectors
	for _, cancel := range a.collectors {
		cancel()
	}

	// Close all writers
	for _, writer := range a.writers {
		writer.Close()
	}

	close(a.outputStream)
}

// GetLogs retrieves logs with filters
func (a *Aggregator) GetLogs(options LogOptions) ([]LogEntry, error) {
	logFile := filepath.Join(a.logDir, "combined.json")

	file, err := os.Open(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogEntry{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var logs []LogEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue // Skip malformed entries
		}

		// Apply filters
		if !a.matchesFilters(entry, options) {
			continue
		}

		logs = append(logs, entry)
	}

	// Apply tail limit
	if options.Tail > 0 && len(logs) > options.Tail {
		logs = logs[len(logs)-options.Tail:]
	}

	return logs, scanner.Err()
}

// StreamLogs streams logs in real-time
func (a *Aggregator) StreamLogs(ctx context.Context, options LogOptions, output chan<- LogEntry) error {
	// Start with historical logs if not following
	if !options.Follow {
		logs, err := a.GetLogs(options)
		if err != nil {
			return err
		}
		for _, log := range logs {
			select {
			case output <- log:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}

	// Stream new logs
	for {
		select {
		case entry, ok := <-a.outputStream:
			if !ok {
				return nil
			}
			if a.matchesFilters(entry, options) {
				select {
				case output <- entry:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// startCollector starts log collection for a container
func (a *Aggregator) startCollector(ctx context.Context, containerID, serviceName string) error {
	ctx, cancel := context.WithCancel(ctx)

	a.mu.Lock()
	a.collectors[containerID] = cancel
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			delete(a.collectors, containerID)
			a.mu.Unlock()
		}()

		options := types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Timestamps: true,
			Since:      "0m",
		}

		reader, err := a.client.ContainerLogs(ctx, containerID, options)
		if err != nil {
			fmt.Printf("Error getting logs for %s: %v\n", serviceName, err)
			return
		}
		defer reader.Close()

		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			entry := a.parseLogLine(line, serviceName, containerID)

			select {
			case a.outputStream <- entry:
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// parseLogLine parses a Docker log line into a structured entry
func (a *Aggregator) parseLogLine(line, serviceName, containerID string) LogEntry {
	entry := LogEntry{
		Service:     serviceName,
		Container:   containerID[:12],
		ContainerID: containerID,
		Timestamp:   time.Now(),
		Level:       LogLevelInfo,
		Metadata:    make(map[string]interface{}),
	}

	// Docker log format: HEADER[8 bytes] + TIMESTAMP + SPACE + MESSAGE
	if len(line) > 8 {
		// Skip header bytes
		line = line[8:]

		// Parse timestamp if present
		parts := strings.SplitN(line, " ", 2)
		if len(parts) > 1 {
			if ts, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
				entry.Timestamp = ts
				line = parts[1]
			}
		}
	}

	// Detect log level from message
	entry.Level = detectLogLevel(line)
	entry.Message = line

	// Try to parse as JSON for structured logs
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(line), &jsonData); err == nil {
		if msg, ok := jsonData["message"].(string); ok {
			entry.Message = msg
			delete(jsonData, "message")
		}
		if level, ok := jsonData["level"].(string); ok {
			entry.Level = LogLevel(level)
			delete(jsonData, "level")
		}
		// Store remaining fields as metadata
		for k, v := range jsonData {
			entry.Metadata[k] = v
		}
	}

	return entry
}

// writeLoop writes log entries to files
func (a *Aggregator) writeLoop(ctx context.Context) {
	// Get or create combined log writer
	writer, err := a.getWriter("combined")
	if err != nil {
		fmt.Printf("Failed to create log writer: %v\n", err)
		return
	}

	for {
		select {
		case entry, ok := <-a.outputStream:
			if !ok {
				return
			}

			// Write to combined log
			if err := writer.WriteJSON(entry); err != nil {
				fmt.Printf("Failed to write log: %v\n", err)
			}

			// Also write to service-specific log
			if serviceWriter, err := a.getWriter(entry.Service); err == nil {
				serviceWriter.WriteJSON(entry)
			}

		case <-ctx.Done():
			return
		}
	}
}

// getWriter gets or creates a rotating writer for a log file
func (a *Aggregator) getWriter(name string) (*RotatingWriter, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if writer, exists := a.writers[name]; exists {
		return writer, nil
	}

	logPath := filepath.Join(a.logDir, fmt.Sprintf("%s.json", name))
	writer, err := NewRotatingWriter(logPath, 100*1024*1024) // 100MB
	if err != nil {
		return nil, err
	}

	a.writers[name] = writer
	return writer, nil
}

// matchesFilters checks if a log entry matches the given filters
func (a *Aggregator) matchesFilters(entry LogEntry, options LogOptions) bool {
	// Filter by service
	if options.Service != "" && entry.Service != options.Service {
		return false
	}

	// Filter by level
	if options.Level != "" && !isLevelHigherOrEqual(entry.Level, LogLevel(options.Level)) {
		return false
	}

	// Filter by time
	if !options.Since.IsZero() && entry.Timestamp.Before(options.Since) {
		return false
	}

	// Filter by search term
	if options.Search != "" && !strings.Contains(strings.ToLower(entry.Message), strings.ToLower(options.Search)) {
		return false
	}

	return true
}

// getServiceName extracts service name from container
func (a *Aggregator) getServiceName(container types.Container) string {
	// Try to get from labels first
	if service, ok := container.Labels["com.localcloud.service"]; ok {
		return service
	}

	// Extract from container name
	name := strings.TrimPrefix(container.Names[0], "/")
	parts := strings.Split(name, "-")
	if len(parts) >= 2 {
		return parts[1]
	}

	return name
}

// detectLogLevel attempts to detect log level from message
func detectLogLevel(message string) LogLevel {
	lower := strings.ToLower(message)

	if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") {
		return LogLevelError
	}
	if strings.Contains(lower, "warn") || strings.Contains(lower, "warning") {
		return LogLevelWarn
	}
	if strings.Contains(lower, "debug") || strings.Contains(lower, "trace") {
		return LogLevelDebug
	}

	return LogLevelInfo
}

// isLevelHigherOrEqual checks if level1 >= level2
func isLevelHigherOrEqual(level1, level2 LogLevel) bool {
	levels := map[LogLevel]int{
		LogLevelDebug: 0,
		LogLevelInfo:  1,
		LogLevelWarn:  2,
		LogLevelError: 3,
	}

	return levels[level1] >= levels[level2]
}

// LogOptions represents log filtering options
type LogOptions struct {
	Service string
	Level   string
	Since   time.Time
	Tail    int
	Follow  bool
	Search  string
}
