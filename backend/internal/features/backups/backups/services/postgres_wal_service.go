package backups_services

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"

	backups_core "databasus-backend/internal/features/backups/backups/core"
	backups_dto "databasus-backend/internal/features/backups/backups/dto"
	backup_encryption "databasus-backend/internal/features/backups/backups/encryption"
	backups_config "databasus-backend/internal/features/backups/config"
	"databasus-backend/internal/features/databases"
	"databasus-backend/internal/features/databases/databases/postgresql"
	encryption_secrets "databasus-backend/internal/features/encryption/secrets"
	util_encryption "databasus-backend/internal/util/encryption"
	util_wal "databasus-backend/internal/util/wal"
)

// PostgreWalBackupService handles WAL segment and basebackup uploads from the databasus-cli agent.
type PostgreWalBackupService struct {
	backupConfigService *backups_config.BackupConfigService
	backupRepository    *backups_core.BackupRepository
	fieldEncryptor      util_encryption.FieldEncryptor
	secretKeyService    *encryption_secrets.SecretKeyService
	logger              *slog.Logger
	backupService       *BackupService
}

// UploadWal accepts a streaming WAL segment or basebackup upload from the agent.
// For WAL segments it validates the WAL chain before accepting. Returns an UploadGapResponse
// (409) when the chain is broken so the agent knows to trigger a full basebackup.
func (s *PostgreWalBackupService) UploadWal(
	ctx context.Context,
	database *databases.Database,
	uploadType backups_core.PgWalUploadType,
	walSegmentName string,
	fullBackupWalStartSegment string,
	fullBackupWalStopSegment string,
	walSegmentSizeBytes int64,
	body io.Reader,
) (*backups_dto.UploadGapResponse, error) {
	if err := s.validateWalBackupType(database); err != nil {
		return nil, err
	}

	if uploadType == backups_core.PgWalUploadTypeBasebackup {
		if fullBackupWalStartSegment == "" || fullBackupWalStopSegment == "" {
			return nil, fmt.Errorf(
				"fullBackupWalStartSegment and fullBackupWalStopSegment are required for basebackup uploads",
			)
		}
	}

	backupConfig, err := s.backupConfigService.GetBackupConfigByDbId(database.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup config: %w", err)
	}

	if backupConfig.Storage == nil {
		return nil, fmt.Errorf("no storage configured for database %s", database.ID)
	}

	if uploadType == backups_core.PgWalUploadTypeWal {
		// Idempotency: check before chain validation so a successful re-upload is
		// not misidentified as a gap.
		existing, err := s.backupRepository.FindWalSegmentByName(database.ID, walSegmentName)
		if err != nil {
			return nil, fmt.Errorf("failed to check for duplicate WAL segment: %w", err)
		}

		if existing != nil {
			return nil, nil
		}

		gapResp, err := s.validateWalChain(database.ID, walSegmentName, walSegmentSizeBytes)
		if err != nil {
			return nil, err
		}

		if gapResp != nil {
			return gapResp, nil
		}
	}

	backup := s.createBackupRecord(
		database.ID,
		backupConfig.Storage.ID,
		uploadType,
		database.Name,
		walSegmentName,
		fullBackupWalStartSegment,
		fullBackupWalStopSegment,
		backupConfig.Encryption,
	)

	if err := s.backupRepository.Save(backup); err != nil {
		return nil, fmt.Errorf("failed to create backup record: %w", err)
	}

	sizeBytes, streamErr := s.streamToStorage(ctx, backup, backupConfig, body)
	if streamErr != nil {
		errMsg := streamErr.Error()
		s.markFailed(backup, errMsg)

		return nil, fmt.Errorf("upload failed: %w", streamErr)
	}

	s.markCompleted(backup, sizeBytes)

	return nil, nil
}

