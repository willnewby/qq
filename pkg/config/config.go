package config

import (
	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Database DatabaseConfig
	Worker   WorkerConfig
	Server   ServerConfig
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	URL string
}

// WorkerConfig holds worker settings
type WorkerConfig struct {
	ID          string
	Concurrency int
	Queues      []string
	Interval    int // in seconds
}

// ServerConfig holds server settings
type ServerConfig struct {
	Address string
}

// LoadConfig loads the application configuration from viper
func LoadConfig() (*Config, error) {
	config := &Config{
		Database: DatabaseConfig{
			URL: viper.GetString("db_url"),
		},
		Worker: WorkerConfig{
			ID:          viper.GetString("worker.id"),
			Concurrency: viper.GetInt("worker.concurrency"),
			Queues:      viper.GetStringSlice("worker.queues"),
			Interval:    viper.GetInt("worker.interval"),
		},
		Server: ServerConfig{
			Address: viper.GetString("server.address"),
		},
	}

	// Backward compat: if worker.queues is empty, fall back to worker.queue (singular)
	if len(config.Worker.Queues) == 0 {
		if q := viper.GetString("worker.queue"); q != "" {
			config.Worker.Queues = []string{q}
		}
	}

	return config, nil
}
