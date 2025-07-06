// internal/cli/export.go
package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/localcloud-sh/localcloud/internal/config"
	"github.com/localcloud-sh/localcloud/internal/services/postgres"
	"github.com/minio/minio-go/v7"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export LocalCloud data for migration",
	Long: `Export LocalCloud data to portable files for migration to cloud services.

This command creates portable export files that can be imported to any compatible service:
- PostgreSQL exports work with any PostgreSQL instance (AWS RDS, Google Cloud SQL, etc.)
- MongoDB exports work with MongoDB Atlas, managed MongoDB, or any MongoDB instance
- Storage exports work with AWS S3, Google Cloud Storage, or any S3-compatible service

The export files are created in the current directory by default, or you can specify 
a custom output location with the --output flag.`,
	Example: `  # Export all services to current directory
  lc export all

  # Export specific service to current directory
  lc export db
  lc export mongo
  lc export storage
  lc export vector

  # Export to specific location
  lc export all --output=./backups/
  lc export db --output=./my-db-backup.sql

  # Export with custom filename
  lc export mongo --output=./prod-mongo-backup.tar.gz`,
}

var exportAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Export all configured services",
	Long: `Export all configured LocalCloud services to portable files.

This command will export:
- PostgreSQL database (if configured) → .sql file
- MongoDB database (if configured) → .tar.gz file  
- MinIO storage (if configured) → .tar.gz file

Files are created with timestamp-based names in the current directory unless --output is specified.`,
	Example: `  # Export all to current directory
  lc export all

  # Export all to specific directory
  lc export all --output=./migration-backup/

  # Files created:
  # - localcloud-db-20240105-143022.sql
  # - localcloud-mongo-20240105-143022.tar.gz
  # - localcloud-storage-20240105-143022.tar.gz`,
	RunE: runExportAll,
}

var exportDBCmd = &cobra.Command{
	Use:   "db",
	Short: "Export PostgreSQL database",
	Long: `Export PostgreSQL database to a .sql file using pg_dump.

The exported file can be imported to any PostgreSQL instance using:
  psql -h hostname -U username -d database_name < exported_file.sql

This works with:
- AWS RDS PostgreSQL
- Google Cloud SQL PostgreSQL  
- Azure Database for PostgreSQL
- Any managed or self-hosted PostgreSQL instance`,
	Example: `  # Export to current directory (default filename)
  lc export db

  # Export to specific file
  lc export db --output=./production-db-backup.sql

  # Export to specific directory (auto-generated filename)
  lc export db --output=./backups/`,
	RunE: runExportDB,
}

var exportMongoCmd = &cobra.Command{
	Use:   "mongo",
	Short: "Export MongoDB database",
	Long: `Export MongoDB database to a .tar.gz archive using mongodump.

The exported file can be imported to any MongoDB instance using:
  mongorestore --uri="mongodb://connection-string" --archive=exported_file.tar.gz --gzip

This works with:
- MongoDB Atlas
- AWS DocumentDB
- Google Cloud Firestore in MongoDB mode
- Any managed or self-hosted MongoDB instance`,
	Example: `  # Export to current directory (default filename)
  lc export mongo

  # Export to specific file
  lc export mongo --output=./production-mongo-backup.tar.gz

  # Export to specific directory (auto-generated filename)
  lc export mongo --output=./backups/`,
	RunE: runExportMongo,
}

var exportStorageCmd = &cobra.Command{
	Use:     "storage",
	Aliases: []string{"s3", "minio"},
	Short:   "Export MinIO storage",
	Long: `Export MinIO storage to a .tar.gz archive containing all buckets and objects.

The exported file preserves the bucket structure and can be uploaded to any S3-compatible service:
- AWS S3: aws s3 sync extracted_folder/ s3://your-bucket/
- Google Cloud Storage: gsutil rsync -r extracted_folder/ gs://your-bucket/
- Azure Blob Storage: az storage blob upload-batch

The archive contains:
- All buckets as directories
- All objects with original paths
- Metadata and permissions where possible`,
	Example: `  # Export to current directory (default filename)
  lc export storage

  # Export to specific file
  lc export storage --output=./production-storage-backup.tar.gz

  # Export to specific directory (auto-generated filename)
  lc export storage --output=./backups/`,
	RunE: runExportStorage,
}

