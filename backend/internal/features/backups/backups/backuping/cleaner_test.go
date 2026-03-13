package backuping

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	backups_core "databasus-backend/internal/features/backups/backups/core"
	backups_config "databasus-backend/internal/features/backups/config"
	"databasus-backend/internal/features/databases"
	"databasus-backend/internal/features/intervals"
	"databasus-backend/internal/features/notifiers"
	"databasus-backend/internal/features/storages"
	users_enums "databasus-backend/internal/features/users/enums"
	users_testing "databasus-backend/internal/features/users/testing"
	workspaces_testing "databasus-backend/internal/features/workspaces/testing"
	"databasus-backend/internal/storage"
	"databasus-backend/internal/util/period"
)

func Test_CleanOldBackups_DeletesBackupsOlderThanRetentionTimePeriod(t *testing.T) {
	router := CreateTestRouter()
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := storages.CreateTestStorage(workspace.ID)
	notifier := notifiers.CreateTestNotifier(workspace.ID)
	database := databases.CreateTestDatabase(workspace.ID, storage, notifier)

	defer func() {
		backups, _ := backupRepository.FindByDatabaseID(database.ID)
		for _, backup := range backups {
			backupRepository.DeleteByID(backup.ID)
		}

		databases.RemoveTestDatabase(database)
		time.Sleep(50 * time.Millisecond)
		notifiers.RemoveTestNotifier(notifier)
		storages.RemoveTestStorage(storage.ID)
		workspaces_testing.RemoveTestWorkspace(workspace, router)
	}()

	interval := createTestInterval()

	backupConfig := &backups_config.BackupConfig{
		DatabaseID:          database.ID,
		IsBackupsEnabled:    true,
		RetentionPolicyType: backups_config.RetentionPolicyTypeTimePeriod,
		RetentionTimePeriod: period.PeriodWeek,
		StorageID:           &storage.ID,
		BackupIntervalID:    interval.ID,
		BackupInterval:      interval,
	}
	_, err := backups_config.GetBackupConfigService().SaveBackupConfig(backupConfig)
	assert.NoError(t, err)

	now := time.Now().UTC()
	oldBackup1 := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 10,
		CreatedAt:    now.Add(-10 * 24 * time.Hour),
	}
	oldBackup2 := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 10,
		CreatedAt:    now.Add(-8 * 24 * time.Hour),
	}
	recentBackup := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 10,
		CreatedAt:    now.Add(-3 * 24 * time.Hour),
	}

	err = backupRepository.Save(oldBackup1)
	assert.NoError(t, err)
	err = backupRepository.Save(oldBackup2)
	assert.NoError(t, err)
	err = backupRepository.Save(recentBackup)
	assert.NoError(t, err)

	cleaner := GetBackupCleaner()
	err = cleaner.cleanByRetentionPolicy()
	assert.NoError(t, err)

	remainingBackups, err := backupRepository.FindByDatabaseID(database.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(remainingBackups))
	assert.Equal(t, recentBackup.ID, remainingBackups[0].ID)
}

func Test_CleanOldBackups_SkipsDatabaseWithForeverRetentionPeriod(t *testing.T) {
	router := CreateTestRouter()
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := storages.CreateTestStorage(workspace.ID)
	notifier := notifiers.CreateTestNotifier(workspace.ID)
	database := databases.CreateTestDatabase(workspace.ID, storage, notifier)

	defer func() {
		backups, _ := backupRepository.FindByDatabaseID(database.ID)
		for _, backup := range backups {
			backupRepository.DeleteByID(backup.ID)
		}

		databases.RemoveTestDatabase(database)
		time.Sleep(50 * time.Millisecond)
		notifiers.RemoveTestNotifier(notifier)
		storages.RemoveTestStorage(storage.ID)
		workspaces_testing.RemoveTestWorkspace(workspace, router)
	}()

	interval := createTestInterval()

	backupConfig := &backups_config.BackupConfig{
		DatabaseID:          database.ID,
		IsBackupsEnabled:    true,
		RetentionPolicyType: backups_config.RetentionPolicyTypeTimePeriod,
		RetentionTimePeriod: period.PeriodForever,
		StorageID:           &storage.ID,
		BackupIntervalID:    interval.ID,
		BackupInterval:      interval,
	}
	_, err := backups_config.GetBackupConfigService().SaveBackupConfig(backupConfig)
	assert.NoError(t, err)

	oldBackup := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 10,
		CreatedAt:    time.Now().UTC().Add(-365 * 24 * time.Hour),
	}
	err = backupRepository.Save(oldBackup)
	assert.NoError(t, err)

	cleaner := GetBackupCleaner()
	err = cleaner.cleanByRetentionPolicy()
	assert.NoError(t, err)

	remainingBackups, err := backupRepository.FindByDatabaseID(database.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(remainingBackups))
	assert.Equal(t, oldBackup.ID, remainingBackups[0].ID)
}

