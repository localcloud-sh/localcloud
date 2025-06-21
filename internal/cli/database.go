// internal/cli/database.go
package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/localcloud/localcloud/internal/config"
	"github.com/localcloud/localcloud/internal/services/postgres"
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management commands",
	Long:  `Manage PostgreSQL database including connections, backups, and migrations.`,
}

var dbConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to the database",
	Long:  `Open an interactive PostgreSQL session using psql.`,
	RunE:  runDBConnect,
}

var dbBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup the database",
	Long:  `Create a backup of the LocalCloud database.`,
	RunE:  runDBBackup,
}

var dbRestoreCmd = &cobra.Command{
	Use:   "restore [backup-file]",
	Short: "Restore a database backup",
	Long:  `Restore the LocalCloud database from a backup file.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runDBRestore,
}

var dbMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  `Apply pending database migrations.`,
	RunE:  runDBMigrate,
}

var dbStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status",
	Long:  `Display the status of database migrations.`,
	RunE:  runDBStatus,
}

func init() {
	// Add subcommands
	dbCmd.AddCommand(dbConnectCmd)
	dbCmd.AddCommand(dbBackupCmd)
	dbCmd.AddCommand(dbRestoreCmd)
	dbCmd.AddCommand(dbMigrateCmd)
	dbCmd.AddCommand(dbStatusCmd)

	// Add to root command
	rootCmd.AddCommand(dbCmd)
}

func runDBConnect(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !isProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	// Debug: Config'i kontrol et
	fmt.Printf("Config file: %s\n", configFile)
	fmt.Printf("Project path: %s\n", projectPath)

	// Get config
	cfg := config.Get()

	// Debug: Config içeriğini yazdır
	fmt.Printf("Database Type: %s\n", cfg.Services.Database.Type)

	if cfg.Services.Database.Type == "" {
		return fmt.Errorf("database service not configured")
	}

	// Create PostgreSQL service
	service := postgres.NewService(&cfg.Services.Database)

	// Initialize (this connects to the database)
	if err := service.Initialize(); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer service.Close()

	// Create client and connect
	client := postgres.NewClient(service)

	printInfo("Connecting to PostgreSQL database...")
	return client.Connect()
}

func runDBBackup(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !isProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	// Get config
	cfg := config.Get()
	if cfg.Services.Database.Type == "" {
		return fmt.Errorf("database service not configured")
	}

	// Create PostgreSQL service
	service := postgres.NewService(&cfg.Services.Database)
	if err := service.Initialize(); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer service.Close()

	// Create backup filename
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("localcloud_backup_%s.sql", timestamp)
	backupPath := filepath.Join(".localcloud", "backups", filename)

	// Create client and backup
	client := postgres.NewClient(service)

	printInfo(fmt.Sprintf("Creating backup: %s", backupPath))
	if err := client.Backup(backupPath); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Backup created successfully: %s", backupPath))
	return nil
}

func runDBRestore(cmd *cobra.Command, args []string) error {
	backupFile := args[0]

	// Check if project is initialized
	if !isProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	// Get config
	cfg := config.Get()
	if cfg.Services.Database.Type == "" {
		return fmt.Errorf("database service not configured")
	}

	// Confirmation prompt
	fmt.Printf("⚠️  This will overwrite the current database. Continue? (y/N): ")
	var response string
	fmt.Scanln(&response)
	if response != "y" && response != "Y" {
		fmt.Println("Restore cancelled")
		return nil
	}

	// Create PostgreSQL service
	service := postgres.NewService(&cfg.Services.Database)
	if err := service.Initialize(); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer service.Close()

	// Create client and restore
	client := postgres.NewClient(service)

	printInfo(fmt.Sprintf("Restoring from: %s", backupFile))
	if err := client.Restore(backupFile); err != nil {
		return err
	}

	printSuccess("Database restored successfully")
	return nil
}

func runDBMigrate(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !isProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	// Get config
	cfg := config.Get()
	if cfg.Services.Database.Type == "" {
		return fmt.Errorf("database service not configured")
	}

	// Create PostgreSQL service
	service := postgres.NewService(&cfg.Services.Database)
	if err := service.Initialize(); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer service.Close()

	// Create migration manager
	client := postgres.NewClient(service)
	migrator := postgres.NewMigrationManager(client)

	// Run migrations
	migrationsPath := filepath.Join(projectPath, "migrations")
	printInfo("Running database migrations...")

	if err := migrator.Migrate(migrationsPath); err != nil {
		return err
	}

	printSuccess("Migrations completed")
	return nil
}

func runDBStatus(cmd *cobra.Command, args []string) error {
	// Check if project is initialized
	if !isProjectInitialized() {
		return fmt.Errorf("no LocalCloud project found")
	}

	// Get config
	cfg := config.Get()
	if cfg.Services.Database.Type == "" {
		return fmt.Errorf("database service not configured")
	}

	// Create PostgreSQL service
	service := postgres.NewService(&cfg.Services.Database)
	if err := service.Initialize(); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer service.Close()

	// Show migration status
	client := postgres.NewClient(service)
	migrator := postgres.NewMigrationManager(client)

	return migrator.Status()
}
