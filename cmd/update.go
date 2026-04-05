/*
Copyright © 2025 Will Atlas <will@atls.dev>
*/
package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Download and install the latest version of qq from GitHub",
	Long: `The update command downloads the latest qq release from GitHub
and replaces the current binary.

It automatically detects your platform (OS and architecture) and downloads
the appropriate binary from https://github.com/willnewby/qq/releases.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		osName, err := releasePlatform()
		if err != nil {
			return err
		}
		arch, err := releaseArch()
		if err != nil {
			return err
		}

		url := fmt.Sprintf(
			"https://github.com/willnewby/qq/releases/latest/download/qq_%s_%s.tar.gz",
			osName, arch,
		)

		// Resolve current binary path
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to determine executable path: %w", err)
		}
		exe, err = filepath.EvalSymlinks(exe)
		if err != nil {
			return fmt.Errorf("failed to resolve executable path: %w", err)
		}

		fmt.Printf("Current binary: %s\n", exe)
		fmt.Printf("Downloading %s_%s from GitHub...\n", osName, arch)

		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to download release: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("download failed: HTTP %d from %s", resp.StatusCode, url)
		}

		// Extract the qq binary from the tarball
		binaryData, err := extractBinaryFromTarGz(resp.Body, "qq")
		if err != nil {
			return fmt.Errorf("failed to extract binary from archive: %w", err)
		}

		// Write the new binary by replacing the old one atomically:
		// write to a temp file in the same directory, then rename.
		dir := filepath.Dir(exe)
		tmp, err := os.CreateTemp(dir, "qq-update-*")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		tmpPath := tmp.Name()

		if _, err := tmp.Write(binaryData); err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("failed to write new binary: %w", err)
		}
		if err := tmp.Close(); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to close temp file: %w", err)
		}

		// Preserve the original file's permissions
		info, err := os.Stat(exe)
		if err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to stat current binary: %w", err)
		}
		if err := os.Chmod(tmpPath, info.Mode()); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to set permissions: %w", err)
		}

		// Atomic rename
		if err := os.Rename(tmpPath, exe); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to replace binary: %w (you may need to run with sudo)", err)
		}

		fmt.Println("Updated successfully.")
		return nil
	},
}

func releasePlatform() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "Darwin", nil
	case "linux":
		return "Linux", nil
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func releaseArch() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
}

func extractBinaryFromTarGz(r io.Reader, name string) ([]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Match the binary by base name (archive may contain paths like ./qq or qq)
		base := filepath.Base(hdr.Name)
		if base == name && hdr.Typeflag == tar.TypeReg {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("failed to read binary from archive: %w", err)
			}
			return data, nil
		}
	}

	// List what was in the archive for debugging
	return nil, fmt.Errorf("binary %q not found in archive", name)
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
