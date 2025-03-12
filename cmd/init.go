/*
Copyright © 2025 Will Atlas <will@atls.dev>
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

You can provide the database connection in two ways:
1. Using the --db-url flag
2. Using the DATABASE_URL environment variable

Examples:
  qq init --db-url=postgres://localhost:5432/mydb
  
  # Or using environment variable:
  export DATABASE_URL=postgres://localhost:5432/mydb
  qq init`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Initializing queue database schema...")

		// Get database URL
		dbURL := viper.GetString("db_url")
		if dbURL == "" {
			fmt.Println("Database URL is required. You can provide it in one of these ways:")
			fmt.Println("1. --db-url flag: qq init --db-url=postgres://user:password@localhost:5432/mydb")
			fmt.Println("2. DATABASE_URL environment variable: export DATABASE_URL=postgres://user:password@localhost:5432/mydb")
			fmt.Println("3. Config file (.qq.yaml) with db_url setting")
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

		// Check if River tables already exist
		var riverTablesExist bool
		err = pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'river_client')").Scan(&riverTablesExist)
		if err != nil {
			fmt.Printf("Failed to check if River tables exist: %v\n", err)
			os.Exit(1)
		}

		if riverTablesExist {
			fmt.Println("River schema already exists. Skipping migration.")
		} else {
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
			
			// Verify migration by checking if key River tables exist
			err = pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'river_client')").Scan(&riverTablesExist)
			if err != nil {
				fmt.Printf("Failed to verify River table creation: %v\n", err)
				os.Exit(1)
			}
			
			if !riverTablesExist {
				fmt.Println("Error: River tables were not created by migration")
				os.Exit(1)
			}
			
			fmt.Println("River migration completed successfully.")
		}

		// Check if job_results table already exists
		var tableExists bool
		err = pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'job_results')").Scan(&tableExists)
		if err != nil {
			fmt.Printf("Failed to check if job_results table exists: %v\n", err)
			os.Exit(1)
		}

		// Create our custom job_results table
		fmt.Println("Creating job_results table (if not exists)...")
		
		// First create the table without constraints
		_, err = pool.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS job_results (
				job_id BIGINT NOT NULL,
				attempt INT NOT NULL,
				output TEXT,
				exit_code INT,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY (job_id, attempt)
			);
		`)
		if err != nil {
			fmt.Printf("Failed to create job_results table: %v\n", err)
			os.Exit(1)
		}
		
		// Check if River job table exists for foreign key constraint
		var riverJobTableExists bool
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'river_job'
				   OR table_name = 'river_queue_job'
			)
		`).Scan(&riverJobTableExists)
		if err != nil {
			fmt.Printf("Failed to check if River job table exists: %v\n", err)
			os.Exit(1)
		}

		// Only add foreign key constraint if river_job table exists
		if !riverJobTableExists {
			fmt.Println("Skipping foreign key constraint creation since no River job table was found.")
			fmt.Println("Please run qq init again after the River job table is created to add the constraint.")
		} else {
			// Get the actual job table name
			var jobTableName string
			err = pool.QueryRow(ctx, `
				SELECT table_name FROM information_schema.columns
				WHERE table_name IN ('river_job', 'river_queue_job')
				LIMIT 1
			`).Scan(&jobTableName)
			if err != nil {
				fmt.Printf("Failed to get job table name: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Found River job table: %s\n", jobTableName)

			// Then add the foreign key constraint separately to handle case where it might already exist
			if tableExists {
				fmt.Println("Job results table already exists. Checking if constraint needs to be added...")
				
				// Check if the constraint already exists
				var constraintExists bool
				err = pool.QueryRow(ctx, `
					SELECT EXISTS (
						SELECT 1 
						FROM information_schema.table_constraints 
						WHERE constraint_name = 'fk_job' 
						AND table_name = 'job_results'
					)
				`).Scan(&constraintExists)
				
				if err != nil {
					fmt.Printf("Failed to check if constraint exists: %v\n", err)
					fmt.Println("Continuing anyway...")
				} else if constraintExists {
					fmt.Println("Foreign key constraint already exists.")
				} else {
					fmt.Println("Adding foreign key constraint...")
					_, err = pool.Exec(ctx, fmt.Sprintf(`
						ALTER TABLE job_results 
						ADD CONSTRAINT fk_job 
						FOREIGN KEY (job_id) REFERENCES %s(id) ON DELETE CASCADE;
					`, jobTableName))
					if err != nil {
						fmt.Printf("Failed to add foreign key constraint: %v\n", err)
						fmt.Println("Continuing anyway...")
					} else {
						fmt.Println("Foreign key constraint added successfully.")
					}
				}
			} else {
				fmt.Println("Adding foreign key constraint...")
				_, err = pool.Exec(ctx, fmt.Sprintf(`
					ALTER TABLE job_results 
					ADD CONSTRAINT fk_job 
					FOREIGN KEY (job_id) REFERENCES %s(id) ON DELETE CASCADE;
				`, jobTableName))
				if err != nil {
					fmt.Printf("Failed to add foreign key constraint: %v\n", err)
					// Not failing if the constraint already exists or can't be created
					fmt.Println("Continuing anyway...")
				} else {
					fmt.Println("Foreign key constraint added successfully.")
				}
			}
		}

		fmt.Println("Initialization complete! The database is now ready for use.")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}