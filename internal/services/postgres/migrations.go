// internal/services/postgres/migrations.go
package postgres

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Migration represents a database migration
type Migration struct {
	Version   int
	Name      string
	SQL       string
	Timestamp time.Time
}

// MigrationManager handles database migrations
type MigrationManager struct {
	client *Client
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(client *Client) *MigrationManager {
	return &MigrationManager{client: client}
}

// Initialize creates the migration tracking table
func (m *MigrationManager) Initialize() error {
	query := `
	CREATE TABLE IF NOT EXISTS localcloud.migrations (
		version INTEGER PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		applied_at TIMESTAMP DEFAULT NOW()
	);`

	_, err := m.client.Exec(query)
	return err
}

// Migrate runs all pending migrations
func (m *MigrationManager) Migrate(migrationsPath string) error {
	// Initialize migration table
	if err := m.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize migrations: %w", err)
	}

	// Get applied migrations
	applied, err := m.getAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Get pending migrations
	pending, err := m.getPendingMigrations(migrationsPath, applied)
	if err != nil {
		return fmt.Errorf("failed to get pending migrations: %w", err)
	}

	// Apply pending migrations
	for _, migration := range pending {
		if err := m.applyMigration(migration); err != nil {
			return fmt.Errorf("failed to apply migration %d_%s: %w",
				migration.Version, migration.Name, err)
		}
		fmt.Printf("Applied migration: %d_%s\n", migration.Version, migration.Name)
	}

	if len(pending) == 0 {
		fmt.Println("Database is up to date")
	}

	return nil
}

// getAppliedMigrations returns list of applied migration versions
func (m *MigrationManager) getAppliedMigrations() (map[int]bool, error) {
	applied := make(map[int]bool)

	query := "SELECT version FROM localcloud.migrations"
	rows, err := m.client.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, nil
}

// getPendingMigrations returns list of migrations to apply
func (m *MigrationManager) getPendingMigrations(migrationsPath string, applied map[int]bool) ([]Migration, error) {
	var pending []Migration

	// Read migration files
	err := filepath.WalkDir(migrationsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-SQL files
		if d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}

		// Parse migration file name (e.g., "001_initial.sql")
		filename := d.Name()
		parts := strings.SplitN(strings.TrimSuffix(filename, ".sql"), "_", 2)
		if len(parts) != 2 {
			return nil // Skip invalid filenames
		}

		// Parse version number
		var version int
		if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
			return nil // Skip non-numeric versions
		}

		// Skip if already applied
		if applied[version] {
			return nil
		}

		// Read migration content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", filename, err)
		}

		pending = append(pending, Migration{
			Version: version,
			Name:    parts[1],
			SQL:     string(content),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by version
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].Version < pending[j].Version
	})

	return pending, nil
}

// applyMigration applies a single migration
func (m *MigrationManager) applyMigration(migration Migration) error {
	// Start transaction
	tx, err := m.client.Transaction()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Execute migration
	if _, err := tx.Exec(migration.SQL); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Record migration
	query := `INSERT INTO localcloud.migrations (version, name) VALUES ($1, $2)`
	if _, err := tx.Exec(query, migration.Version, migration.Name); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	return tx.Commit()
}

// Status shows migration status
func (m *MigrationManager) Status() error {
	query := `
	SELECT version, name, applied_at 
	FROM localcloud.migrations 
	ORDER BY version`

	rows, err := m.client.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Println("Applied migrations:")
	fmt.Println("Version | Name                    | Applied At")
	fmt.Println("--------|-------------------------|-------------------")

	for rows.Next() {
		var version int
		var name string
		var appliedAt time.Time

		if err := rows.Scan(&version, &name, &appliedAt); err != nil {
			return err
		}

		fmt.Printf("%-7d | %-23s | %s\n",
			version, name, appliedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}