var exportVectorCmd = &cobra.Command{
	Use:     "vector",
	Aliases: []string{"embeddings", "pgvector"},
	Short:   "Export vector database embeddings",
	Long: `Export vector database embeddings to a JSON file for migration to cloud vector services.

The exported file contains all vector embeddings, metadata, and can be imported to:
- Pinecone: Using their bulk upsert API
- Weaviate: Using their batch import functionality  
- Chroma: Using their add() method with embeddings
- Qdrant: Using their upsert API
- Any pgvector instance: Using the included SQL import script

The export includes:
- Document IDs and content
- Vector embeddings (1536-dimensional by default)
- Metadata and collection information
- SQL script for reimporting to pgvector`,
	Example: `  # Export to current directory (default filename)
  lc export vector

  # Export to specific file
  lc export vector --output=./production-vectors.json

  # Export to specific directory (auto-generated filename)
  lc export vector --output=./backups/

  # Export specific collection only
  lc export vector --collection=my-documents`,
	RunE: runExportVector,
}

var (
	exportOutput     string
	exportCollection string
)

func init() {
	// Add global output flag to all export commands
	exportAllCmd.Flags().StringVar(&exportOutput, "output", "", "Output directory or file path")
	exportDBCmd.Flags().StringVar(&exportOutput, "output", "", "Output directory or file path")
	exportMongoCmd.Flags().StringVar(&exportOutput, "output", "", "Output directory or file path")
	exportStorageCmd.Flags().StringVar(&exportOutput, "output", "", "Output directory or file path")
	exportVectorCmd.Flags().StringVar(&exportOutput, "output", "", "Output directory or file path")

	// Vector-specific flags
	exportVectorCmd.Flags().StringVar(&exportCollection, "collection", "", "Export specific collection only (default: all collections)")

	// Add subcommands
	exportCmd.AddCommand(exportAllCmd)
	exportCmd.AddCommand(exportDBCmd)
	exportCmd.AddCommand(exportMongoCmd)
	exportCmd.AddCommand(exportStorageCmd)
	exportCmd.AddCommand(exportVectorCmd)

	// Add to root command
	rootCmd.AddCommand(exportCmd)
}

func runExportAll(cmd *cobra.Command, args []string) error {
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	cfg := config.Get()
	exported := []string{}

	printInfo("Starting export of all configured services...")

	// Export PostgreSQL if configured
	if cfg.Services.Database.Type != "" {
		printInfo("Exporting PostgreSQL database...")
		if err := exportPostgreSQL(cfg); err != nil {
			printWarning(fmt.Sprintf("Failed to export PostgreSQL: %v", err))
		} else {
			exported = append(exported, "PostgreSQL")
		}
	}

	// Export MongoDB if configured
	if cfg.Services.MongoDB.Type != "" {
		printInfo("Exporting MongoDB database...")
		if err := exportMongoDB(cfg); err != nil {
			printWarning(fmt.Sprintf("Failed to export MongoDB: %v", err))
		} else {
			exported = append(exported, "MongoDB")
		}
	}

	// Export Storage if configured
	if cfg.Services.Storage.Type != "" {
		printInfo("Exporting MinIO storage...")
		if err := exportStorage(cfg); err != nil {
			printWarning(fmt.Sprintf("Failed to export Storage: %v", err))
		} else {
			exported = append(exported, "Storage")
		}
	}

	// Export Vector Database if PostgreSQL is configured (pgvector)
	if cfg.Services.Database.Type != "" {
		printInfo("Exporting vector database embeddings...")
		if err := exportVectorDatabase(cfg); err != nil {
			printWarning(fmt.Sprintf("Failed to export vector database: %v", err))
		} else {
			exported = append(exported, "Vector Database")
		}
	}

	if len(exported) == 0 {
		printWarning("No services configured for export")
		return nil
	}

	printSuccess(fmt.Sprintf("Successfully exported: %s", strings.Join(exported, ", ")))
	return nil
}

