package backups_controllers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	backups_core "databasus-backend/internal/features/backups/backups/core"
	backups_dto "databasus-backend/internal/features/backups/backups/dto"
	backups_services "databasus-backend/internal/features/backups/backups/services"
	"databasus-backend/internal/features/databases"
)

// PostgreWalBackupController handles WAL backup endpoints used by the databasus-cli agent.
// Authentication is via a plain agent token in the Authorization header (no Bearer prefix).
type PostgreWalBackupController struct {
	databaseService *databases.DatabaseService
	walService      *backups_services.PostgreWalBackupService
}

func (c *PostgreWalBackupController) RegisterRoutes(router *gin.RouterGroup) {
	walRoutes := router.Group("/backups/postgres/wal")

	walRoutes.GET("/next-full-backup-time", c.GetNextFullBackupTime)
	walRoutes.GET("/is-wal-chain-valid-since-last-full-backup", c.IsWalChainValidSinceLastBackup)
	walRoutes.POST("/error", c.ReportError)
	walRoutes.POST("/upload", c.Upload)
	walRoutes.GET("/restore/plan", c.GetRestorePlan)
	walRoutes.GET("/restore/download", c.DownloadBackupFile)
}

// GetNextFullBackupTime
// @Summary Get next full backup time
// @Description Returns the next scheduled full basebackup time for the authenticated database
// @Tags backups-wal
// @Produce json
// @Security AgentToken
// @Success 200 {object} backups_dto.GetNextFullBackupTimeResponse
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /backups/postgres/wal/next-full-backup-time [get]
func (c *PostgreWalBackupController) GetNextFullBackupTime(ctx *gin.Context) {
	database, err := c.getDatabase(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	response, err := c.walService.GetNextFullBackupTime(database)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

// ReportError
// @Summary Report agent error
// @Description Records a fatal error from the agent against the database record and marks it as errored
// @Tags backups-wal
// @Accept json
// @Security AgentToken
// @Param request body backups_dto.ReportErrorRequest true "Error details"
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /backups/postgres/wal/error [post]
func (c *PostgreWalBackupController) ReportError(ctx *gin.Context) {
	database, err := c.getDatabase(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	var request backups_dto.ReportErrorRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.walService.ReportError(database, request.Error); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.Status(http.StatusOK)
}

// IsWalChainValidSinceLastBackup
// @Summary Check WAL chain validity since last full backup
// @Description Checks whether the WAL chain is continuous since the last completed full backup.
// Returns isValid=true if the chain is intact, or isValid=false with error details if not.
// @Tags backups-wal
// @Produce json
// @Security AgentToken
// @Success 200 {object} backups_dto.IsWalChainValidResponse
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /backups/postgres/wal/is-wal-chain-valid-since-last-full-backup [get]
func (c *PostgreWalBackupController) IsWalChainValidSinceLastBackup(ctx *gin.Context) {
	database, err := c.getDatabase(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	response, err := c.walService.IsWalChainValid(database)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

// Upload
// @Summary Stream upload a basebackup or WAL segment
// @Description Accepts a zstd-compressed binary stream and stores it in the database's configured storage.
// The server generates the storage filename; agents do not control the destination path.
// WAL segments are accepted unconditionally
// @Tags backups-wal
// @Accept application/octet-stream
// @Produce json
// @Security AgentToken
// @Param X-Upload-Type header string true "Upload type" Enums(basebackup, wal)
// @Param X-Wal-Segment-Name header string false "24-hex WAL segment identifier (required for wal uploads, e.g. 0000000100000001000000AB)"
// @Param fullBackupWalStartSegment query string false "First WAL segment needed to make the basebackup consistent (required for basebackup uploads)"
// @Param fullBackupWalStopSegment query string false "Last WAL segment included in the basebackup (required for basebackup uploads)"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /backups/postgres/wal/upload [post]
func (c *PostgreWalBackupController) Upload(ctx *gin.Context) {
	database, err := c.getDatabase(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	uploadType := backups_core.PgWalUploadType(ctx.GetHeader("X-Upload-Type"))
	if uploadType != backups_core.PgWalUploadTypeBasebackup &&
		uploadType != backups_core.PgWalUploadTypeWal {
		ctx.JSON(
			http.StatusBadRequest,
			gin.H{"error": "X-Upload-Type must be 'basebackup' or 'wal'"},
		)
		return
	}

	walSegmentName := ""
	if uploadType == backups_core.PgWalUploadTypeWal {
		walSegmentName = ctx.GetHeader("X-Wal-Segment-Name")
		if walSegmentName == "" {
			ctx.JSON(
				http.StatusBadRequest,
				gin.H{"error": "X-Wal-Segment-Name is required for wal uploads"},
			)
			return
		}
	}

	if uploadType == backups_core.PgWalUploadTypeBasebackup {
		if ctx.Query("fullBackupWalStartSegment") == "" ||
			ctx.Query("fullBackupWalStopSegment") == "" {
			ctx.JSON(
				http.StatusBadRequest,
				gin.H{
					"error": "fullBackupWalStartSegment and fullBackupWalStopSegment are required for basebackup uploads",
				},
			)
			return
		}
	}

	uploadErr := c.walService.UploadWal(
		ctx.Request.Context(),
		database,
		uploadType,
		walSegmentName,
		ctx.Query("fullBackupWalStartSegment"),
		ctx.Query("fullBackupWalStopSegment"),
		ctx.Request.Body,
	)

	if uploadErr != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": uploadErr.Error()})
		return
	}

	ctx.Status(http.StatusNoContent)
}

// GetRestorePlan
// @Summary Get restore plan
// @Description Resolves the full backup and all required WAL segments needed for recovery. Validates the WAL chain is continuous.
// @Tags backups-wal
// @Produce json
// @Security AgentToken
// @Param backupId query string false "UUID of a specific full backup to restore from; defaults to the most recent"
// @Success 200 {object} backups_dto.GetRestorePlanResponse
// @Failure 400 {object} map[string]string "Broken WAL chain or no backups available"
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /backups/postgres/wal/restore/plan [get]
func (c *PostgreWalBackupController) GetRestorePlan(ctx *gin.Context) {
	database, err := c.getDatabase(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	var backupID *uuid.UUID
	if raw := ctx.Query("backupId"); raw != "" {
		parsed, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid backupId format"})
			return
		}

		backupID = &parsed
	}

	response, planErr, err := c.walService.GetRestorePlan(database, backupID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if planErr != nil {
		ctx.JSON(http.StatusBadRequest, planErr)
		return
	}

	ctx.JSON(http.StatusOK, response)
}

// DownloadBackupFile
// @Summary Download a backup or WAL segment file for restore
// @Description Retrieves the backup file by ID (validated against the authenticated database), decrypts it server-side if encrypted, and streams the zstd-compressed result to the agent
// @Tags backups-wal
// @Produce application/octet-stream
// @Security AgentToken
// @Param backupId query string true "Backup ID from the restore plan response"
// @Success 200 {file} file
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /backups/postgres/wal/restore/download [get]
func (c *PostgreWalBackupController) DownloadBackupFile(ctx *gin.Context) {
	database, err := c.getDatabase(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	backupIDRaw := ctx.Query("backupId")
	if backupIDRaw == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "backupId is required"})
		return
	}

	backupID, err := uuid.Parse(backupIDRaw)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid backupId format"})
		return
	}

	reader, err := c.walService.DownloadBackupFile(database, backupID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer func() { _ = reader.Close() }()

	ctx.Header("Content-Type", "application/octet-stream")
	ctx.Status(http.StatusOK)

	_, _ = io.Copy(ctx.Writer, reader)
}

func (c *PostgreWalBackupController) getDatabase(
	ctx *gin.Context,
) (*databases.Database, error) {
	token := ctx.GetHeader("Authorization")
	return c.databaseService.GetDatabaseByAgentToken(token)
}
