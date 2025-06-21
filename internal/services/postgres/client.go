// internal/services/postgres/client.go
package postgres

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Client provides database operations
type Client struct {
	service *Service
}

// NewClient creates a new database client
func NewClient(service *Service) *Client {
	return &Client{service: service}
}

// Connect opens an interactive psql session
func (c *Client) Connect() error {
	// First try native psql if available
	if _, err := exec.LookPath("psql"); err == nil {
		cmd := exec.Command("psql", c.service.connString)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Fallback to Docker exec - no additional dependencies needed!
	fmt.Println("Connecting via Docker (psql not found locally)...")

	containerName := "localcloud-postgres"

	// Check if container is running
	checkCmd := exec.Command("docker", "ps", "-q", "-f", "name="+containerName)
	output, err := checkCmd.Output()
	if err != nil || len(output) == 0 {
		return fmt.Errorf("PostgreSQL container not running. Run 'localcloud start' first")
	}

	// Connect using docker exec
	cmd := exec.Command("docker", "exec", "-it", containerName,
		"psql", "-U", "localcloud", "-d", "localcloud")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	return nil
}

// Backup creates a database backup
func (c *Client) Backup(outputPath string) error {
	// Create output directory if needed
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Try native pg_dump first
	if _, err := exec.LookPath("pg_dump"); err == nil {
		cmd := exec.Command("pg_dump",
			c.service.connString,
			"--clean",
			"--if-exists",
			"--no-owner",
			"--no-acl",
			"-f", outputPath,
		)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("backup failed: %w\n%s", err, output)
		}
		return nil
	}

	// Fallback to Docker exec
	fmt.Println("Creating backup via Docker...")

	containerName := "localcloud-postgres"

	// Check if container is running
	checkCmd := exec.Command("docker", "ps", "-q", "-f", "name="+containerName)
	output, err := checkCmd.Output()
	if err != nil || len(output) == 0 {
		return fmt.Errorf("PostgreSQL container not running. Run 'localcloud start' first")
	}

	// Create backup inside container first
	tempFile := "/tmp/backup.sql"
	cmd := exec.Command("docker", "exec", containerName,
		"pg_dump", "-U", "localcloud", "-d", "localcloud",
		"--clean", "--if-exists", "--no-owner", "--no-acl",
		"-f", tempFile)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("backup failed: %w\n%s", err, output)
	}

	// Copy backup file from container to host
	copyCmd := exec.Command("docker", "cp",
		fmt.Sprintf("%s:%s", containerName, tempFile), outputPath)

	if output, err := copyCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy backup: %w\n%s", err, output)
	}

	// Clean up temp file in container
	exec.Command("docker", "exec", containerName, "rm", tempFile).Run()

	fmt.Printf("Backup created successfully: %s\n", outputPath)
	return nil
}

// Restore restores a database backup
func (c *Client) Restore(inputPath string) error {
	// Check if file exists
	if _, err := os.Stat(inputPath); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	// Try native psql first
	if _, err := exec.LookPath("psql"); err == nil {
		cmd := exec.Command("psql", c.service.connString, "-f", inputPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("restore failed: %w\n%s", err, output)
		}
		return nil
	}

	// Fallback to Docker exec
	fmt.Println("Restoring via Docker...")

	containerName := "localcloud-postgres"

	// Check if container is running
	checkCmd := exec.Command("docker", "ps", "-q", "-f", "name="+containerName)
	output, err := checkCmd.Output()
	if err != nil || len(output) == 0 {
		return fmt.Errorf("PostgreSQL container not running. Run 'localcloud start' first")
	}

	// Copy backup file to container
	tempFile := "/tmp/restore.sql"
	copyCmd := exec.Command("docker", "cp", inputPath,
		fmt.Sprintf("%s:%s", containerName, tempFile))

	if output, err := copyCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy backup file: %w\n%s", err, output)
	}

	// Run psql to restore
	cmd := exec.Command("docker", "exec", containerName,
		"psql", "-U", "localcloud", "-d", "localcloud", "-f", tempFile)

	if output, err := cmd.CombinedOutput(); err != nil {
		// Clean up temp file even on error
		exec.Command("docker", "exec", containerName, "rm", tempFile).Run()
		return fmt.Errorf("restore failed: %w\n%s", err, output)
	}

	// Clean up temp file in container
	exec.Command("docker", "exec", containerName, "rm", tempFile).Run()

	fmt.Println("Database restored successfully")
	return nil
}

// ExecuteFile executes a SQL file
func (c *Client) ExecuteFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read SQL file: %w", err)
	}

	_, err = c.service.db.Exec(string(content))
	if err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	return nil
}

// Query executes a query and returns results
func (c *Client) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return c.service.db.Query(query, args...)
}

// Exec executes a statement
func (c *Client) Exec(query string, args ...interface{}) (sql.Result, error) {
	return c.service.db.Exec(query, args...)
}

// Transaction starts a new transaction
func (c *Client) Transaction() (*sql.Tx, error) {
	return c.service.db.Begin()
}