func Test_CleanExceededBackups_WhenUnderLimit_NoBackupsDeleted(t *testing.T) {
	router := CreateTestRouter()
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := storages.CreateTestStorage(workspace.ID)
	notifier := notifiers.CreateTestNotifier(workspace.ID)
	database := databases.CreateTestDatabase(workspace.ID, storage, notifier)

	defer func() {
		backups, _ := backupRepository.FindByDatabaseID(database.ID)
		for _, backup := range backups {
			backupRepository.DeleteByID(backup.ID)
		}

		databases.RemoveTestDatabase(database)
		time.Sleep(50 * time.Millisecond)
		notifiers.RemoveTestNotifier(notifier)
		storages.RemoveTestStorage(storage.ID)
		workspaces_testing.RemoveTestWorkspace(workspace, router)
	}()

	interval := createTestInterval()

	backupConfig := &backups_config.BackupConfig{
		DatabaseID:            database.ID,
		IsBackupsEnabled:      true,
		RetentionPolicyType:   backups_config.RetentionPolicyTypeTimePeriod,
		RetentionTimePeriod:   period.PeriodForever,
		StorageID:             &storage.ID,
		MaxBackupsTotalSizeMB: 100,
		BackupIntervalID:      interval.ID,
		BackupInterval:        interval,
	}
	_, err := backups_config.GetBackupConfigService().SaveBackupConfig(backupConfig)
	assert.NoError(t, err)

	for i := 0; i < 3; i++ {
		backup := &backups_core.Backup{
			ID:           uuid.New(),
			DatabaseID:   database.ID,
			StorageID:    storage.ID,
			Status:       backups_core.BackupStatusCompleted,
			BackupSizeMb: 16.67,
			CreatedAt:    time.Now().UTC().Add(-time.Duration(i) * time.Hour),
		}
		err = backupRepository.Save(backup)
		assert.NoError(t, err)
	}

	cleaner := GetBackupCleaner()
	err = cleaner.cleanExceededBackups()
	assert.NoError(t, err)

	remainingBackups, err := backupRepository.FindByDatabaseID(database.ID)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(remainingBackups))
}

func Test_CleanExceededBackups_WhenOverLimit_DeletesOldestBackups(t *testing.T) {
	router := CreateTestRouter()
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := storages.CreateTestStorage(workspace.ID)
	notifier := notifiers.CreateTestNotifier(workspace.ID)
	database := databases.CreateTestDatabase(workspace.ID, storage, notifier)

	defer func() {
		backups, _ := backupRepository.FindByDatabaseID(database.ID)
		for _, backup := range backups {
			backupRepository.DeleteByID(backup.ID)
		}

		databases.RemoveTestDatabase(database)
		time.Sleep(50 * time.Millisecond)
		notifiers.RemoveTestNotifier(notifier)
		storages.RemoveTestStorage(storage.ID)
		workspaces_testing.RemoveTestWorkspace(workspace, router)
	}()

	interval := createTestInterval()

	backupConfig := &backups_config.BackupConfig{
		DatabaseID:            database.ID,
		IsBackupsEnabled:      true,
		RetentionPolicyType:   backups_config.RetentionPolicyTypeTimePeriod,
		RetentionTimePeriod:   period.PeriodForever,
		StorageID:             &storage.ID,
		MaxBackupsTotalSizeMB: 30,
		BackupIntervalID:      interval.ID,
		BackupInterval:        interval,
	}
	_, err := backups_config.GetBackupConfigService().SaveBackupConfig(backupConfig)
	assert.NoError(t, err)

	now := time.Now().UTC()
	var backupIDs []uuid.UUID
	for i := 0; i < 5; i++ {
		backup := &backups_core.Backup{
			ID:           uuid.New(),
			DatabaseID:   database.ID,
			StorageID:    storage.ID,
			Status:       backups_core.BackupStatusCompleted,
			BackupSizeMb: 10,
			CreatedAt:    now.Add(-time.Duration(4-i) * time.Hour),
		}
		err = backupRepository.Save(backup)
		assert.NoError(t, err)
		backupIDs = append(backupIDs, backup.ID)
	}

	cleaner := GetBackupCleaner()
	err = cleaner.cleanExceededBackups()
	assert.NoError(t, err)

	remainingBackups, err := backupRepository.FindByDatabaseID(database.ID)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(remainingBackups))

	remainingIDs := make(map[uuid.UUID]bool)
	for _, backup := range remainingBackups {
		remainingIDs[backup.ID] = true
	}
	assert.False(t, remainingIDs[backupIDs[0]])
	assert.False(t, remainingIDs[backupIDs[1]])
	assert.True(t, remainingIDs[backupIDs[2]])
	assert.True(t, remainingIDs[backupIDs[3]])
	assert.True(t, remainingIDs[backupIDs[4]])
}

