// internal/services/vectordb/providers/pgvector/pgvector.go
package pgvector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/localcloud/localcloud/internal/services/postgres"
	"github.com/localcloud/localcloud/internal/services/vectordb"
)

// PgVectorDB implements VectorDB interface using PostgreSQL with pgvector
type PgVectorDB struct {
	client       *postgres.Client
	config       *vectordb.Config
	defaultTable string
}

// New creates a new PgVector instance
func New(client *postgres.Client, config *vectordb.Config) (*PgVectorDB, error) {
	db := &PgVectorDB{
		client:       client,
		config:       config,
		defaultTable: "localcloud.embeddings",
	}

	// Ensure pgvector tables exist
	if err := db.ensureTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return db, nil
}

// StoreEmbedding stores a single document embedding
func (db *PgVectorDB) StoreEmbedding(ctx context.Context, doc vectordb.Document) error {
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	vectorStr := vectorToString(doc.Vector)

	query := `
		INSERT INTO localcloud.embeddings 
		(id, document_id, chunk_index, content, embedding, metadata)
		VALUES ($1, $1, 0, $2, $3::vector, $4)
		ON CONFLICT (document_id, chunk_index) 
		DO UPDATE SET 
			content = EXCLUDED.content,
			embedding = EXCLUDED.embedding,
			metadata = EXCLUDED.metadata,
			created_at = NOW()
	`

	_, err = db.client.Exec(query, doc.ID, doc.Content, vectorStr, metadataJSON)
	return err
}

// StoreEmbeddings stores multiple document embeddings
func (db *PgVectorDB) StoreEmbeddings(ctx context.Context, docs []vectordb.Document) error {
	tx, err := db.client.Transaction()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, doc := range docs {
		metadataJSON, err := json.Marshal(doc.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		vectorStr := vectorToString(doc.Vector)

		query := `
			INSERT INTO localcloud.embeddings 
			(id, document_id, chunk_index, content, embedding, metadata)
			VALUES ($1, $1, 0, $2, $3::vector, $4)
			ON CONFLICT (document_id, chunk_index) 
			DO UPDATE SET 
				content = EXCLUDED.content,
				embedding = EXCLUDED.embedding,
				metadata = EXCLUDED.metadata,
				created_at = NOW()
		`

		if _, err := tx.Exec(query, doc.ID, doc.Content, vectorStr, metadataJSON); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// SearchSimilar performs similarity search
func (db *PgVectorDB) SearchSimilar(ctx context.Context, query vectordb.QueryVector, limit int) ([]vectordb.SearchResult, error) {
	vectorStr := vectorToString(query.Vector)

	sqlQuery := `
		SELECT 
			id,
			document_id,
			chunk_index,
			content,
			metadata,
			1 - (embedding <=> $1::vector) as similarity
		FROM localcloud.embeddings
		ORDER BY embedding <=> $1::vector
		LIMIT $2
	`

	rows, err := db.client.Query(sqlQuery, vectorStr, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []vectordb.SearchResult
	for rows.Next() {
		var result vectordb.SearchResult
		var metadataJSON []byte
		var id string

		err := rows.Scan(
			&id,
			&result.DocumentID,
			&result.ChunkIndex,
			&result.Content,
			&metadataJSON,
			&result.Score,
		)
		if err != nil {
			return nil, err
		}

		result.ID = id
		if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}

// DeleteDocument deletes all embeddings for a document
func (db *PgVectorDB) DeleteDocument(ctx context.Context, documentID string) error {
	query := "DELETE FROM localcloud.embeddings WHERE document_id = $1"
	_, err := db.client.Exec(query, documentID)
	return err
}

// StoreChunks stores document chunks for RAG
func (db *PgVectorDB) StoreChunks(ctx context.Context, chunks []vectordb.Chunk) error {
	tx, err := db.client.Transaction()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, chunk := range chunks {
		metadataJSON, err := json.Marshal(chunk.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		vectorStr := vectorToString(chunk.Vector)

		query := `
			INSERT INTO localcloud.embeddings 
			(document_id, chunk_index, content, embedding, metadata)
			VALUES ($1, $2, $3, $4::vector, $5)
			ON CONFLICT (document_id, chunk_index) 
			DO UPDATE SET 
				content = EXCLUDED.content,
				embedding = EXCLUDED.embedding,
				metadata = EXCLUDED.metadata,
				created_at = NOW()
		`

		if _, err := tx.Exec(query,
			chunk.DocumentID,
			chunk.ChunkIndex,
			chunk.Content,
			vectorStr,
			metadataJSON,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// HybridSearch performs both text and vector search
func (db *PgVectorDB) HybridSearch(ctx context.Context, textQuery string, vector []float32, limit int) ([]vectordb.SearchResult, error) {
	vectorStr := vectorToString(vector)

	// Combine vector similarity with text search using pg_trgm
	query := `
		SELECT 
			id,
			document_id,
			chunk_index,
			content,
			metadata,
			(
				0.7 * (1 - (embedding <=> $1::vector)) +
				0.3 * similarity(content, $2)
			) as combined_score
		FROM localcloud.embeddings
		WHERE 
			content % $2  -- pg_trgm similarity threshold
			OR embedding <=> $1::vector < 0.8
		ORDER BY combined_score DESC
		LIMIT $3
	`

	rows, err := db.client.Query(query, vectorStr, textQuery, limit)
	if err != nil {
		// Fallback to vector-only search if pg_trgm is not available
		return db.SearchSimilar(ctx, vectordb.QueryVector{Vector: vector}, limit)
	}
	defer rows.Close()

	var results []vectordb.SearchResult
	for rows.Next() {
		var result vectordb.SearchResult
		var metadataJSON []byte
		var id string

		err := rows.Scan(
			&id,
			&result.DocumentID,
			&result.ChunkIndex,
			&result.Content,
			&metadataJSON,
			&result.Score,
		)
		if err != nil {
			return nil, err
		}

		result.ID = id
		if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}

// GetStats returns database statistics
func (db *PgVectorDB) GetStats(ctx context.Context) (vectordb.Stats, error) {
	var stats vectordb.Stats
	stats.LastUpdated = time.Now()

	// Count total documents
	err := db.client.Query(
		"SELECT COUNT(DISTINCT document_id) FROM localcloud.embeddings",
	).Scan(&stats.TotalDocuments)
	if err != nil {
		return stats, err
	}

	// Count total vectors
	err = db.client.Query(
		"SELECT COUNT(*) FROM localcloud.embeddings",
	).Scan(&stats.TotalVectors)
	if err != nil {
		return stats, err
	}

	stats.Collections = []string{"default"} // pgvector uses single table

	return stats, nil
}

// CreateCollection is a no-op for pgvector (uses single table)
func (db *PgVectorDB) CreateCollection(ctx context.Context, name string, dimension int) error {
	// pgvector uses a single table, collections are logical
	return nil
}

// DeleteCollection is a no-op for pgvector
func (db *PgVectorDB) DeleteCollection(ctx context.Context, name string) error {
	// pgvector uses a single table
	return nil
}

// ensureTables creates necessary tables if they don't exist
func (db *PgVectorDB) ensureTables() error {
	// This is already handled in postgres.go initialization
	// Just verify the table exists
	var exists bool
	err := db.client.Query(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'localcloud' 
			AND table_name = 'embeddings'
		)
	`).Scan(&exists)

	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("embeddings table not found. Ensure pgvector extension is enabled")
	}

	return nil
}

// vectorToString converts float32 slice to PostgreSQL vector format
func vectorToString(vector []float32) string {
	parts := make([]string, len(vector))
	for i, v := range vector {
		parts[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
