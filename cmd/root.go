/*
Copyright © 2025 Will Atlas <will@atls.dev>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Version is set during build using ldflags
var Version = "dev"

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "qq",
	Short:   "A simple, fast job queue for executing bash commands",
	Version: Version,
	Long: `qq is a simple, fast job queue based on River Queue, Cobra and Viper.
It is designed to be used in a distributed environment where you have multiple 
workers running on different machines. All data persistence is done using Postgres.

qq allows you to queue, monitor, and execute bash commands reliably.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.qq.yaml)")
	rootCmd.PersistentFlags().String("db-url", "", "database connection URL")
	viper.BindPFlag("db_url", rootCmd.PersistentFlags().Lookup("db-url"))

	// Enable command completion
	rootCmd.AddCommand(completionCmd)

	// Remove the toggle flag as it's not needed
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".qq" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".qq")
	}

	// Explicitly map DATABASE_URL to db_url
	viper.BindEnv("db_url", "DATABASE_URL")
	
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
