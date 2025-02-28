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
	Concurrency int
	Queue       string
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
			Concurrency: viper.GetInt("worker.concurrency"),
			Queue:       viper.GetString("worker.queue"),
			Interval:    viper.GetInt("worker.interval"),
		},
		Server: ServerConfig{
			Address: viper.GetString("server.address"),
		},
	}

	return config, nil
}
