package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type versionResponse struct {
	Version string `json:"version"`
}

func CheckAndUpdate(databasusHost, currentVersion string, isDev bool, log Logger) error {
	if isDev {
		log.Info("Skipping update check (development mode)")
		return nil
	}

	serverVersion, err := fetchServerVersion(databasusHost, log)
	if err != nil {
		return nil
	}

	if serverVersion == currentVersion {
		log.Info("Agent version is up to date", "version", currentVersion)
		return nil
	}

	log.Info("Updating agent...", "current", currentVersion, "target", serverVersion)

	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}

	tempPath := selfPath + ".update"

	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := downloadBinary(databasusHost, tempPath); err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	if err := os.Chmod(tempPath, 0o755); err != nil {
		return fmt.Errorf("failed to set permissions on update: %w", err)
	}

	if err := verifyBinary(tempPath, serverVersion); err != nil {
		return fmt.Errorf("update verification failed: %w", err)
	}

	if err := os.Rename(tempPath, selfPath); err != nil {
		return fmt.Errorf("failed to replace binary (try --skip-update if this persists): %w", err)
	}

	log.Info("Update complete, re-executing...")

	return syscall.Exec(selfPath, os.Args, os.Environ())
}

func fetchServerVersion(host string, log Logger) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, host+"/api/v1/system/version", nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Warn("Could not reach server for update check, continuing", "error", err)
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		log.Warn(
			"Server returned non-OK status for version check, continuing",
			"status",
			resp.StatusCode,
		)
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var ver versionResponse
	if err := json.NewDecoder(resp.Body).Decode(&ver); err != nil {
		log.Warn("Failed to parse server version response, continuing", "error", err)
		return "", err
	}

	return ver.Version, nil
}

func downloadBinary(host, destPath string) error {
	url := fmt.Sprintf("%s/api/v1/system/agent?arch=%s", host, runtime.GOARCH)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d for agent download", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = io.Copy(f, resp.Body)

	return err
}

func verifyBinary(binaryPath, expectedVersion string) error {
	cmd := exec.CommandContext(context.Background(), binaryPath, "version")

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("binary failed to execute: %w", err)
	}

	got := strings.TrimSpace(string(output))
	if got != expectedVersion {
		return fmt.Errorf("version mismatch: expected %q, got %q", expectedVersion, got)
	}

	return nil
}
