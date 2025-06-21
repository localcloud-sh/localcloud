// internal/services/vectordb/interface.go
package vectordb

import (
	"context"
	"time"
)

// VectorDB defines the interface for vector database operations
type VectorDB interface {
	// Basic operations
	StoreEmbedding(ctx context.Context, doc Document) error
	StoreEmbeddings(ctx context.Context, docs []Document) error
	SearchSimilar(ctx context.Context, query QueryVector, limit int) ([]SearchResult, error)
	DeleteDocument(ctx context.Context, documentID string) error

	// RAG specific operations
	StoreChunks(ctx context.Context, chunks []Chunk) error
	HybridSearch(ctx context.Context, textQuery string, vector []float32, limit int) ([]SearchResult, error)

	// Management
	GetStats(ctx context.Context) (Stats, error)
	CreateCollection(ctx context.Context, name string, dimension int) error
	DeleteCollection(ctx context.Context, name string) error
}

// Document represents a document with its embedding
type Document struct {
	ID         string                 `json:"id"`
	Content    string                 `json:"content"`
	Vector     []float32              `json:"vector"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Collection string                 `json:"collection,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// Chunk represents a document chunk for RAG
type Chunk struct {
	DocumentID  string                 `json:"document_id"`
	ChunkIndex  int                    `json:"chunk_index"`
	Content     string                 `json:"content"`
	Vector      []float32              `json:"vector"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	StartOffset int                    `json:"start_offset"`
	EndOffset   int                    `json:"end_offset"`
}

// QueryVector represents a search query
type QueryVector struct {
	Vector     []float32              `json:"vector"`
	Collection string                 `json:"collection,omitempty"`
	Filter     map[string]interface{} `json:"filter,omitempty"`
}

// SearchResult represents a search result
type SearchResult struct {
	ID         string                 `json:"id"`
	Content    string                 `json:"content"`
	Score      float32                `json:"score"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	DocumentID string                 `json:"document_id,omitempty"`
	ChunkIndex int                    `json:"chunk_index,omitempty"`
}

// Stats represents vector database statistics
type Stats struct {
	TotalDocuments int64     `json:"total_documents"`
	TotalVectors   int64     `json:"total_vectors"`
	Collections    []string  `json:"collections"`
	IndexSize      int64     `json:"index_size_bytes"`
	LastUpdated    time.Time `json:"last_updated"`
}

// Config represents vector database configuration
type Config struct {
	Provider     string `json:"provider"`      // "pgvector" or "chroma"
	EmbeddingDim int    `json:"embedding_dim"` // Default: 1536
	MaxResults   int    `json:"max_results"`   // Default: 10
	IndexType    string `json:"index_type"`    // "ivfflat" or "hnsw"
}

// Provider represents a vector database provider
type Provider string

const (
	ProviderPgVector Provider = "pgvector"
	ProviderChroma   Provider = "chroma"
)
