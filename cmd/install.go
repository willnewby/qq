/*
Copyright © 2025 Will Atlas <will@atls.dev>
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

const launchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
{{- range .ProgramArguments}}
		<string>{{.}}</string>
{{- end}}
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>ThrottleInterval</key>
	<integer>5</integer>
{{- if .WorkingDirectory}}
	<key>WorkingDirectory</key>
	<string>{{.WorkingDirectory}}</string>
{{- end}}
{{- if .StandardOutPath}}
	<key>StandardOutPath</key>
	<string>{{.StandardOutPath}}</string>
{{- end}}
{{- if .StandardErrorPath}}
	<key>StandardErrorPath</key>
	<string>{{.StandardErrorPath}}</string>
{{- end}}
{{- if .EnvironmentVariables}}
	<key>EnvironmentVariables</key>
	<dict>
{{- range $key, $value := .EnvironmentVariables}}
		<key>{{$key}}</key>
		<string>{{$value}}</string>
{{- end}}
	</dict>
{{- end}}
</dict>
</plist>
`

type unitConfig struct {
	Description     string
	ExecStart       string
	User            string
	EnvironmentFile string
	Environment     []string
}

type plistConfig struct {
	Label                string
	ProgramArguments     []string
	WorkingDirectory     string
	StandardOutPath      string
	StandardErrorPath    string
	EnvironmentVariables map[string]string
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install service units for the worker and web server",
	Long: `The install command creates service files for the qq worker and web server.

On Linux, it creates systemd unit files in /etc/systemd/system/ and reloads
systemd. This must be run as root (or with sudo).

On macOS, it creates launchd plist files in ~/Library/LaunchAgents/ and loads
them via launchctl. These run as the current user and start on login.

Examples:
  # Linux (systemd)
  sudo qq install
  sudo qq install --user qq --env-file /etc/qq/env

  # macOS (launchd)
  qq install
  qq install --log-dir ~/Library/Logs/qq
  qq install --concurrency 4`,
	RunE: func(cmd *cobra.Command, args []string) error {
		switch runtime.GOOS {
		case "linux":
			return installSystemd(cmd)
		case "darwin":
			return installLaunchd(cmd)
		default:
			return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
		}
	},
}

func resolveQQBin(cmd *cobra.Command) (string, error) {
	qqBin, _ := cmd.Flags().GetString("bin")
	if qqBin != "" {
		return qqBin, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to determine executable path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable path: %w", err)
	}
	return resolved, nil
}

func buildCommonFlags() string {
	var flags string
	dbURL := viper.GetString("db_url")
	if dbURL != "" {
		flags = fmt.Sprintf(" --db-url=%s", dbURL)
	}
	configFile := viper.ConfigFileUsed()
	if configFile != "" {
		flags = fmt.Sprintf(" --config=%s", configFile)
	}
	return flags
}

func installSystemd(cmd *cobra.Command) error {
	user, _ := cmd.Flags().GetString("user")
	envFile, _ := cmd.Flags().GetString("env-file")
	addr, _ := cmd.Flags().GetString("addr")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	queue, _ := cmd.Flags().GetString("queue")
	workerID, _ := cmd.Flags().GetString("id")

	qqBin, err := resolveQQBin(cmd)
	if err != nil {
		return err
	}

	commonFlags := buildCommonFlags()

	workerExec := qqBin + " worker" + commonFlags
	if workerID != "" {
		workerExec += fmt.Sprintf(" --id=%s", workerID)
	}
	if concurrency > 0 {
		workerExec += fmt.Sprintf(" --concurrency=%d", concurrency)
	}
	if queue != "" {
		workerExec += fmt.Sprintf(" --queue=%s", queue)
	}

	serverExec := qqBin + " server" + commonFlags
	if addr != "" {
		serverExec += fmt.Sprintf(" --addr=%s", addr)
	}

	tmpl, err := template.New("unit").Parse(systemdUnitTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse unit template: %w", err)
	}

	workerUnit := unitConfig{
		Description:     "qq job queue worker",
		ExecStart:       workerExec,
		User:            user,
		EnvironmentFile: envFile,
	}
	if err := writeUnit(tmpl, "/etc/systemd/system/qq-worker.service", workerUnit); err != nil {
		return err
	}
	fmt.Println("Wrote /etc/systemd/system/qq-worker.service")

	serverUnit := unitConfig{
		Description:     "qq web dashboard server",
		ExecStart:       serverExec,
		User:            user,
		EnvironmentFile: envFile,
	}
	if err := writeUnit(tmpl, "/etc/systemd/system/qq-server.service", serverUnit); err != nil {
		return err
	}
	fmt.Println("Wrote /etc/systemd/system/qq-server.service")

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
}

func installLaunchd(cmd *cobra.Command) error {
	addr, _ := cmd.Flags().GetString("addr")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	queue, _ := cmd.Flags().GetString("queue")
	workerID, _ := cmd.Flags().GetString("id")
	logDir, _ := cmd.Flags().GetString("log-dir")

	qqBin, err := resolveQQBin(cmd)
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine home directory: %w", err)
	}

	if logDir == "" {
		logDir = filepath.Join(home, "Library", "Logs", "qq")
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory %s: %w", logDir, err)
	}

	// Build environment variables for plist
	envVars := make(map[string]string)
	dbURL := viper.GetString("db_url")
	if dbURL != "" {
		envVars["DATABASE_URL"] = dbURL
	}
	configFile := viper.ConfigFileUsed()
	if configFile != "" {
		envVars["QQ_CONFIG"] = configFile
	}

	// Build worker arguments
	workerArgs := []string{qqBin, "worker"}
	if dbURL != "" {
		workerArgs = append(workerArgs, fmt.Sprintf("--db-url=%s", dbURL))
	}
	if configFile != "" {
		workerArgs = append(workerArgs, fmt.Sprintf("--config=%s", configFile))
	}
	if workerID != "" {
		workerArgs = append(workerArgs, fmt.Sprintf("--id=%s", workerID))
	}
	if concurrency > 0 {
		workerArgs = append(workerArgs, fmt.Sprintf("--concurrency=%d", concurrency))
	}
	if queue != "" {
		workerArgs = append(workerArgs, fmt.Sprintf("--queue=%s", queue))
	}

	// Build server arguments
	serverArgs := []string{qqBin, "server"}
	if dbURL != "" {
		serverArgs = append(serverArgs, fmt.Sprintf("--db-url=%s", dbURL))
	}
	if configFile != "" {
		serverArgs = append(serverArgs, fmt.Sprintf("--config=%s", configFile))
	}
	if addr != "" {
		serverArgs = append(serverArgs, fmt.Sprintf("--addr=%s", addr))
	}

	tmpl, err := template.New("plist").Parse(launchdPlistTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse plist template: %w", err)
	}

	agentsDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	// Worker plist
	workerLabel := "dev.atls.qq.worker"
	workerPath := filepath.Join(agentsDir, workerLabel+".plist")
	workerCfg := plistConfig{
		Label:             workerLabel,
		ProgramArguments:  workerArgs,
		StandardOutPath:   filepath.Join(logDir, "worker.log"),
		StandardErrorPath: filepath.Join(logDir, "worker.err.log"),
		EnvironmentVariables: envVars,
	}

	// Unload existing agents before overwriting (ignore errors — may not be loaded)
	unloadPlist(workerPath)

	if err := writePlist(tmpl, workerPath, workerCfg); err != nil {
		return err
	}
	fmt.Printf("Wrote %s\n", workerPath)

	// Server plist
	serverLabel := "dev.atls.qq.server"
	serverPath := filepath.Join(agentsDir, serverLabel+".plist")
	serverCfg := plistConfig{
		Label:             serverLabel,
		ProgramArguments:  serverArgs,
		StandardOutPath:   filepath.Join(logDir, "server.log"),
		StandardErrorPath: filepath.Join(logDir, "server.err.log"),
		EnvironmentVariables: envVars,
	}

	unloadPlist(serverPath)

	if err := writePlist(tmpl, serverPath, serverCfg); err != nil {
		return err
	}
	fmt.Printf("Wrote %s\n", serverPath)

	// Load the new plists
	for _, path := range []string{workerPath, serverPath} {
		load := exec.Command("launchctl", "load", path)
		load.Stdout = os.Stdout
		load.Stderr = os.Stderr
		if err := load.Run(); err != nil {
			return fmt.Errorf("failed to load %s: %w", path, err)
		}
	}

	fmt.Println("\nServices installed and started.")
	fmt.Println("They will start automatically on login.")
	fmt.Printf("Logs: %s\n", logDir)
	fmt.Println("\nTo stop them:")
	fmt.Printf("  launchctl unload %s\n", workerPath)
	fmt.Printf("  launchctl unload %s\n", serverPath)
	fmt.Println("\nTo uninstall:")
	fmt.Printf("  launchctl unload %s && rm %s\n", workerPath, workerPath)
	fmt.Printf("  launchctl unload %s && rm %s\n", serverPath, serverPath)

	return nil
}

func writeUnit(tmpl *template.Template, path string, cfg unitConfig) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w (are you running as root?)", path, err)
	}
	defer f.Close()
	return tmpl.Execute(f, cfg)
}

func writePlist(tmpl *template.Template, path string, cfg plistConfig) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	defer f.Close()
	return tmpl.Execute(f, cfg)
}

func unloadPlist(path string) {
	if _, err := os.Stat(path); err == nil {
		cmd := exec.Command("launchctl", "unload", path)
		_ = cmd.Run()
	}
}

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().String("user", "", "User to run the services as (Linux only)")
	installCmd.Flags().String("env-file", "", "Path to environment file (Linux only)")
	installCmd.Flags().String("bin", "", "Path to the qq binary (default: current executable)")
	installCmd.Flags().String("addr", ":8080", "Address for the web server to listen on")
	installCmd.Flags().String("id", "", "Worker ID for identifying the worker instance")
	installCmd.Flags().IntP("concurrency", "c", 0, "Worker concurrency (0 uses default)")
	installCmd.Flags().StringP("queue", "q", "", "Comma-separated list of queues for the worker to process (default: default)")
	installCmd.Flags().String("log-dir", "", "Log directory for service output (macOS only, default: ~/Library/Logs/qq)")
}
