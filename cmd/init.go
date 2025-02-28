/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"qq/pkg/database"
	"qq/pkg/migration"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the queue database schema",
	Long: `The init command creates the necessary database schema for the queue.
It must be run before starting any workers or adding jobs.

Example:
  qq init --db-url=postgres://localhost:5432/mydb`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Initializing queue database schema...")

		// Create context
		ctx := context.Background()

		// Get database URL
		dbURL := viper.GetString("db_url")
		if dbURL == "" {
			fmt.Println("Database URL is required. Use --db-url flag or set it in the config file.")
			os.Exit(1)
		}

		// Connect to the database
		fmt.Println("Connecting to the database...")
		db, err := database.New(ctx, dbURL)
		if err != nil {
			fmt.Printf("Failed to connect to the database: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()

		// Create the schema
		fmt.Println("Creating River Queue schema...")
		if err := migration.CreateSchema(ctx, db.Pool); err != nil {
			fmt.Printf("Failed to create schema: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Initialization complete! The database is now ready for use.")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
