package start

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"databasus-agent/internal/config"
)

const (
	pgBasebackupVerifyTimeout = 10 * time.Second
	dbVerifyTimeout           = 10 * time.Second
)

func Run(cfg *config.Config, log *slog.Logger) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}

	if err := verifyPgBasebackup(cfg, log); err != nil {
		return err
	}

	if err := verifyDatabase(cfg, log); err != nil {
		return err
	}

	log.Info("start: stub — not yet implemented",
		"dbId", cfg.DbID,
		"hasToken", cfg.Token != "",
	)

	return nil
}

func validateConfig(cfg *config.Config) error {
	if cfg.DatabasusHost == "" {
		return errors.New("argument databasus-host is required")
	}

	if cfg.DbID == "" {
		return errors.New("argument db-id is required")
	}

	if cfg.Token == "" {
		return errors.New("argument token is required")
	}

	if cfg.PgHost == "" {
		return errors.New("argument pg-host is required")
	}

	if cfg.PgPort <= 0 {
		return errors.New("argument pg-port must be a positive number")
	}

	if cfg.PgUser == "" {
		return errors.New("argument pg-user is required")
	}

	if cfg.PgType != "host" && cfg.PgType != "docker" {
		return fmt.Errorf("argument pg-type must be 'host' or 'docker', got '%s'", cfg.PgType)
	}

	if cfg.WalDir == "" {
		return errors.New("argument wal-dir is required")
	}

	if cfg.PgType == "docker" && cfg.PgDockerContainerName == "" {
		return errors.New("argument pg-docker-container-name is required when pg-type is 'docker'")
	}

	return nil
}

func verifyPgBasebackup(cfg *config.Config, log *slog.Logger) error {
	switch cfg.PgType {
	case "host":
		return verifyPgBasebackupHost(cfg, log)
	case "docker":
		return verifyPgBasebackupDocker(cfg, log)
	default:
		return fmt.Errorf("unexpected pg-type: %s", cfg.PgType)
	}
}

func verifyPgBasebackupHost(cfg *config.Config, log *slog.Logger) error {
	binary := "pg_basebackup"
	if cfg.PgHostBinDir != "" {
		binary = filepath.Join(cfg.PgHostBinDir, "pg_basebackup")
	}

	ctx, cancel := context.WithTimeout(context.Background(), pgBasebackupVerifyTimeout)
	defer cancel()

	output, err := exec.CommandContext(ctx, binary, "--version").CombinedOutput()
	if err != nil {
		if cfg.PgHostBinDir != "" {
			return fmt.Errorf(
				"pg_basebackup not found at '%s': %w. Verify pg-host-bin-dir is correct",
				binary, err,
			)
		}

		return fmt.Errorf(
			"pg_basebackup not found in PATH: %w. Install PostgreSQL client tools or set pg-host-bin-dir",
			err,
		)
	}

	log.Info("pg_basebackup verified", "version", strings.TrimSpace(string(output)))

	return nil
}

func verifyPgBasebackupDocker(cfg *config.Config, log *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), pgBasebackupVerifyTimeout)
	defer cancel()

	output, err := exec.CommandContext(ctx,
		"docker", "exec", cfg.PgDockerContainerName,
		"pg_basebackup", "--version",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"pg_basebackup not available in container '%s': %w. "+
				"Check that the container is running and pg_basebackup is installed inside it",
			cfg.PgDockerContainerName, err,
		)
	}

	log.Info("pg_basebackup verified (docker)",
		"container", cfg.PgDockerContainerName,
		"version", strings.TrimSpace(string(output)),
	)

	return nil
}

func verifyDatabase(cfg *config.Config, log *slog.Logger) error {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable",
		cfg.PgHost, cfg.PgPort, cfg.PgUser, cfg.PgPassword,
	)

	ctx, cancel := context.WithTimeout(context.Background(), dbVerifyTimeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return fmt.Errorf(
			"failed to connect to PostgreSQL at %s:%d as user '%s': %w",
			cfg.PgHost, cfg.PgPort, cfg.PgUser, err,
		)
	}
	defer func() { _ = conn.Close(ctx) }()

	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("PostgreSQL ping failed at %s:%d: %w",
			cfg.PgHost, cfg.PgPort, err,
		)
	}

	log.Info("PostgreSQL connection verified",
		"host", cfg.PgHost,
		"port", cfg.PgPort,
		"user", cfg.PgUser,
	)

	return nil
}
