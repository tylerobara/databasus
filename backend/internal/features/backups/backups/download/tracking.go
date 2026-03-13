package backups_download

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	cache_utils "databasus-backend/internal/util/cache"
)

const (
	downloadLockPrefix     = "backup_download_lock:"
	downloadLockTTL        = 5 * time.Second
	downloadLockValue      = "1"
	downloadHeartbeatDelay = 3 * time.Second
)

var ErrDownloadAlreadyInProgress = errors.New("download already in progress for this user")

type DownloadTracker struct {
	cache *cache_utils.CacheUtil[string]
}

func NewDownloadTracker(client valkey.Client) *DownloadTracker {
	return &DownloadTracker{
		cache: cache_utils.NewCacheUtil[string](client, downloadLockPrefix),
	}
}

func (t *DownloadTracker) AcquireDownloadLock(userID uuid.UUID) error {
	key := userID.String()

	existingLock := t.cache.Get(key)
	if existingLock != nil {
		return ErrDownloadAlreadyInProgress
	}

	value := downloadLockValue
	t.cache.Set(key, &value)

	return nil
}

func (t *DownloadTracker) RefreshDownloadLock(userID uuid.UUID) {
	key := userID.String()
	value := downloadLockValue
	t.cache.Set(key, &value)
}

func (t *DownloadTracker) ReleaseDownloadLock(userID uuid.UUID) {
	key := userID.String()
	t.cache.Invalidate(key)
}

func (t *DownloadTracker) IsDownloadInProgress(userID uuid.UUID) bool {
	key := userID.String()
	existingLock := t.cache.Get(key)
	return existingLock != nil
}

func GetDownloadHeartbeatInterval() time.Duration {
	return downloadHeartbeatDelay
}
