/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

		// Get database URL
		dbURL := viper.GetString("db_url")
		if dbURL == "" {
			fmt.Println("Database URL is required. Use --db-url flag or set it in the config file.")
			os.Exit(1)
		}

		// Connect to the database
		ctx := context.Background()
		config, err := pgxpool.ParseConfig(dbURL)
		if err != nil {
			fmt.Printf("Failed to parse database URL: %v\n", err)
			os.Exit(1)
		}

		pool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			fmt.Printf("Failed to connect to database: %v\n", err)
			os.Exit(1)
		}
		defer pool.Close()

		// Run River migration
		fmt.Println("Running River migration...")
		driver := riverpgxv5.New(pool)
		migrator, err := rivermigrate.New(driver, &rivermigrate.Config{})
		if err != nil {
			fmt.Printf("Failed to create migrator: %v\n", err)
			os.Exit(1)
		}

		// Run the migrations
		if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
			fmt.Printf("Failed to run River migration: %v\n", err)
			os.Exit(1)
		}

		// Create our custom job_results table
		fmt.Println("Creating job_results table...")
		_, err = pool.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS job_results (
				job_id BIGINT NOT NULL,
				attempt INT NOT NULL,
				output TEXT,
				exit_code INT,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY (job_id, attempt),
				CONSTRAINT fk_job FOREIGN KEY (job_id) REFERENCES river_job(id) ON DELETE CASCADE
			);
		`)
		if err != nil {
			fmt.Printf("Failed to create job_results table: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Initialization complete! The database is now ready for use.")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
