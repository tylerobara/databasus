package backups_dto

import (
	"io"
	"time"

	"github.com/google/uuid"

	backups_core "databasus-backend/internal/features/backups/backups/core"
	"databasus-backend/internal/features/backups/backups/encryption"
)

type GetBackupsRequest struct {
	DatabaseID string `form:"database_id" binding:"required"`
	Limit      int    `form:"limit"`
	Offset     int    `form:"offset"`
}

type GetBackupsResponse struct {
	Backups []*backups_core.Backup `json:"backups"`
	Total   int64                  `json:"total"`
	Limit   int                    `json:"limit"`
	Offset  int                    `json:"offset"`
}

type DecryptionReaderCloser struct {
	*encryption.DecryptionReader
	BaseReader io.ReadCloser
}

func (r *DecryptionReaderCloser) Close() error {
	return r.BaseReader.Close()
}

type MakeBackupRequest struct {
	DatabaseID uuid.UUID `json:"database_id" binding:"required"`
}

type GetNextFullBackupTimeResponse struct {
	NextFullBackupTime *time.Time `json:"nextFullBackupTime"`
}

type ReportErrorRequest struct {
	Error string `json:"error" binding:"required"`
}

type UploadGapResponse struct {
	Error               string `json:"error"`
	ExpectedSegmentName string `json:"expectedSegmentName"`
	ReceivedSegmentName string `json:"receivedSegmentName"`
}

type RestorePlanFullBackup struct {
	BackupID                  uuid.UUID `json:"id"`
	FullBackupWalStartSegment string    `json:"fullBackupWalStartSegment"`
	FullBackupWalStopSegment  string    `json:"fullBackupWalStopSegment"`
	PgVersion                 string    `json:"pgVersion"`
	CreatedAt                 time.Time `json:"createdAt"`
	SizeBytes                 int64     `json:"sizeBytes"`
}

type RestorePlanWalSegment struct {
	BackupID    uuid.UUID `json:"backupId"`
	SegmentName string    `json:"segmentName"`
	SizeBytes   int64     `json:"sizeBytes"`
}

type GetRestorePlanErrorResponse struct {
	Error                 string `json:"error"`
	Message               string `json:"message"`
	LastContiguousSegment string `json:"lastContiguousSegment,omitempty"`
}

type GetRestorePlanResponse struct {
	FullBackup             RestorePlanFullBackup   `json:"fullBackup"`
	WalSegments            []RestorePlanWalSegment `json:"walSegments"`
	TotalSizeBytes         int64                   `json:"totalSizeBytes"`
	LatestAvailableSegment string                  `json:"latestAvailableSegment"`
}