func Test_CleanExceededBackups_SkipsInProgressBackups(t *testing.T) {
	router := CreateTestRouter()
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := storages.CreateTestStorage(workspace.ID)
	notifier := notifiers.CreateTestNotifier(workspace.ID)
	database := databases.CreateTestDatabase(workspace.ID, storage, notifier)

	defer func() {
		backups, _ := backupRepository.FindByDatabaseID(database.ID)
		for _, backup := range backups {
			backupRepository.DeleteByID(backup.ID)
		}

		databases.RemoveTestDatabase(database)
		time.Sleep(50 * time.Millisecond)
		notifiers.RemoveTestNotifier(notifier)
		storages.RemoveTestStorage(storage.ID)
		workspaces_testing.RemoveTestWorkspace(workspace, router)
	}()

	interval := createTestInterval()

	backupConfig := &backups_config.BackupConfig{
		DatabaseID:            database.ID,
		IsBackupsEnabled:      true,
		RetentionPolicyType:   backups_config.RetentionPolicyTypeTimePeriod,
		RetentionTimePeriod:   period.PeriodForever,
		StorageID:             &storage.ID,
		MaxBackupsTotalSizeMB: 50,
		BackupIntervalID:      interval.ID,
		BackupInterval:        interval,
	}
	_, err := backups_config.GetBackupConfigService().SaveBackupConfig(backupConfig)
	assert.NoError(t, err)

	now := time.Now().UTC()

	completedBackups := make([]*backups_core.Backup, 3)
	for i := 0; i < 3; i++ {
		backup := &backups_core.Backup{
			ID:           uuid.New(),
			DatabaseID:   database.ID,
			StorageID:    storage.ID,
			Status:       backups_core.BackupStatusCompleted,
			BackupSizeMb: 30,
			CreatedAt:    now.Add(-time.Duration(3-i) * time.Hour),
		}
		err = backupRepository.Save(backup)
		assert.NoError(t, err)
		completedBackups[i] = backup
	}

	inProgressBackup := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusInProgress,
		BackupSizeMb: 10,
		CreatedAt:    now,
	}
	err = backupRepository.Save(inProgressBackup)
	assert.NoError(t, err)

	cleaner := GetBackupCleaner()
	err = cleaner.cleanExceededBackups()
	assert.NoError(t, err)

	remainingBackups, err := backupRepository.FindByDatabaseID(database.ID)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(remainingBackups), 2)

	var inProgressFound bool
	for _, backup := range remainingBackups {
		if backup.ID == inProgressBackup.ID {
			inProgressFound = true
			assert.Equal(t, backups_core.BackupStatusInProgress, backup.Status)
		}
	}
	assert.True(t, inProgressFound, "In-progress backup should not be deleted")
}

