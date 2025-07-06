// internal/cli/database.go
package cli

import (
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management commands",
	Long:  `Manage PostgreSQL database operations. Use 'lc export db' for database exports.`,
}

func init() {
	// Add to root command
	rootCmd.AddCommand(dbCmd)
}
