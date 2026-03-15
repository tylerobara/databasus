package wal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"

	"databasus-agent/internal/config"
)

const (
	pollInterval  = 2 * time.Second
	uploadTimeout = 5 * time.Minute
	uploadPath    = "/api/v1/backups/postgres/wal/upload"
)

var segmentNameRegex = regexp.MustCompile(`^[0-9A-Fa-f]{24}$`)

type Streamer struct {
	cfg        *config.Config
	httpClient *http.Client
	log        *slog.Logger
}

type uploadErrorResponse struct {
	Error               string `json:"error"`
	ExpectedSegmentName string `json:"expectedSegmentName"`
	ReceivedSegmentName string `json:"receivedSegmentName"`
}

func NewStreamer(cfg *config.Config, log *slog.Logger) *Streamer {
	return &Streamer{
		cfg:        cfg,
		httpClient: &http.Client{},
		log:        log,
	}
}

func (s *Streamer) Run(ctx context.Context) {
	s.log.Info("WAL streamer started", "walDir", s.cfg.WalDir)

	s.processQueue(ctx)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("WAL streamer stopping")
			return
		case <-ticker.C:
			s.processQueue(ctx)
		}
	}
}

func (s *Streamer) processQueue(ctx context.Context) {
	segments, err := s.listSegments()
	if err != nil {
		s.log.Error("Failed to list WAL segments", "error", err)
		return
	}

	for _, segmentName := range segments {
		if ctx.Err() != nil {
			return
		}

		if err := s.uploadSegment(ctx, segmentName); err != nil {
			s.log.Error("Failed to upload WAL segment",
				"segment", segmentName,
				"error", err,
			)
			return
		}
	}
}

func (s *Streamer) listSegments() ([]string, error) {
	entries, err := os.ReadDir(s.cfg.WalDir)
	if err != nil {
		return nil, fmt.Errorf("read wal dir: %w", err)
	}

	var segments []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if strings.HasSuffix(name, ".tmp") {
			continue
		}

		if !segmentNameRegex.MatchString(name) {
			continue
		}

		segments = append(segments, name)
	}

	sort.Strings(segments)

	return segments, nil
}

func (s *Streamer) uploadSegment(ctx context.Context, segmentName string) error {
	filePath := filepath.Join(s.cfg.WalDir, segmentName)

	pr, pw := io.Pipe()

	go s.compressAndStream(pw, filePath)

	uploadCtx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(uploadCtx, http.MethodPost, s.buildUploadURL(), pr)
	if err != nil {
		_ = pr.Close()
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", s.cfg.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Upload-Type", "wal")
	req.Header.Set("X-Wal-Segment-Name", segmentName)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusNoContent:
		s.log.Debug("WAL segment uploaded", "segment", segmentName)

		if *s.cfg.IsDeleteWalAfterUpload {
			if err := os.Remove(filePath); err != nil {
				s.log.Warn("Failed to delete uploaded WAL segment",
					"segment", segmentName,
					"error", err,
				)
			}
		}

		return nil

	case http.StatusConflict:
		var errResp uploadErrorResponse

		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			s.log.Warn("WAL chain gap detected",
				"segment", segmentName,
				"expected", errResp.ExpectedSegmentName,
				"received", errResp.ReceivedSegmentName,
			)
		} else {
			s.log.Warn("WAL chain gap detected", "segment", segmentName)
		}

		return fmt.Errorf("gap detected for segment %s", segmentName)

	default:
		body, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}
}

func (s *Streamer) compressAndStream(pw *io.PipeWriter, filePath string) {
	f, err := os.Open(filePath)
	if err != nil {
		_ = pw.CloseWithError(fmt.Errorf("open file: %w", err))
		return
	}
	defer func() { _ = f.Close() }()

	encoder, err := zstd.NewWriter(pw,
		zstd.WithEncoderLevel(zstd.SpeedDefault),
		zstd.WithEncoderCRC(true),
	)
	if err != nil {
		_ = pw.CloseWithError(fmt.Errorf("create zstd encoder: %w", err))
		return
	}

	if _, err := io.Copy(encoder, f); err != nil {
		_ = encoder.Close()
		_ = pw.CloseWithError(fmt.Errorf("compress: %w", err))
		return
	}

	if err := encoder.Close(); err != nil {
		_ = pw.CloseWithError(fmt.Errorf("close encoder: %w", err))
		return
	}

	_ = pw.Close()
}

func (s *Streamer) buildUploadURL() string {
	return s.cfg.DatabasusHost + uploadPath
}