func Test_CleanExceededBackups_WithZeroLimit_SkipsDatabase(t *testing.T) {
	router := CreateTestRouter()
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := storages.CreateTestStorage(workspace.ID)
	notifier := notifiers.CreateTestNotifier(workspace.ID)
	database := databases.CreateTestDatabase(workspace.ID, storage, notifier)

	defer func() {
		backups, _ := backupRepository.FindByDatabaseID(database.ID)
		for _, backup := range backups {
			backupRepository.DeleteByID(backup.ID)
		}

		databases.RemoveTestDatabase(database)
		time.Sleep(50 * time.Millisecond)
		notifiers.RemoveTestNotifier(notifier)
		storages.RemoveTestStorage(storage.ID)
		workspaces_testing.RemoveTestWorkspace(workspace, router)
	}()

	interval := createTestInterval()

	backupConfig := &backups_config.BackupConfig{
		DatabaseID:            database.ID,
		IsBackupsEnabled:      true,
		RetentionPolicyType:   backups_config.RetentionPolicyTypeTimePeriod,
		RetentionTimePeriod:   period.PeriodForever,
		StorageID:             &storage.ID,
		MaxBackupsTotalSizeMB: 0,
		BackupIntervalID:      interval.ID,
		BackupInterval:        interval,
	}
	_, err := backups_config.GetBackupConfigService().SaveBackupConfig(backupConfig)
	assert.NoError(t, err)

	for i := 0; i < 10; i++ {
		backup := &backups_core.Backup{
			ID:           uuid.New(),
			DatabaseID:   database.ID,
			StorageID:    storage.ID,
			Status:       backups_core.BackupStatusCompleted,
			BackupSizeMb: 100,
			CreatedAt:    time.Now().UTC().Add(-time.Duration(i) * time.Hour),
		}
		err = backupRepository.Save(backup)
		assert.NoError(t, err)
	}

	cleaner := GetBackupCleaner()
	err = cleaner.cleanExceededBackups()
	assert.NoError(t, err)

	remainingBackups, err := backupRepository.FindByDatabaseID(database.ID)
	assert.NoError(t, err)
	assert.Equal(t, 10, len(remainingBackups))
}

func Test_GetTotalSizeByDatabase_CalculatesCorrectly(t *testing.T) {
	router := CreateTestRouter()
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := storages.CreateTestStorage(workspace.ID)
	notifier := notifiers.CreateTestNotifier(workspace.ID)
	database := databases.CreateTestDatabase(workspace.ID, storage, notifier)

	defer func() {
		backups, _ := backupRepository.FindByDatabaseID(database.ID)
		for _, backup := range backups {
			backupRepository.DeleteByID(backup.ID)
		}

		databases.RemoveTestDatabase(database)
		time.Sleep(50 * time.Millisecond)
		notifiers.RemoveTestNotifier(notifier)
		storages.RemoveTestStorage(storage.ID)
		workspaces_testing.RemoveTestWorkspace(workspace, router)
	}()

	completedBackup1 := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 10.5,
		CreatedAt:    time.Now().UTC(),
	}
	completedBackup2 := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 20.3,
		CreatedAt:    time.Now().UTC(),
	}
	failedBackup := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusFailed,
		BackupSizeMb: 5.2,
		CreatedAt:    time.Now().UTC(),
	}
	inProgressBackup := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusInProgress,
		BackupSizeMb: 100,
		CreatedAt:    time.Now().UTC(),
	}

	err := backupRepository.Save(completedBackup1)
	assert.NoError(t, err)
	err = backupRepository.Save(completedBackup2)
	assert.NoError(t, err)
	err = backupRepository.Save(failedBackup)
	assert.NoError(t, err)
	err = backupRepository.Save(inProgressBackup)
	assert.NoError(t, err)

	totalSize, err := backupRepository.GetTotalSizeByDatabase(database.ID)
	assert.NoError(t, err)
	assert.InDelta(t, 36.0, totalSize, 0.1)
}