func (s *PostgreWalBackupService) GetRestorePlan(
	database *databases.Database,
	backupID *uuid.UUID,
) (*backups_dto.GetRestorePlanResponse, *backups_dto.GetRestorePlanErrorResponse, error) {
	if err := s.validateWalBackupType(database); err != nil {
		return nil, nil, err
	}

	fullBackup, err := s.resolveFullBackup(database.ID, backupID)
	if err != nil {
		return nil, nil, err
	}

	if fullBackup == nil {
		msg := "no full backups available for this database"
		if backupID != nil {
			msg = fmt.Sprintf("full backup %s not found or not completed", backupID)
		}

		return nil, &backups_dto.GetRestorePlanErrorResponse{
			Error:   "no_backups",
			Message: msg,
		}, nil
	}

	startSegment := ""
	if fullBackup.PgFullBackupWalStartSegmentName != nil {
		startSegment = *fullBackup.PgFullBackupWalStartSegmentName
	}

	walSegments, err := s.backupRepository.FindCompletedWalSegmentsAfter(database.ID, startSegment)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query WAL segments: %w", err)
	}

	chainErr := s.validateRestoreWalChain(fullBackup, walSegments)
	if chainErr != nil {
		return nil, chainErr, nil
	}

	fullBackupSizeBytes := int64(fullBackup.BackupSizeMb * 1024 * 1024)

	pgVersion := ""
	if fullBackup.PgVersion != nil {
		pgVersion = *fullBackup.PgVersion
	}

	stopSegment := ""
	if fullBackup.PgFullBackupWalStopSegmentName != nil {
		stopSegment = *fullBackup.PgFullBackupWalStopSegmentName
	}

	response := &backups_dto.GetRestorePlanResponse{
		FullBackup: backups_dto.RestorePlanFullBackup{
			BackupID:                  fullBackup.ID,
			FullBackupWalStartSegment: startSegment,
			FullBackupWalStopSegment:  stopSegment,
			PgVersion:                 pgVersion,
			CreatedAt:                 fullBackup.CreatedAt,
			SizeBytes:                 fullBackupSizeBytes,
		},
		TotalSizeBytes: fullBackupSizeBytes,
	}

	for _, seg := range walSegments {
		segName := ""
		if seg.PgWalSegmentName != nil {
			segName = *seg.PgWalSegmentName
		}

		segSizeBytes := int64(seg.BackupSizeMb * 1024 * 1024)

		response.WalSegments = append(response.WalSegments, backups_dto.RestorePlanWalSegment{
			BackupID:    seg.ID,
			SegmentName: segName,
			SizeBytes:   segSizeBytes,
		})

		response.TotalSizeBytes += segSizeBytes
		response.LatestAvailableSegment = segName
	}

	return response, nil, nil
}

// DownloadBackupFile returns a reader for a backup file belonging to the given database.
// Decryption is handled transparently if the backup is encrypted.
func (s *PostgreWalBackupService) DownloadBackupFile(
	database *databases.Database,
	backupID uuid.UUID,
) (io.ReadCloser, error) {
	if err := s.validateWalBackupType(database); err != nil {
		return nil, err
	}

	backup, err := s.backupRepository.FindByID(backupID)
	if err != nil {
		return nil, fmt.Errorf("backup not found: %w", err)
	}

	if backup.DatabaseID != database.ID {
		return nil, fmt.Errorf("backup does not belong to this database")
	}

	if backup.Status != backups_core.BackupStatusCompleted {
		return nil, fmt.Errorf("backup is not completed")
	}

	return s.backupService.GetBackupReader(backupID)
}

func (s *PostgreWalBackupService) GetNextFullBackupTime(
	database *databases.Database,
) (*backups_dto.GetNextFullBackupTimeResponse, error) {
	if err := s.validateWalBackupType(database); err != nil {
		return nil, err
	}

	backupConfig, err := s.backupConfigService.GetBackupConfigByDbId(database.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup config: %w", err)
	}

	if backupConfig.BackupInterval == nil {
		return nil, fmt.Errorf("no backup interval configured for database %s", database.ID)
	}

	lastFullBackup, err := s.backupRepository.FindLastCompletedFullWalBackupByDatabaseID(
		database.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query last full backup: %w", err)
	}

	var lastBackupTime *time.Time
	if lastFullBackup != nil {
		lastBackupTime = &lastFullBackup.CreatedAt
	}

	now := time.Now().UTC()
	nextTime := backupConfig.BackupInterval.NextTriggerTime(now, lastBackupTime)

	return &backups_dto.GetNextFullBackupTimeResponse{
		NextFullBackupTime: nextTime,
	}, nil
}

// ReportError creates a FAILED backup record with the agent's error message.
func (s *PostgreWalBackupService) ReportError(
	database *databases.Database,
	errorMsg string,
) error {
	if err := s.validateWalBackupType(database); err != nil {
		return err
	}

	backupConfig, err := s.backupConfigService.GetBackupConfigByDbId(database.ID)
	if err != nil {
		return fmt.Errorf("failed to get backup config: %w", err)
	}

	if backupConfig.Storage == nil {
		return fmt.Errorf("no storage configured for database %s", database.ID)
	}

	now := time.Now().UTC()
	backup := &backups_core.Backup{
		ID:          uuid.New(),
		DatabaseID:  database.ID,
		StorageID:   backupConfig.Storage.ID,
		Status:      backups_core.BackupStatusFailed,
		FailMessage: &errorMsg,
		Encryption:  backupConfig.Encryption,
		CreatedAt:   now,
	}

	backup.GenerateFilename(database.Name)

	if err := s.backupRepository.Save(backup); err != nil {
		return fmt.Errorf("failed to save error backup record: %w", err)
	}

	return nil
}

