// internal/cli/logs.go - Enhanced version with centralized logging
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/localcloud/localcloud/internal/config"
	"github.com/localcloud/localcloud/internal/logging"
	"github.com/spf13/cobra"
)

var (
	followLogs   bool
	tailLines    int
	sinceTime    string
	logLevel     string
	outputFormat string
	searchTerm   string
)

var logsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "Show logs from LocalCloud services",
	Long: `Display logs from LocalCloud services with advanced filtering options.
	
Logs are collected centrally and can be filtered by service, time, level, and search terms.
All logs are stored in JSON format for easy parsing and analysis.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLogs,
	Example: `  localcloud logs                      # Show all logs
  localcloud logs ai                    # Show AI service logs
  localcloud logs -f                    # Follow log output
  localcloud logs -n 50                 # Show last 50 lines
  localcloud logs --since 1h            # Show logs from last hour
  localcloud logs --level error         # Show only errors
  localcloud logs --json                # Output as JSON
  localcloud logs --search "model"      # Search for specific term`,
}

func init() {
	logsCmd.Flags().BoolVarP(&followLogs, "follow", "f", false, "Follow log output")
	logsCmd.Flags().IntVarP(&tailLines, "tail", "n", 100, "Number of lines to show from the end")
	logsCmd.Flags().StringVar(&sinceTime, "since", "", "Show logs since timestamp (e.g., 1h, 30m, 2006-01-02T15:04:05)")
	logsCmd.Flags().StringVar(&logLevel, "level", "", "Minimum log level to show (debug, info, warn, error)")
	logsCmd.Flags().StringVar(&outputFormat, "output", "text", "Output format (text, json)")
	logsCmd.Flags().StringVar(&searchTerm, "search", "", "Search logs for specific term")
}

func runLogs(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	// Get config
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	// Create log aggregator
	aggregator, err := logging.NewAggregator(cfg)
	if err != nil {
		return fmt.Errorf("failed to create log aggregator: %w", err)
	}

	// Parse since time
	var sinceTimestamp time.Time
	if sinceTime != "" {
		sinceTimestamp, err = parseSinceTime(sinceTime)
		if err != nil {
			return fmt.Errorf("invalid since time: %w", err)
		}
	}

	// Build log options
	options := logging.LogOptions{
		Tail:   tailLines,
		Since:  sinceTimestamp,
		Follow: followLogs,
		Level:  logLevel,
		Search: searchTerm,
	}

	// Filter by service if specified
	if len(args) > 0 {
		options.Service = args[0]
		// Validate service name
		validServices := []string{"ai", "postgres", "redis", "minio", "database", "cache", "storage"}
		isValid := false
		for _, s := range validServices {
			if s == options.Service {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("unknown service '%s'. Available services: %s",
				options.Service, strings.Join(validServices, ", "))
		}

		// Map aliases
		switch options.Service {
		case "database":
			options.Service = "postgres"
		case "cache":
			options.Service = "redis"
		case "storage":
			options.Service = "minio"
		}
	}

	// Start log aggregation
	ctx := context.Background()
	if err := aggregator.Start(ctx); err != nil {
		return fmt.Errorf("failed to start log aggregation: %w", err)
	}
	defer aggregator.Stop()

	// Show header if not following
	if !followLogs && outputFormat == "text" {
		if options.Service != "" {
			fmt.Printf("Showing logs for %s service", options.Service)
		} else {
			fmt.Printf("Showing logs for all services")
		}

		if tailLines > 0 && !followLogs {
			fmt.Printf(" (last %d lines)", tailLines)
		}
		if sinceTime != "" {
			fmt.Printf(" since %s", sinceTime)
		}
		fmt.Println(":")
		fmt.Println(strings.Repeat("─", 80))
	}

	// Create output channel
	output := make(chan logging.LogEntry, 100)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle interrupt
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)
		<-sigChan
		cancel()
	}()

	// Start streaming logs
	go func() {
		if err := aggregator.StreamLogs(ctx, options, output); err != nil && err != context.Canceled {
			fmt.Printf("Error streaming logs: %v\n", err)
		}
		close(output)
	}()

	// Display logs
	for entry := range output {
		if outputFormat == "json" {
			data, _ := json.Marshal(entry)
			fmt.Println(string(data))
		} else {
			formatLogEntry(entry)
		}
	}

	return nil
}

// formatLogEntry formats a log entry for display
func formatLogEntry(entry logging.LogEntry) {
	// Color codes for levels
	levelColors := map[logging.LogLevel]string{
		logging.LogLevelDebug: "\033[36m", // Cyan
		logging.LogLevelInfo:  "\033[37m", // White
		logging.LogLevelWarn:  "\033[33m", // Yellow
		logging.LogLevelError: "\033[31m", // Red
	}

	resetColor := "\033[0m"

	// Format timestamp
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")

	// Format level with color
	level := string(entry.Level)
	if color, ok := levelColors[entry.Level]; ok {
		level = fmt.Sprintf("%s%-5s%s", color, strings.ToUpper(level), resetColor)
	} else {
		level = fmt.Sprintf("%-5s", strings.ToUpper(level))
	}

	// Format service name
	service := fmt.Sprintf("[%-10s]", entry.Service)

	// Build output line
	fmt.Printf("%s %s %s %s", timestamp, level, service, entry.Message)

	// Add metadata if present
	if len(entry.Metadata) > 0 {
		metadataStr := []string{}
		for k, v := range entry.Metadata {
			metadataStr = append(metadataStr, fmt.Sprintf("%s=%v", k, v))
		}
		fmt.Printf(" {%s}", strings.Join(metadataStr, " "))
	}

	fmt.Println()
}

// parseSinceTime parses various time formats
func parseSinceTime(since string) (time.Time, error) {
	// Try parsing as duration (e.g., "1h", "30m")
	if strings.HasSuffix(since, "h") || strings.HasSuffix(since, "m") || strings.HasSuffix(since, "s") {
		duration, err := time.ParseDuration(since)
		if err == nil {
			return time.Now().Add(-duration), nil
		}
	}

	// Try parsing as RFC3339
	if t, err := time.Parse(time.RFC3339, since); err == nil {
		return t, nil
	}

	// Try parsing as date
	if t, err := time.Parse("2006-01-02", since); err == nil {
		return t, nil
	}

	// Try parsing as datetime
	if t, err := time.Parse("2006-01-02T15:04:05", since); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", since)
}