func Test_CleanByCount_KeepsNewestNBackups_DeletesOlder(t *testing.T) {
	router := CreateTestRouter()
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := storages.CreateTestStorage(workspace.ID)
	notifier := notifiers.CreateTestNotifier(workspace.ID)
	database := databases.CreateTestDatabase(workspace.ID, storage, notifier)

	defer func() {
		backups, _ := backupRepository.FindByDatabaseID(database.ID)
		for _, backup := range backups {
			backupRepository.DeleteByID(backup.ID)
		}

		databases.RemoveTestDatabase(database)
		time.Sleep(50 * time.Millisecond)
		notifiers.RemoveTestNotifier(notifier)
		storages.RemoveTestStorage(storage.ID)
		workspaces_testing.RemoveTestWorkspace(workspace, router)
	}()

	interval := createTestInterval()

	backupConfig := &backups_config.BackupConfig{
		DatabaseID:          database.ID,
		IsBackupsEnabled:    true,
		RetentionPolicyType: backups_config.RetentionPolicyTypeCount,
		RetentionCount:      3,
		StorageID:           &storage.ID,
		BackupIntervalID:    interval.ID,
		BackupInterval:      interval,
	}
	_, err := backups_config.GetBackupConfigService().SaveBackupConfig(backupConfig)
	assert.NoError(t, err)

	now := time.Now().UTC()
	var backupIDs []uuid.UUID
	for i := 0; i < 5; i++ {
		backup := &backups_core.Backup{
			ID:           uuid.New(),
			DatabaseID:   database.ID,
			StorageID:    storage.ID,
			Status:       backups_core.BackupStatusCompleted,
			BackupSizeMb: 10,
			CreatedAt: now.Add(
				-time.Duration(4-i) * time.Hour,
			), // oldest first in loop, newest = i=4
		}
		err = backupRepository.Save(backup)
		assert.NoError(t, err)
		backupIDs = append(backupIDs, backup.ID)
	}

	cleaner := GetBackupCleaner()
	err = cleaner.cleanByRetentionPolicy()
	assert.NoError(t, err)

	remainingBackups, err := backupRepository.FindByDatabaseID(database.ID)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(remainingBackups))

	remainingIDs := make(map[uuid.UUID]bool)
	for _, backup := range remainingBackups {
		remainingIDs[backup.ID] = true
	}
	assert.False(t, remainingIDs[backupIDs[0]], "Oldest backup should be deleted")
	assert.False(t, remainingIDs[backupIDs[1]], "2nd oldest backup should be deleted")
	assert.True(t, remainingIDs[backupIDs[2]], "3rd backup should remain")
	assert.True(t, remainingIDs[backupIDs[3]], "4th backup should remain")
	assert.True(t, remainingIDs[backupIDs[4]], "Newest backup should remain")
}

func Test_CleanByCount_WhenUnderLimit_NoBackupsDeleted(t *testing.T) {
	router := CreateTestRouter()
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := storages.CreateTestStorage(workspace.ID)
	notifier := notifiers.CreateTestNotifier(workspace.ID)
	database := databases.CreateTestDatabase(workspace.ID, storage, notifier)

	defer func() {
		backups, _ := backupRepository.FindByDatabaseID(database.ID)
		for _, backup := range backups {
			backupRepository.DeleteByID(backup.ID)
		}

		databases.RemoveTestDatabase(database)
		time.Sleep(50 * time.Millisecond)
		notifiers.RemoveTestNotifier(notifier)
		storages.RemoveTestStorage(storage.ID)
		workspaces_testing.RemoveTestWorkspace(workspace, router)
	}()

	interval := createTestInterval()

	backupConfig := &backups_config.BackupConfig{
		DatabaseID:          database.ID,
		IsBackupsEnabled:    true,
		RetentionPolicyType: backups_config.RetentionPolicyTypeCount,
		RetentionCount:      10,
		StorageID:           &storage.ID,
		BackupIntervalID:    interval.ID,
		BackupInterval:      interval,
	}
	_, err := backups_config.GetBackupConfigService().SaveBackupConfig(backupConfig)
	assert.NoError(t, err)

	for i := 0; i < 5; i++ {
		backup := &backups_core.Backup{
			ID:           uuid.New(),
			DatabaseID:   database.ID,
			StorageID:    storage.ID,
			Status:       backups_core.BackupStatusCompleted,
			BackupSizeMb: 10,
			CreatedAt:    time.Now().UTC().Add(-time.Duration(i) * time.Hour),
		}
		err = backupRepository.Save(backup)
		assert.NoError(t, err)
	}

	cleaner := GetBackupCleaner()
	err = cleaner.cleanByRetentionPolicy()
	assert.NoError(t, err)

	remainingBackups, err := backupRepository.FindByDatabaseID(database.ID)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(remainingBackups))
}

