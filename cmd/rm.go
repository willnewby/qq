/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// jobRmCmd represents the job rm command
var jobRmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Remove a job from the queue",
	Long: `Remove a job from the queue based on its ID.

Example:
  qq job rm 12345`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("Error: job ID is required")
			return
		}

		jobID := args[0]
		force, _ := cmd.Flags().GetBool("force")
		
		fmt.Printf("Removing job: %s\n", jobID)
		if force {
			fmt.Println("Force option enabled, will remove even if job is running")
		}

		// TODO: Implement job removal functionality
	},
}

// queueRmCmd represents the queue rm command
var queueRmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Remove a queue",
	Long: `Remove a queue from the system.

Example:
  qq queue rm low_priority`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("Error: queue name is required")
			return
		}

		queueName := args[0]
		force, _ := cmd.Flags().GetBool("force")
		
		fmt.Printf("Removing queue: %s\n", queueName)
		if force {
			fmt.Println("Force option enabled, will remove even if queue has jobs")
		}

		// TODO: Implement queue removal functionality
	},
}

func init() {
	// Add jobRmCmd to the job command
	jobCmd.AddCommand(jobRmCmd)

	// Add queueRmCmd to the queue command
	queueCmd.AddCommand(queueRmCmd)

	// Add flags
	jobRmCmd.Flags().BoolP("force", "f", false, "Force removal even if job is running")
	queueRmCmd.Flags().BoolP("force", "f", false, "Force removal even if queue has jobs")
}
