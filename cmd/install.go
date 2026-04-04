/*
Copyright © 2025 Will Atlas <will@atls.dev>
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const systemdUnitTemplate = `[Unit]
Description={{.Description}}
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
ExecStart={{.ExecStart}}
Restart=on-failure
RestartSec=5
{{- if .User}}
User={{.User}}
Group={{.User}}
{{- end}}
{{- if .EnvironmentFile}}
EnvironmentFile={{.EnvironmentFile}}
{{- end}}
{{- if .Environment}}
{{- range .Environment}}
Environment="{{.}}"
{{- end}}
{{- end}}

[Install]
WantedBy=multi-user.target
`

type unitConfig struct {
	Description     string
	ExecStart       string
	User            string
	EnvironmentFile string
	Environment     []string
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install systemd service units for the worker and web server",
	Long: `The install command creates systemd unit files for the qq worker
and web server, then reloads systemd to pick up the new units.

This allows you to run qq as a system service that starts on boot.

The command must be run as root (or with sudo) to write to /etc/systemd/system/.

Examples:
  sudo qq install
  sudo qq install --user qq --env-file /etc/qq/env
  sudo qq install --addr :9090 --concurrency 4`,
	RunE: func(cmd *cobra.Command, args []string) error {
		user, _ := cmd.Flags().GetString("user")
		envFile, _ := cmd.Flags().GetString("env-file")
		addr, _ := cmd.Flags().GetString("addr")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		queue, _ := cmd.Flags().GetString("queue")

		// Resolve the qq binary path
		qqBin, _ := cmd.Flags().GetString("bin")
		if qqBin == "" {
			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to determine executable path: %w", err)
			}
			qqBin, err = filepath.EvalSymlinks(exe)
			if err != nil {
				return fmt.Errorf("failed to resolve executable path: %w", err)
			}
		}

		// Build common flags that both services need
		var commonFlags string
		dbURL := viper.GetString("db_url")
		if dbURL != "" {
			commonFlags = fmt.Sprintf(" --db-url=%s", dbURL)
		}

		configFile := viper.ConfigFileUsed()
		if configFile != "" {
			commonFlags = fmt.Sprintf(" --config=%s", configFile)
		}

		// Build worker ExecStart
		workerExec := qqBin + " worker" + commonFlags
		if concurrency > 0 {
			workerExec += fmt.Sprintf(" --concurrency=%d", concurrency)
		}
		if queue != "" {
			workerExec += fmt.Sprintf(" --queue=%s", queue)
		}

		// Build server ExecStart
		serverExec := qqBin + " server" + commonFlags
		if addr != "" {
			serverExec += fmt.Sprintf(" --addr=%s", addr)
		}

		tmpl, err := template.New("unit").Parse(systemdUnitTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse unit template: %w", err)
		}

		// Write worker unit
		workerUnit := unitConfig{
			Description: "qq job queue worker",
			ExecStart:   workerExec,
			User:        user,
			EnvironmentFile: envFile,
		}
		if err := writeUnit(tmpl, "/etc/systemd/system/qq-worker.service", workerUnit); err != nil {
			return err
		}
		fmt.Println("Wrote /etc/systemd/system/qq-worker.service")

		// Write server unit
		serverUnit := unitConfig{
			Description: "qq web dashboard server",
			ExecStart:   serverExec,
			User:        user,
			EnvironmentFile: envFile,
		}
		if err := writeUnit(tmpl, "/etc/systemd/system/qq-server.service", serverUnit); err != nil {
			return err
		}
		fmt.Println("Wrote /etc/systemd/system/qq-server.service")

		// Reload systemd
		fmt.Println("Reloading systemd daemon...")
		reload := exec.Command("systemctl", "daemon-reload")
		reload.Stdout = os.Stdout
		reload.Stderr = os.Stderr
		if err := reload.Run(); err != nil {
			return fmt.Errorf("failed to reload systemd: %w", err)
		}

		fmt.Println("\nServices installed successfully.")
		fmt.Println("To start them:")
		fmt.Println("  sudo systemctl start qq-worker")
		fmt.Println("  sudo systemctl start qq-server")
		fmt.Println("\nTo enable on boot:")
		fmt.Println("  sudo systemctl enable qq-worker qq-server")

		return nil
	},
}

func writeUnit(tmpl *template.Template, path string, cfg unitConfig) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w (are you running as root?)", path, err)
	}
	defer f.Close()
	return tmpl.Execute(f, cfg)
}

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().String("user", "", "User to run the services as")
	installCmd.Flags().String("env-file", "", "Path to environment file for the services")
	installCmd.Flags().String("bin", "", "Path to the qq binary (default: current executable)")
	installCmd.Flags().String("addr", ":8080", "Address for the web server to listen on")
	installCmd.Flags().IntP("concurrency", "c", 0, "Worker concurrency (0 uses default)")
	installCmd.Flags().StringP("queue", "q", "", "Queue for the worker to process (default: all)")
}
