// internal/logging/rotation.go
package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RotatingWriter handles log file rotation
type RotatingWriter struct {
	mu          sync.Mutex
	file        *os.File
	path        string
	maxSize     int64
	currentSize int64
}

// NewRotatingWriter creates a new rotating log writer
func NewRotatingWriter(path string, maxSize int64) (*RotatingWriter, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open or create log file
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat log file: %w", err)
	}

	return &RotatingWriter{
		file:        file,
		path:        path,
		maxSize:     maxSize,
		currentSize: info.Size(),
	}, nil
}

// Write writes data to the log file
func (rw *RotatingWriter) Write(p []byte) (n int, err error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	// Check if rotation is needed
	if rw.currentSize+int64(len(p)) > rw.maxSize {
		if err := rw.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = rw.file.Write(p)
	if err != nil {
		return n, err
	}

	rw.currentSize += int64(n)
	return n, nil
}

// WriteJSON writes a JSON object to the log
func (rw *RotatingWriter) WriteJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	data = append(data, '\n')
	_, err = rw.Write(data)
	return err
}

// Close closes the log file
func (rw *RotatingWriter) Close() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.file != nil {
		return rw.file.Close()
	}
	return nil
}

// rotate performs log rotation
func (rw *RotatingWriter) rotate() error {
	// Close current file
	if err := rw.file.Close(); err != nil {
		return err
	}

	// Generate timestamp for rotated file
	timestamp := time.Now().Format("20060102-150405")
	ext := filepath.Ext(rw.path)
	base := rw.path[:len(rw.path)-len(ext)]
	rotatedPath := fmt.Sprintf("%s-%s%s", base, timestamp, ext)

	// Rename current file
	if err := os.Rename(rw.path, rotatedPath); err != nil {
		return err
	}

	// Clean up old files
	go rw.cleanupOldFiles(base, ext)

	// Create new file
	file, err := os.OpenFile(rw.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	rw.file = file
	rw.currentSize = 0

	return nil
}

// cleanupOldFiles removes old rotated files, keeping only the most recent 5
func (rw *RotatingWriter) cleanupOldFiles(base, ext string) {
	dir := filepath.Dir(base)
	pattern := filepath.Base(base) + "-*" + ext

	files, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil || len(files) <= 5 {
		return
	}

	// Sort files by modification time (oldest first)
	type fileInfo struct {
		path    string
		modTime time.Time
	}

	var infos []fileInfo
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		infos = append(infos, fileInfo{
			path:    file,
			modTime: info.ModTime(),
		})
	}

	// Sort by modification time
	for i := 0; i < len(infos)-1; i++ {
		for j := i + 1; j < len(infos); j++ {
			if infos[i].modTime.After(infos[j].modTime) {
				infos[i], infos[j] = infos[j], infos[i]
			}
		}
	}

	// Remove oldest files, keeping only 5 most recent
	for i := 0; i < len(infos)-5; i++ {
		os.Remove(infos[i].path)
	}
}