func runExportDB(cmd *cobra.Command, args []string) error {
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	cfg := config.Get()
	if cfg.Services.Database.Type == "" {
		return fmt.Errorf("PostgreSQL database not configured")
	}

	return exportPostgreSQL(cfg)
}

func runExportMongo(cmd *cobra.Command, args []string) error {
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	cfg := config.Get()
	if cfg.Services.MongoDB.Type == "" {
		return fmt.Errorf("MongoDB not configured")
	}

	return exportMongoDB(cfg)
}

func runExportStorage(cmd *cobra.Command, args []string) error {
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	cfg := config.Get()
	if cfg.Services.Storage.Type == "" {
		return fmt.Errorf("MinIO storage not configured")
	}

	return exportStorage(cfg)
}

func exportPostgreSQL(cfg *config.Config) error {
	outputFile := getOutputPath("db", "sql")

	// Create PostgreSQL service to get connection details
	service := postgres.NewService(&cfg.Services.Database)
	if err := service.Initialize(); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer service.Close()

	// Use pg_dump to export
	cmd := exec.Command("pg_dump",
		fmt.Sprintf("--host=localhost"),
		fmt.Sprintf("--port=%d", cfg.Services.Database.Port),
		fmt.Sprintf("--username=localcloud"),
		fmt.Sprintf("--dbname=localcloud"),
		fmt.Sprintf("--file=%s", outputFile),
		"--verbose",
		"--no-password",
	)

	// Set password via environment
	cmd.Env = append(os.Environ(), "PGPASSWORD=localcloud")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %w", err)
	}

	printSuccess(fmt.Sprintf("PostgreSQL exported to: %s", outputFile))
	return nil
}

func exportMongoDB(cfg *config.Config) error {
	outputFile := getOutputPath("mongo", "tar.gz")

	// Create MongoDB connection URI
	mongoURI := fmt.Sprintf("mongodb://localcloud:localcloud@localhost:%d/localcloud?authSource=admin", cfg.Services.MongoDB.Port)

	// Use mongodump to export
	cmd := exec.Command("mongodump",
		"--uri", mongoURI,
		"--archive="+outputFile,
		"--gzip",
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mongodump failed: %w", err)
	}

	printSuccess(fmt.Sprintf("MongoDB exported to: %s", outputFile))
	return nil
}

func exportStorage(cfg *config.Config) error {
	outputFile := getOutputPath("storage", "tar.gz")

	// Get MinIO client
	client, err := getMinIOClient()
	if err != nil {
		return fmt.Errorf("failed to connect to MinIO: %w", err)
	}

	// Create temporary directory for storage data
	tempDir, err := os.MkdirTemp("", "localcloud-storage-export")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download all buckets and objects
	ctx := context.Background()
	buckets, err := client.ListBuckets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list buckets: %w", err)
	}

	for _, bucket := range buckets {
		bucketDir := filepath.Join(tempDir, bucket.Name)
		if err := os.MkdirAll(bucketDir, 0755); err != nil {
			return fmt.Errorf("failed to create bucket directory: %w", err)
		}

		// List objects in bucket
		objectCh := client.ListObjects(ctx, bucket.Name, minio.ListObjectsOptions{
			Recursive: true,
		})
		for object := range objectCh {
			if object.Err != nil {
				printWarning(fmt.Sprintf("Error listing object: %v", object.Err))
				continue
			}

			// Download object
			objectPath := filepath.Join(bucketDir, object.Key)
			if err := os.MkdirAll(filepath.Dir(objectPath), 0755); err != nil {
				return fmt.Errorf("failed to create object directory: %w", err)
			}

			if err := client.FGetObject(ctx, bucket.Name, object.Key, objectPath, minio.GetObjectOptions{}); err != nil {
				printWarning(fmt.Sprintf("Failed to download %s: %v", object.Key, err))
				continue
			}
		}
	}

	// Create tar.gz archive
	if err := createTarGz(tempDir, outputFile); err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}

	printSuccess(fmt.Sprintf("Storage exported to: %s", outputFile))
	return nil
}

