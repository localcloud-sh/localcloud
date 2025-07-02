// internal/services/postgres/postgres.go
package postgres

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/localcloud-sh/localcloud/internal/config"
)

// Service represents PostgreSQL service
type Service struct {
	config     *config.DatabaseConfig
	db         *sql.DB
	connString string
	hasVector  bool
	isReady    bool
}

// NewService creates a new PostgreSQL service
func NewService(cfg *config.DatabaseConfig) *Service {
	return &Service{
		config: cfg,
	}
}

// Initialize initializes the PostgreSQL service
func (s *Service) Initialize() error {
	// Generate connection string
	s.connString = s.generateConnectionString()

	// Wait for database to be ready
	if err := s.waitForReady(); err != nil {
		return fmt.Errorf("database not ready: %w", err)
	}

	// Connect to database
	db, err := sql.Open("postgres", s.connString)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	s.db = db
	s.isReady = true

	// Run initialization scripts
	if err := s.runInitScripts(); err != nil {
		return fmt.Errorf("failed to run init scripts: %w", err)
	}

	// Check for pgvector
	s.checkExtensions()

	return nil
}

// GetDB returns the database connection
func (s *Service) GetDB() *sql.DB {
	return s.db
}

// GetConnectionString returns the connection string
func (s *Service) GetConnectionString() string {
	return s.connString
}

// HasExtension checks if an extension is available
func (s *Service) HasExtension(name string) bool {
	if name == "pgvector" || name == "vector" {
		return s.hasVector
	}

	// Check in database
	var exists bool
	query := `SELECT EXISTS(
		SELECT 1 FROM pg_extension WHERE extname = $1
	)`
	s.db.QueryRow(query, name).Scan(&exists)
	return exists
}

// Close closes the database connection
func (s *Service) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// generateConnectionString generates PostgreSQL connection string
func (s *Service) generateConnectionString() string {
	// Default values
	host := "localhost"
	port := s.config.Port
	user := "localcloud"
	password := "localcloud-dev"
	dbname := "localcloud"

	// In Docker network, use service name as host
	if os.Getenv("LOCALCLOUD_DOCKER") == "true" {
		host = "localcloud-postgres"
	}

	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)
}

// waitForReady waits for database to be ready
func (s *Service) waitForReady() error {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for database")
		case <-ticker.C:
			db, err := sql.Open("postgres", s.connString)
			if err == nil {
				if err := db.Ping(); err == nil {
					db.Close()
					return nil
				}
				db.Close()
			}
		}
	}
}

// runInitScripts runs initialization SQL scripts
func (s *Service) runInitScripts() error {
	// Base initialization script
	baseScript := s.getBaseInitScript()
	if err := s.executeScript(baseScript); err != nil {
		return fmt.Errorf("failed to run base script: %w", err)
	}

	// Extension-specific scripts
	for _, ext := range s.config.Extensions {
		switch ext {
		case "pgvector", "vector":
			if err := s.executeScript(s.getVectorInitScript()); err != nil {
				return fmt.Errorf("failed to install pgvector: %w", err)
			}
			s.hasVector = true
		case "uuid-ossp":
			if err := s.executeScript(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`); err != nil {
				return fmt.Errorf("failed to install uuid-ossp: %w", err)
			}
		case "pg_trgm":
			if err := s.executeScript(`CREATE EXTENSION IF NOT EXISTS pg_trgm;`); err != nil {
				return fmt.Errorf("failed to install pg_trgm: %w", err)
			}
		}
	}

	return nil
}

// executeScript executes a SQL script
func (s *Service) executeScript(script string) error {
	_, err := s.db.Exec(script)
	return err
}

// checkExtensions checks which extensions are installed
func (s *Service) checkExtensions() {
	extensions := []string{"vector", "uuid-ossp", "pg_trgm"}
	for _, ext := range extensions {
		if s.HasExtension(ext) {
			fmt.Printf("Extension %s is installed\n", ext)
		}
	}
}

// getBaseInitScript returns base initialization script
func (s *Service) getBaseInitScript() string {
	return `
-- Base LocalCloud initialization
CREATE SCHEMA IF NOT EXISTS localcloud;

-- Metadata table
CREATE TABLE IF NOT EXISTS localcloud.metadata (
	key VARCHAR(255) PRIMARY KEY,
	value TEXT,
	created_at TIMESTAMP DEFAULT NOW(),
	updated_at TIMESTAMP DEFAULT NOW()
);

-- Insert version
INSERT INTO localcloud.metadata (key, value) 
VALUES ('version', '1.0.0')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
`
}

// getVectorInitScript returns pgvector initialization script
func (s *Service) getVectorInitScript() string {
	return `
-- Install pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Create embeddings table (only if using pgvector)
CREATE TABLE IF NOT EXISTS localcloud.embeddings (
	id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
	document_id TEXT NOT NULL,
	chunk_index INTEGER NOT NULL,
	content TEXT NOT NULL,
	embedding vector(1536),
	metadata JSONB,
	created_at TIMESTAMP DEFAULT NOW(),
	UNIQUE(document_id, chunk_index)
);

-- Create index for similarity search
CREATE INDEX IF NOT EXISTS embeddings_vector_idx 
ON localcloud.embeddings 
USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 100);
`
}
