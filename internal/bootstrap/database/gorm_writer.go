package database

import (
	"fmt"
	"strings"

	"github.com/tianjinli/dragz/pkg/appkit"

	"go.uber.org/zap"
	"gorm.io/gorm/logger"
)

type gormZap struct {
	Logger   *zap.Logger
	LogLevel logger.LogLevel
}

func (l *gormZap) Printf(s string, i ...any) {
	msg := fmt.Sprintf(s, i...)
	switch l.LogLevel {
	case logger.Info:
		l.Logger.Info(msg)
	case logger.Warn:
		l.Logger.Warn(msg)
	case logger.Error:
		l.Logger.Error(msg)
	default:
		return
	}
}

// parseLevel is converts a string to a log level (default: silent)
func parseLevel(level string) logger.LogLevel {
	switch strings.ToLower(level) {
	case "info":
		return logger.Info
	case "warn":
		return logger.Warn
	case "error":
		return logger.Error
	default:
		return logger.Silent
	}
}

func newGormWriter(conf *appkit.DatabaseConfig, logger *zap.Logger) logger.Writer {
	return &gormZap{Logger: logger, LogLevel: parseLevel(conf.LogLevel)}
}