func getOutputPath(component, extension string) string {
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("localcloud-%s-%s.%s", component, timestamp, extension)

	if exportOutput == "" {
		return filename
	}

	// If output is a directory, use auto-generated filename
	if info, err := os.Stat(exportOutput); err == nil && info.IsDir() {
		return filepath.Join(exportOutput, filename)
	}

	// If output is a file path, use it directly
	return exportOutput
}

func createTarGz(sourceDir, targetFile string) error {
	file, err := os.Create(targetFile)
	if err != nil {
		return err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Update the name to maintain directory structure
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tw, file)
		return err
	})
}

func runExportVector(cmd *cobra.Command, args []string) error {
	if !IsProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	cfg := config.Get()
	if cfg.Services.Database.Type == "" {
		return fmt.Errorf("PostgreSQL database not configured (required for vector database)")
	}

	return exportVectorDatabase(cfg)
}

// VectorExportData represents the structure for vector database export
type VectorExportData struct {
	ExportInfo   ExportInfo      `json:"export_info"`
	Collections  []string        `json:"collections"`
	Embeddings   []EmbeddingData `json:"embeddings"`
	ImportScript string          `json:"import_script,omitempty"`
}

type ExportInfo struct {
	ExportedAt   time.Time `json:"exported_at"`
	Source       string    `json:"source"`
	Version      string    `json:"version"`
	TotalVectors int       `json:"total_vectors"`
	Dimension    int       `json:"dimension"`
}