func (s *PostgreWalBackupService) validateWalChain(
	databaseID uuid.UUID,
	incomingSegment string,
	walSegmentSizeBytes int64,
) (*backups_dto.UploadGapResponse, error) {
	fullBackup, err := s.backupRepository.FindLastCompletedFullWalBackupByDatabaseID(databaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to query full backup: %w", err)
	}

	// No full backup exists yet: cannot accept WAL segments without a chain anchor.
	if fullBackup == nil || fullBackup.PgFullBackupWalStopSegmentName == nil {
		return &backups_dto.UploadGapResponse{
			Error:               "no_full_backup",
			ExpectedSegmentName: "",
			ReceivedSegmentName: incomingSegment,
		}, nil
	}

	stopSegment := *fullBackup.PgFullBackupWalStopSegmentName

	lastWal, err := s.backupRepository.FindLastWalSegmentAfter(databaseID, stopSegment)
	if err != nil {
		return nil, fmt.Errorf("failed to query last WAL segment: %w", err)
	}

	walCalculator := util_wal.NewWalCalculator(walSegmentSizeBytes)

	var chainTail string
	if lastWal != nil && lastWal.PgWalSegmentName != nil {
		chainTail = *lastWal.PgWalSegmentName
	} else {
		chainTail = stopSegment
	}

	expectedNext, err := walCalculator.NextSegment(chainTail)
	if err != nil {
		return nil, fmt.Errorf("WAL arithmetic failed for %q: %w", chainTail, err)
	}

	if incomingSegment != expectedNext {
		return &backups_dto.UploadGapResponse{
			Error:               "gap_detected",
			ExpectedSegmentName: expectedNext,
			ReceivedSegmentName: incomingSegment,
		}, nil
	}

	return nil, nil
}

func (s *PostgreWalBackupService) createBackupRecord(
	databaseID uuid.UUID,
	storageID uuid.UUID,
	uploadType backups_core.PgWalUploadType,
	dbName string,
	walSegmentName string,
	fullBackupWalStartSegment string,
	fullBackupWalStopSegment string,
	encryption backups_config.BackupEncryption,
) *backups_core.Backup {
	now := time.Now().UTC()

	backup := &backups_core.Backup{
		ID:         uuid.New(),
		DatabaseID: databaseID,
		StorageID:  storageID,
		Status:     backups_core.BackupStatusInProgress,
		Encryption: encryption,
		CreatedAt:  now,
	}

	backup.GenerateFilename(dbName)

	if uploadType == backups_core.PgWalUploadTypeBasebackup {
		walBackupType := backups_core.PgWalBackupTypeFullBackup
		backup.PgWalBackupType = &walBackupType

		if fullBackupWalStartSegment != "" {
			backup.PgFullBackupWalStartSegmentName = &fullBackupWalStartSegment
		}

		if fullBackupWalStopSegment != "" {
			backup.PgFullBackupWalStopSegmentName = &fullBackupWalStopSegment
		}
	} else {
		walBackupType := backups_core.PgWalBackupTypeWalSegment
		backup.PgWalBackupType = &walBackupType
		backup.PgWalSegmentName = &walSegmentName
	}

	return backup
}

func (s *PostgreWalBackupService) streamToStorage(
	ctx context.Context,
	backup *backups_core.Backup,
	backupConfig *backups_config.BackupConfig,
	body io.Reader,
) (int64, error) {
	if backupConfig.Encryption == backups_config.BackupEncryptionEncrypted {
		return s.streamEncrypted(ctx, backup, backupConfig, body, backup.FileName)
	}

	return s.streamDirect(ctx, backupConfig, body, backup.FileName)
}

func (s *PostgreWalBackupService) streamDirect(
	ctx context.Context,
	backupConfig *backups_config.BackupConfig,
	body io.Reader,
	fileName string,
) (int64, error) {
	cr := &countingReader{r: body}

	if err := backupConfig.Storage.SaveFile(ctx, s.fieldEncryptor, s.logger, fileName, cr); err != nil {
		return 0, err
	}

	return cr.n, nil
}