func Test_CleanByCount_DoesNotDeleteInProgressBackups(t *testing.T) {
	router := CreateTestRouter()
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := storages.CreateTestStorage(workspace.ID)
	notifier := notifiers.CreateTestNotifier(workspace.ID)
	database := databases.CreateTestDatabase(workspace.ID, storage, notifier)

	defer func() {
		backups, _ := backupRepository.FindByDatabaseID(database.ID)
		for _, backup := range backups {
			backupRepository.DeleteByID(backup.ID)
		}

		databases.RemoveTestDatabase(database)
		time.Sleep(50 * time.Millisecond)
		notifiers.RemoveTestNotifier(notifier)
		storages.RemoveTestStorage(storage.ID)
		workspaces_testing.RemoveTestWorkspace(workspace, router)
	}()

	interval := createTestInterval()

	backupConfig := &backups_config.BackupConfig{
		DatabaseID:          database.ID,
		IsBackupsEnabled:    true,
		RetentionPolicyType: backups_config.RetentionPolicyTypeCount,
		RetentionCount:      2,
		StorageID:           &storage.ID,
		BackupIntervalID:    interval.ID,
		BackupInterval:      interval,
	}
	_, err := backups_config.GetBackupConfigService().SaveBackupConfig(backupConfig)
	assert.NoError(t, err)

	now := time.Now().UTC()

	for i := 0; i < 3; i++ {
		backup := &backups_core.Backup{
			ID:           uuid.New(),
			DatabaseID:   database.ID,
			StorageID:    storage.ID,
			Status:       backups_core.BackupStatusCompleted,
			BackupSizeMb: 10,
			CreatedAt:    now.Add(-time.Duration(3-i) * time.Hour),
		}
		err = backupRepository.Save(backup)
		assert.NoError(t, err)
	}

	inProgressBackup := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusInProgress,
		BackupSizeMb: 5,
		CreatedAt:    now,
	}
	err = backupRepository.Save(inProgressBackup)
	assert.NoError(t, err)

	cleaner := GetBackupCleaner()
	err = cleaner.cleanByRetentionPolicy()
	assert.NoError(t, err)

	remainingBackups, err := backupRepository.FindByDatabaseID(database.ID)
	assert.NoError(t, err)

	var inProgressFound bool
	for _, backup := range remainingBackups {
		if backup.ID == inProgressBackup.ID {
			inProgressFound = true
		}
	}
	assert.True(t, inProgressFound, "In-progress backup should not be deleted by count policy")
}

// Test_DeleteBackup_WhenStorageDeleteFails_BackupStillRemovedFromDatabase verifies resilience
// when storage becomes unavailable. Even if storage.DeleteFile fails (e.g., storage is offline,
// credentials changed, or storage was deleted), the backup record should still be removed from
// the database. This prevents orphaned backup records when storage is no longer accessible.
func Test_DeleteBackup_WhenStorageDeleteFails_BackupStillRemovedFromDatabase(t *testing.T) {
	router := CreateTestRouter()
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	testStorage := storages.CreateTestStorage(workspace.ID)
	notifier := notifiers.CreateTestNotifier(workspace.ID)
	database := databases.CreateTestDatabase(workspace.ID, testStorage, notifier)

	defer func() {
		backups, _ := backupRepository.FindByDatabaseID(database.ID)
		for _, backup := range backups {
			backupRepository.DeleteByID(backup.ID)
		}

		databases.RemoveTestDatabase(database)
		time.Sleep(50 * time.Millisecond)
		notifiers.RemoveTestNotifier(notifier)
		storages.RemoveTestStorage(testStorage.ID)
		workspaces_testing.RemoveTestWorkspace(workspace, router)
	}()

	backup := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    testStorage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 10,
		CreatedAt:    time.Now().UTC(),
	}
	err := backupRepository.Save(backup)
	assert.NoError(t, err)

	cleaner := GetBackupCleaner()

	err = cleaner.DeleteBackup(backup)
	assert.NoError(t, err, "DeleteBackup should succeed even when storage file doesn't exist")

	deletedBackup, err := backupRepository.FindByID(backup.ID)
	assert.Error(t, err, "Backup should not exist in database")
	assert.Nil(t, deletedBackup)
}

