// internal/cli/storage.go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/cobra"
)

// StorageCredentials holds MinIO connection information
type StorageCredentials struct {
	Endpoint   string `json:"endpoint"`
	AccessKey  string `json:"access_key"`
	SecretKey  string `json:"secret_key"`
	UseSSL     bool   `json:"use_ssl"`
	ConsoleURL string `json:"console_url"`
	BucketName string `json:"default_bucket"`
}

var storageCmd = &cobra.Command{
	Use:     "storage",
	Short:   "Manage storage service",
	Aliases: []string{"s3", "minio"},
	Long: `Manage MinIO object storage service.
	
MinIO provides S3-compatible object storage for your LocalCloud project.
You can store files, images, and other assets locally with full S3 API compatibility.`,
}

var storageConsoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Open MinIO web console",
	Long:  `Opens the MinIO web console in your default browser.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, err := getStorageCredentials()
		if err != nil {
			return fmt.Errorf("storage service not running or credentials not found: %w", err)
		}

		printInfo(fmt.Sprintf("Opening MinIO console at %s", creds.ConsoleURL))
		printInfo(fmt.Sprintf("Username: %s", creds.AccessKey))
		printInfo(fmt.Sprintf("Password: %s", creds.SecretKey))

		return openBrowser(creds.ConsoleURL)
	},
}

var storageMakeBucketCmd = &cobra.Command{
	Use:   "mb [bucket-name]",
	Short: "Make a new bucket",
	Long:  `Creates a new bucket in MinIO storage.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bucketName := args[0]

		client, err := getMinIOClient()
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}

		printSuccess(fmt.Sprintf("Bucket '%s' created successfully", bucketName))
		return nil
	},
}

var storageListBucketsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all buckets",
	Long:  `Lists all buckets in MinIO storage.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getMinIOClient()
		if err != nil {
			return err
		}

		ctx := context.Background()
		buckets, err := client.ListBuckets(ctx)
		if err != nil {
			return fmt.Errorf("failed to list buckets: %w", err)
		}

		if len(buckets) == 0 {
			printInfo("No buckets found")
			return nil
		}

		// Simple table output without external dependency
		fmt.Printf("\n%-30s %s\n", "BUCKET NAME", "CREATED")
		fmt.Println(strings.Repeat("-", 50))

		for _, bucket := range buckets {
			fmt.Printf("%-30s %s\n",
				bucket.Name,
				bucket.CreationDate.Format("2006-01-02 15:04:05"),
			)
		}
		fmt.Println()

		return nil
	},
}

var storageRemoveBucketCmd = &cobra.Command{
	Use:     "rb [bucket-name]",
	Short:   "Remove a bucket",
	Long:    `Removes an empty bucket from MinIO storage.`,
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"rmb"},
	RunE: func(cmd *cobra.Command, args []string) error {
		bucketName := args[0]

		client, err := getMinIOClient()
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.RemoveBucket(ctx, bucketName); err != nil {
			return fmt.Errorf("failed to remove bucket: %w", err)
		}

		printSuccess(fmt.Sprintf("Bucket '%s' removed successfully", bucketName))
		return nil
	},
}

var storagePutCmd = &cobra.Command{
	Use:   "put [local-file] [bucket/path]",
	Short: "Upload a file to storage",
	Long:  `Uploads a local file to MinIO storage.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		localFile := args[0]
		remotePath := args[1]

		// Parse bucket and object name
		parts := strings.SplitN(remotePath, "/", 2)
		if len(parts) < 2 {
			return fmt.Errorf("remote path must be in format: bucket/path/to/file")
		}
		bucketName := parts[0]
		objectName := parts[1]

		client, err := getMinIOClient()
		if err != nil {
			return err
		}

		ctx := context.Background()
		info, err := client.FPutObject(ctx, bucketName, objectName, localFile, minio.PutObjectOptions{})
		if err != nil {
			return fmt.Errorf("failed to upload file: %w", err)
		}

		printSuccess(fmt.Sprintf("Uploaded '%s' to '%s' (%d bytes)", localFile, remotePath, info.Size))
		return nil
	},
}

var storageGetCmd = &cobra.Command{
	Use:   "get [bucket/path] [local-file]",
	Short: "Download a file from storage",
	Long:  `Downloads a file from MinIO storage to local filesystem.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		remotePath := args[0]
		localFile := args[1]

		// Parse bucket and object name
		parts := strings.SplitN(remotePath, "/", 2)
		if len(parts) < 2 {
			return fmt.Errorf("remote path must be in format: bucket/path/to/file")
		}
		bucketName := parts[0]
		objectName := parts[1]

		client, err := getMinIOClient()
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.FGetObject(ctx, bucketName, objectName, localFile, minio.GetObjectOptions{}); err != nil {
			return fmt.Errorf("failed to download file: %w", err)
		}

		printSuccess(fmt.Sprintf("Downloaded '%s' to '%s'", remotePath, localFile))
		return nil
	},
}

var storageInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show storage connection information",
	Long:  `Displays MinIO connection information and example configurations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, err := getStorageCredentials()
		if err != nil {
			return fmt.Errorf("storage service not running or credentials not found: %w", err)
		}

		fmt.Println("\nMinIO Storage Configuration:")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("Endpoint: %s\n", infoColor(creds.Endpoint))
		fmt.Printf("Access Key: %s\n", infoColor(creds.AccessKey))
		fmt.Printf("Secret Key: %s\n", infoColor(creds.SecretKey))
		fmt.Printf("Default Bucket: %s\n", infoColor(creds.BucketName))
		fmt.Printf("Console URL: %s\n", infoColor(creds.ConsoleURL))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		fmt.Println("\nExample S3 SDK Configuration:")
		fmt.Println("```go")
		fmt.Printf(`client, err := minio.New("%s", &minio.Options{
    Creds:  credentials.NewStaticV4("%s", "%s", ""),
    Secure: false,
})`,
			strings.TrimPrefix(creds.Endpoint, "http://"),
			creds.AccessKey,
			creds.SecretKey)
		fmt.Println("\n```")

		fmt.Println("\nExample AWS SDK Configuration:")
		fmt.Println("```javascript")
		fmt.Printf(`const s3Client = new AWS.S3({
    endpoint: '%s',
    accessKeyId: '%s',
    secretAccessKey: '%s',
    s3ForcePathStyle: true,
    signatureVersion: 'v4'
});`, creds.Endpoint, creds.AccessKey, creds.SecretKey)
		fmt.Println("\n```")

		return nil
	},
}

func init() {
	storageCmd.AddCommand(storageConsoleCmd)
	storageCmd.AddCommand(storageMakeBucketCmd)
	storageCmd.AddCommand(storageListBucketsCmd)
	storageCmd.AddCommand(storageRemoveBucketCmd)
	storageCmd.AddCommand(storagePutCmd)
	storageCmd.AddCommand(storageGetCmd)
	storageCmd.AddCommand(storageInfoCmd)
}

// Helper functions

func getStorageCredentials() (*StorageCredentials, error) {
	credsPath := filepath.Join(".localcloud", "storage-credentials.json")
	data, err := os.ReadFile(credsPath)
	if err != nil {
		return nil, err
	}

	var creds StorageCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

func getMinIOClient() (*minio.Client, error) {
	creds, err := getStorageCredentials()
	if err != nil {
		return nil, err
	}

	endpoint := strings.TrimPrefix(creds.Endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(creds.AccessKey, creds.SecretKey, ""),
		Secure: creds.UseSSL,
	})
}