func (s *PostgreWalBackupService) streamEncrypted(
	ctx context.Context,
	backup *backups_core.Backup,
	backupConfig *backups_config.BackupConfig,
	body io.Reader,
	fileName string,
) (int64, error) {
	masterKey, err := s.secretKeyService.GetSecretKey()
	if err != nil {
		return 0, fmt.Errorf("failed to get master encryption key: %w", err)
	}

	pipeReader, pipeWriter := io.Pipe()

	encryptionSetup, err := backup_encryption.SetupEncryptionWriter(
		pipeWriter,
		masterKey,
		backup.ID,
	)
	if err != nil {
		_ = pipeWriter.Close()
		return 0, err
	}

	copyErrCh := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(encryptionSetup.Writer, body)
		if copyErr != nil {
			_ = encryptionSetup.Writer.Close()
			_ = pipeWriter.CloseWithError(copyErr)
			copyErrCh <- copyErr
			return
		}

		if closeErr := encryptionSetup.Writer.Close(); closeErr != nil {
			_ = pipeWriter.CloseWithError(closeErr)
			copyErrCh <- closeErr
			return
		}

		copyErrCh <- pipeWriter.Close()
	}()

	cr := &countingReader{r: pipeReader}
	saveErr := backupConfig.Storage.SaveFile(ctx, s.fieldEncryptor, s.logger, fileName, cr)
	copyErr := <-copyErrCh

	if copyErr != nil {
		return 0, copyErr
	}

	if saveErr != nil {
		return 0, saveErr
	}

	backup.EncryptionSalt = &encryptionSetup.SaltBase64
	backup.EncryptionIV = &encryptionSetup.NonceBase64

	return cr.n, nil
}

func (s *PostgreWalBackupService) markCompleted(backup *backups_core.Backup, sizeBytes int64) {
	backup.Status = backups_core.BackupStatusCompleted
	backup.BackupSizeMb = float64(sizeBytes) / (1024 * 1024)

	if err := s.backupRepository.Save(backup); err != nil {
		s.logger.Error(
			"failed to mark WAL backup as completed",
			"backupId",
			backup.ID,
			"error",
			err,
		)
	}
}

func (s *PostgreWalBackupService) markFailed(backup *backups_core.Backup, errMsg string) {
	backup.Status = backups_core.BackupStatusFailed
	backup.FailMessage = &errMsg

	if err := s.backupRepository.Save(backup); err != nil {
		s.logger.Error("failed to mark WAL backup as failed", "backupId", backup.ID, "error", err)
	}
}

func (s *PostgreWalBackupService) resolveFullBackup(
	databaseID uuid.UUID,
	backupID *uuid.UUID,
) (*backups_core.Backup, error) {
	if backupID != nil {
		return s.backupRepository.FindCompletedFullWalBackupByID(databaseID, *backupID)
	}

	return s.backupRepository.FindLastCompletedFullWalBackupByDatabaseID(databaseID)
}

func (s *PostgreWalBackupService) validateRestoreWalChain(
	fullBackup *backups_core.Backup,
	walSegments []*backups_core.Backup,
) *backups_dto.GetRestorePlanErrorResponse {
	if len(walSegments) == 0 {
		return nil
	}

	stopSegment := ""
	if fullBackup.PgFullBackupWalStopSegmentName != nil {
		stopSegment = *fullBackup.PgFullBackupWalStopSegmentName
	}

	walCalculator := util_wal.NewWalCalculator(0)
	expectedNext, err := walCalculator.NextSegment(stopSegment)
	if err != nil {
		return nil
	}

	for _, seg := range walSegments {
		segName := ""
		if seg.PgWalSegmentName != nil {
			segName = *seg.PgWalSegmentName
		}

		cmp, cmpErr := walCalculator.Compare(segName, stopSegment)
		if cmpErr != nil {
			return nil
		}

		// Skip segments that are <= stopSegment (they are part of the basebackup range)
		if cmp <= 0 {
			continue
		}

		if segName != expectedNext {
			lastContiguous := stopSegment
			// Walk back to find the segment before the gap
			for _, prev := range walSegments {
				prevName := ""
				if prev.PgWalSegmentName != nil {
					prevName = *prev.PgWalSegmentName
				}

				prevCmp, _ := walCalculator.Compare(prevName, stopSegment)
				if prevCmp <= 0 {
					continue
				}

				if prevName == segName {
					break
				}

				lastContiguous = prevName
			}

			return &backups_dto.GetRestorePlanErrorResponse{
				Error: "wal_chain_broken",
				Message: fmt.Sprintf(
					"WAL chain has a gap after segment %s. Recovery is only possible up to this segment.",
					lastContiguous,
				),
				LastContiguousSegment: lastContiguous,
			}
		}

		expectedNext, err = walCalculator.NextSegment(segName)
		if err != nil {
			return nil
		}
	}

	return nil
}

func (s *PostgreWalBackupService) validateWalBackupType(database *databases.Database) error {
	if database.Postgresql == nil ||
		database.Postgresql.BackupType != postgresql.PostgresBackupTypeWalV1 {
		return fmt.Errorf("database %s is not configured for WAL backups", database.ID)
	}
	return nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (cr *countingReader) Read(p []byte) (n int, err error) {
	n, err = cr.r.Read(p)
	cr.n += int64(n)

	return n, err
}