func Test_CleanByTimePeriod_SkipsRecentBackup_EvenIfOlderThanRetention(t *testing.T) {
	router := CreateTestRouter()
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := storages.CreateTestStorage(workspace.ID)
	notifier := notifiers.CreateTestNotifier(workspace.ID)
	database := databases.CreateTestDatabase(workspace.ID, storage, notifier)

	defer func() {
		backups, _ := backupRepository.FindByDatabaseID(database.ID)
		for _, backup := range backups {
			backupRepository.DeleteByID(backup.ID)
		}

		databases.RemoveTestDatabase(database)
		time.Sleep(50 * time.Millisecond)
		notifiers.RemoveTestNotifier(notifier)
		storages.RemoveTestStorage(storage.ID)
		workspaces_testing.RemoveTestWorkspace(workspace, router)
	}()

	interval := createTestInterval()

	// Retention period is 1 day — any backup older than 1 day should be deleted.
	// But the recent backup was created only 30 minutes ago and must be preserved.
	backupConfig := &backups_config.BackupConfig{
		DatabaseID:          database.ID,
		IsBackupsEnabled:    true,
		RetentionPolicyType: backups_config.RetentionPolicyTypeTimePeriod,
		RetentionTimePeriod: period.PeriodDay,
		StorageID:           &storage.ID,
		BackupIntervalID:    interval.ID,
		BackupInterval:      interval,
	}
	_, err := backups_config.GetBackupConfigService().SaveBackupConfig(backupConfig)
	assert.NoError(t, err)

	now := time.Now().UTC()

	oldBackup := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 10,
		CreatedAt:    now.Add(-2 * 24 * time.Hour),
	}
	recentBackup := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 10,
		CreatedAt:    now.Add(-30 * time.Minute),
	}

	err = backupRepository.Save(oldBackup)
	assert.NoError(t, err)
	err = backupRepository.Save(recentBackup)
	assert.NoError(t, err)

	cleaner := GetBackupCleaner()
	err = cleaner.cleanByRetentionPolicy()
	assert.NoError(t, err)

	remainingBackups, err := backupRepository.FindByDatabaseID(database.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(remainingBackups))
	assert.Equal(t, recentBackup.ID, remainingBackups[0].ID)
}

func Test_CleanByCount_SkipsRecentBackup_EvenIfOverLimit(t *testing.T) {
	router := CreateTestRouter()
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := storages.CreateTestStorage(workspace.ID)
	notifier := notifiers.CreateTestNotifier(workspace.ID)
	database := databases.CreateTestDatabase(workspace.ID, storage, notifier)

	defer func() {
		backups, _ := backupRepository.FindByDatabaseID(database.ID)
		for _, backup := range backups {
			backupRepository.DeleteByID(backup.ID)
		}

		databases.RemoveTestDatabase(database)
		time.Sleep(50 * time.Millisecond)
		notifiers.RemoveTestNotifier(notifier)
		storages.RemoveTestStorage(storage.ID)
		workspaces_testing.RemoveTestWorkspace(workspace, router)
	}()

	interval := createTestInterval()

	// Retention count is 2 — 4 backups exist so 2 should be deleted.
	// The oldest backup in the "excess" tail was made 30 min ago — it must be preserved.
	backupConfig := &backups_config.BackupConfig{
		DatabaseID:          database.ID,
		IsBackupsEnabled:    true,
		RetentionPolicyType: backups_config.RetentionPolicyTypeCount,
		RetentionCount:      2,
		StorageID:           &storage.ID,
		BackupIntervalID:    interval.ID,
		BackupInterval:      interval,
	}
	_, err := backups_config.GetBackupConfigService().SaveBackupConfig(backupConfig)
	assert.NoError(t, err)

	now := time.Now().UTC()

	oldBackup1 := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 10,
		CreatedAt:    now.Add(-5 * time.Hour),
	}
	oldBackup2 := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 10,
		CreatedAt:    now.Add(-3 * time.Hour),
	}
	// This backup is 3rd newest and would normally be deleted — but it is recent.
	recentExcessBackup := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 10,
		CreatedAt:    now.Add(-30 * time.Minute),
	}
	newestBackup := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 10,
		CreatedAt:    now.Add(-10 * time.Minute),
	}

	for _, b := range []*backups_core.Backup{oldBackup1, oldBackup2, recentExcessBackup, newestBackup} {
		err = backupRepository.Save(b)
		assert.NoError(t, err)
	}

	cleaner := GetBackupCleaner()
	err = cleaner.cleanByRetentionPolicy()
	assert.NoError(t, err)

	remainingBackups, err := backupRepository.FindByDatabaseID(database.ID)
	assert.NoError(t, err)

	remainingIDs := make(map[uuid.UUID]bool)
	for _, backup := range remainingBackups {
		remainingIDs[backup.ID] = true
	}

	assert.False(t, remainingIDs[oldBackup1.ID], "Oldest non-recent backup should be deleted")
	assert.False(t, remainingIDs[oldBackup2.ID], "2nd oldest non-recent backup should be deleted")
	assert.True(
		t,
		remainingIDs[recentExcessBackup.ID],
		"Recent backup must be preserved despite being over limit",
	)
	assert.True(t, remainingIDs[newestBackup.ID], "Newest backup should be preserved")
}

