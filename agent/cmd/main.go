package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"databasus-agent/internal/config"
	"databasus-agent/internal/features/start"
	"databasus-agent/internal/features/upgrade"
	"databasus-agent/internal/logger"
)

var Version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "start":
		runStart(os.Args[2:])
	case "stop":
		runStop()
	case "status":
		runStatus()
	case "restore":
		runRestore(os.Args[2:])
	case "version":
		fmt.Println(Version)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)

	isDebug := fs.Bool("debug", false, "Enable debug logging")
	isSkipUpdate := fs.Bool("skip-update", false, "Skip auto-update check")

	cfg := &config.Config{}
	cfg.LoadFromJSONAndArgs(fs, args)

	if err := cfg.SaveToJSON(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
	}

	logger.Init(*isDebug)
	log := logger.GetLogger()

	isDev := checkIsDevelopment()
	runUpdateCheck(cfg.DatabasusHost, *isSkipUpdate, isDev, log)

	if err := start.Run(cfg, log); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runStop() {
	logger.Init(false)
	logger.GetLogger().Info("stop: stub — not yet implemented")
}

func runStatus() {
	logger.Init(false)
	logger.GetLogger().Info("status: stub — not yet implemented")
}

func runRestore(args []string) {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)

	targetDir := fs.String("target-dir", "", "Target pgdata directory")
	backupID := fs.String("backup-id", "", "Full backup UUID (optional)")
	targetTime := fs.String("target-time", "", "PITR target time in RFC3339 (optional)")
	isYes := fs.Bool("yes", false, "Skip confirmation prompt")
	isDebug := fs.Bool("debug", false, "Enable debug logging")
	isSkipUpdate := fs.Bool("skip-update", false, "Skip auto-update check")

	cfg := &config.Config{}
	cfg.LoadFromJSONAndArgs(fs, args)

	if err := cfg.SaveToJSON(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
	}

	logger.Init(*isDebug)
	log := logger.GetLogger()

	isDev := checkIsDevelopment()
	runUpdateCheck(cfg.DatabasusHost, *isSkipUpdate, isDev, log)

	log.Info("restore: stub — not yet implemented",
		"targetDir", *targetDir,
		"backupId", *backupID,
		"targetTime", *targetTime,
		"yes", *isYes,
	)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: databasus-agent <command> [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  start    Start the agent (WAL archiving + basebackups)")
	fmt.Fprintln(os.Stderr, "  stop     Stop a running agent")
	fmt.Fprintln(os.Stderr, "  status   Show agent status")
	fmt.Fprintln(os.Stderr, "  restore  Restore a database from backup")
	fmt.Fprintln(os.Stderr, "  version  Print agent version")
}

func runUpdateCheck(host string, isSkipUpdate, isDev bool, log interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
},
) {
	if isSkipUpdate {
		return
	}

	if host == "" {
		return
	}

	if err := upgrade.CheckAndUpdate(host, Version, isDev, log); err != nil {
		log.Error("Auto-update failed", "error", err)
		os.Exit(1)
	}
}

func checkIsDevelopment() bool {
	dir, err := os.Getwd()
	if err != nil {
		return false
	}

	for range 3 {
		if data, err := os.ReadFile(filepath.Join(dir, ".env")); err == nil {
			return parseEnvMode(data)
		}

		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return false
		}

		dir = filepath.Dir(dir)
	}

	return false
}

func parseEnvMode(data []byte) bool {
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == "ENV_MODE" {
			return strings.TrimSpace(parts[1]) == "development"
		}
	}

	return false
}