type EmbeddingData struct {
	ID         string                 `json:"id"`
	DocumentID string                 `json:"document_id"`
	Content    string                 `json:"content"`
	Embedding  []float64              `json:"embedding"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Collection string                 `json:"collection"`
	CreatedAt  time.Time              `json:"created_at"`
}

func exportVectorDatabase(cfg *config.Config) error {
	outputFile := getOutputPath("vector", "json")

	// Create database connection
	connStr := fmt.Sprintf("host=localhost port=%d user=localcloud password=localcloud dbname=localcloud sslmode=disable",
		cfg.Services.Database.Port)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Build query with optional collection filter
	var query string
	var args []interface{}

	if exportCollection != "" {
		query = `
			SELECT id, document_id, content, embedding, metadata, 
			       COALESCE(collection_name, 'default') as collection_name, created_at
			FROM localcloud.embeddings 
			WHERE collection_name = $1 OR (collection_name IS NULL AND $1 = 'default')
			ORDER BY created_at`
		args = append(args, exportCollection)
	} else {
		query = `
			SELECT id, document_id, content, embedding, metadata, 
			       COALESCE(collection_name, 'default') as collection_name, created_at
			FROM localcloud.embeddings 
			ORDER BY created_at`
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("failed to query embeddings: %w", err)
	}
	defer rows.Close()

	var embeddings []EmbeddingData
	var collections = make(map[string]bool)
	var dimension int

	for rows.Next() {
		var embedding EmbeddingData
		var embeddingStr string
		var metadataStr sql.NullString

		err := rows.Scan(
			&embedding.ID,
			&embedding.DocumentID,
			&embedding.Content,
			&embeddingStr,
			&metadataStr,
			&embedding.Collection,
			&embedding.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse vector embedding from PostgreSQL array format
		if err := parsePostgresArray(embeddingStr, &embedding.Embedding); err != nil {
			printWarning(fmt.Sprintf("Failed to parse embedding for document %s: %v", embedding.DocumentID, err))
			continue
		}

		// Parse metadata JSON
		if metadataStr.Valid {
			if err := json.Unmarshal([]byte(metadataStr.String), &embedding.Metadata); err != nil {
				printWarning(fmt.Sprintf("Failed to parse metadata for document %s: %v", embedding.DocumentID, err))
			}
		}

		// Track dimension and collections
		if len(embedding.Embedding) > 0 {
			dimension = len(embedding.Embedding)
		}
		collections[embedding.Collection] = true

		embeddings = append(embeddings, embedding)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	// Convert collections map to slice
	var collectionList []string
	for collection := range collections {
		collectionList = append(collectionList, collection)
	}

	// Create export data structure
	exportData := VectorExportData{
		ExportInfo: ExportInfo{
			ExportedAt:   time.Now(),
			Source:       "LocalCloud pgvector",
			Version:      "1.0",
			TotalVectors: len(embeddings),
			Dimension:    dimension,
		},
		Collections:  collectionList,
		Embeddings:   embeddings,
		ImportScript: generateImportScript(embeddings),
	}

	// Write to JSON file
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(exportData); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	printSuccess(fmt.Sprintf("Vector database exported to: %s", outputFile))

	return nil
}

// parsePostgresArray parses PostgreSQL array format like [1.0,2.0,3.0] to []float64
func parsePostgresArray(arrayStr string, result *[]float64) error {
	// Remove brackets and split by commas
	if !strings.HasPrefix(arrayStr, "[") || !strings.HasSuffix(arrayStr, "]") {
		return fmt.Errorf("invalid array format: %s", arrayStr)
	}

	content := strings.TrimPrefix(strings.TrimSuffix(arrayStr, "]"), "[")
	if content == "" {
		*result = []float64{}
		return nil
	}

	parts := strings.Split(content, ",")
	*result = make([]float64, len(parts))

	for i, part := range parts {
		var val float64
		if _, err := fmt.Sscanf(strings.TrimSpace(part), "%f", &val); err != nil {
			return fmt.Errorf("failed to parse float: %s", part)
		}
		(*result)[i] = val
	}

	return nil
}

// generateImportScript creates a SQL script for reimporting to pgvector
func generateImportScript(embeddings []EmbeddingData) string {
	if len(embeddings) == 0 {
		return ""
	}

	var script strings.Builder

	script.WriteString("-- LocalCloud Vector Database Import Script\n")
	script.WriteString("-- Generated on: " + time.Now().Format("2006-01-02 15:04:05") + "\n\n")
	script.WriteString("-- Create embeddings table if it doesn't exist\n")
	script.WriteString("CREATE EXTENSION IF NOT EXISTS vector;\n")

	// Determine dimension from first embedding
	dimension := len(embeddings[0].Embedding)

	script.WriteString(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS localcloud.embeddings (
    id SERIAL PRIMARY KEY,
    document_id VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    embedding vector(%d),
    metadata JSONB,
    collection_name VARCHAR(100) DEFAULT 'default',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert embeddings data
`, dimension))

	for _, emb := range embeddings {
		// Convert embedding to PostgreSQL array format
		var embeddingStr strings.Builder
		embeddingStr.WriteString("[")
		for i, val := range emb.Embedding {
			if i > 0 {
				embeddingStr.WriteString(",")
			}
			embeddingStr.WriteString(fmt.Sprintf("%.6f", val))
		}
		embeddingStr.WriteString("]")

		// Convert metadata to JSON
		metadataJSON := "NULL"
		if emb.Metadata != nil {
			if jsonBytes, err := json.Marshal(emb.Metadata); err == nil {
				metadataJSON = "'" + strings.Replace(string(jsonBytes), "'", "''", -1) + "'"
			}
		}

		script.WriteString(fmt.Sprintf("INSERT INTO localcloud.embeddings (document_id, content, embedding, metadata, collection_name, created_at) VALUES ('%s', '%s', '%s', %s, '%s', '%s');\n",
			strings.Replace(emb.DocumentID, "'", "''", -1),
			strings.Replace(emb.Content, "'", "''", -1),
			embeddingStr.String(),
			metadataJSON,
			strings.Replace(emb.Collection, "'", "''", -1),
			emb.CreatedAt.Format("2006-01-02 15:04:05"),
		))
	}

	return script.String()
}
