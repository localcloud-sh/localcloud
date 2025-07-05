// internal/cli/export.go
package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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

var (
	exportOutput string
)

func init() {
	// Add flags
	exportCmd.PersistentFlags().StringVarP(&exportOutput, "output", "o", "", "Output file or directory (default: current directory with auto-generated filenames)")

	// Add subcommands
	exportCmd.AddCommand(exportAllCmd)
	exportCmd.AddCommand(exportDBCmd)
	exportCmd.AddCommand(exportMongoCmd)
	exportCmd.AddCommand(exportStorageCmd)

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
	cmd.Env = append(os.Environ(), "PGPASSWORD=localcloud-dev")

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
