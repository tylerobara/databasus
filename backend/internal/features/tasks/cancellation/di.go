package task_cancellation

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"

	cache_utils "databasus-backend/internal/util/cache"
	"databasus-backend/internal/util/logger"
)

var taskCancelManager = &TaskCancelManager{
	sync.RWMutex{},
	make(map[uuid.UUID]context.CancelFunc),
	cache_utils.NewPubSubManager(),
	logger.GetLogger(),
}

func GetTaskCancelManager() *TaskCancelManager {
	return taskCancelManager
}

var (
	setupOnce sync.Once
	isSetup   atomic.Bool
)

func SetupDependencies() {
	wasAlreadySetup := isSetup.Load()

	setupOnce.Do(func() {
		taskCancelManager.StartSubscription()

		isSetup.Store(true)
	})

	if wasAlreadySetup {
		logger.GetLogger().Warn("SetupDependencies called multiple times, ignoring subsequent call")
	}
}
