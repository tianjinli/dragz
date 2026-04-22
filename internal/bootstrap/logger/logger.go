package logger

import (
	"github.com/pkg/errors"
	"github.com/tianjinli/dragz/pkg/appkit"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(conf *appkit.LoggerConfig) (*zap.Logger, func(), error) {
	// zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
	var cfg zap.Config
	if appkit.Debug {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}
	cfg.EncoderConfig = zapcore.EncoderConfig{
		TimeKey:      "timestamp",
		EncodeTime:   zapcore.ISO8601TimeEncoder, // UTC ISO8601
		LevelKey:     "level",
		EncodeLevel:  zapcore.CapitalLevelEncoder,
		CallerKey:    "caller",
		EncodeCaller: zapcore.ShortCallerEncoder,
		MessageKey:   "message",
	}
	level, _ := zapcore.ParseLevel(conf.Level)
	cfg.Level = zap.NewAtomicLevelAt(level)
	logger, err := cfg.Build(zap.AddCaller())
	cleanup := func() { _ = logger.Sync() }
	return logger, cleanup, errors.WithStack(err)
}
