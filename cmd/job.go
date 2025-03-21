/*
Copyright © 2025 Will Atlas <will@atls.dev>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// jobCmd represents the job command
var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Manage jobs in the queue",
	Long: `The job command allows you to manage jobs in the queue.
Use the subcommands to add, remove, list, or view output of jobs.`,
	// This is a parent command that doesn't do anything itself
	// so we'll disable the run function
	Run: nil,
}

func init() {
	rootCmd.AddCommand(jobCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// jobCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// jobCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
