package logger

import (
	"log/slog"
	"os"
	"sync"
	"time"
)

var (
	loggerInstance *slog.Logger
	once           sync.Once
)

func Init(isDebug bool) {
	level := slog.LevelInfo
	if isDebug {
		level = slog.LevelDebug
	}

	once.Do(func() {
		loggerInstance = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					a.Value = slog.StringValue(time.Now().Format("2006/01/02 15:04:05"))
				}
				if a.Key == slog.LevelKey {
					return slog.Attr{}
				}

				return a
			},
		}))
	})
}

// GetLogger returns a singleton slog.Logger that logs to the console
func GetLogger() *slog.Logger {
	if loggerInstance == nil {
		Init(false)
	}

	return loggerInstance
}