func Test_CleanExceededBackups_SkipsRecentBackup_WhenOverTotalSizeLimit(t *testing.T) {
	router := CreateTestRouter()
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := storages.CreateTestStorage(workspace.ID)
	notifier := notifiers.CreateTestNotifier(workspace.ID)
	database := databases.CreateTestDatabase(workspace.ID, storage, notifier)

	defer func() {
		backups, _ := backupRepository.FindByDatabaseID(database.ID)
		for _, backup := range backups {
			backupRepository.DeleteByID(backup.ID)
		}

		databases.RemoveTestDatabase(database)
		time.Sleep(50 * time.Millisecond)
		notifiers.RemoveTestNotifier(notifier)
		storages.RemoveTestStorage(storage.ID)
		workspaces_testing.RemoveTestWorkspace(workspace, router)
	}()

	interval := createTestInterval()

	// Total size limit is 10 MB. We have two backups of 8 MB each (16 MB total).
	// The oldest backup was created 30 minutes ago — within the grace period.
	// The cleaner must stop and leave both backups intact.
	backupConfig := &backups_config.BackupConfig{
		DatabaseID:            database.ID,
		IsBackupsEnabled:      true,
		RetentionPolicyType:   backups_config.RetentionPolicyTypeTimePeriod,
		RetentionTimePeriod:   period.PeriodForever,
		StorageID:             &storage.ID,
		MaxBackupsTotalSizeMB: 10,
		BackupIntervalID:      interval.ID,
		BackupInterval:        interval,
	}
	_, err := backups_config.GetBackupConfigService().SaveBackupConfig(backupConfig)
	assert.NoError(t, err)

	now := time.Now().UTC()

	olderRecentBackup := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 8,
		CreatedAt:    now.Add(-30 * time.Minute),
	}
	newerRecentBackup := &backups_core.Backup{
		ID:           uuid.New(),
		DatabaseID:   database.ID,
		StorageID:    storage.ID,
		Status:       backups_core.BackupStatusCompleted,
		BackupSizeMb: 8,
		CreatedAt:    now.Add(-10 * time.Minute),
	}

	err = backupRepository.Save(olderRecentBackup)
	assert.NoError(t, err)
	err = backupRepository.Save(newerRecentBackup)
	assert.NoError(t, err)

	cleaner := GetBackupCleaner()
	err = cleaner.cleanExceededBackups()
	assert.NoError(t, err)

	remainingBackups, err := backupRepository.FindByDatabaseID(database.ID)
	assert.NoError(t, err)
	assert.Equal(
		t,
		2,
		len(remainingBackups),
		"Both recent backups must be preserved even though total size exceeds limit",
	)
}

// Mock listener for testing
type mockBackupRemoveListener struct {
	onBeforeBackupRemove func(*backups_core.Backup) error
}

func (m *mockBackupRemoveListener) OnBeforeBackupRemove(backup *backups_core.Backup) error {
	if m.onBeforeBackupRemove != nil {
		return m.onBeforeBackupRemove(backup)
	}

	return nil
}

func createTestInterval() *intervals.Interval {
	timeOfDay := "04:00"
	interval := &intervals.Interval{
		Interval:  intervals.IntervalDaily,
		TimeOfDay: &timeOfDay,
	}

	err := storage.GetDb().Create(interval).Error
	if err != nil {
		panic(err)
	}

	return interval
}
