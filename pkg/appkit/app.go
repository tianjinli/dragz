package appkit

// https://github.com/stretchr/testify

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"go.uber.org/zap"
)

const defaultShutdownTimeout = 10 * time.Second

type Container struct {
	Logger   *zap.Logger
	Database *gorm.DB
	Redis    redis.UniversalClient
	Config   *Bootstrap

	Qrcode    QrcodeService
	Engine    EngineService
	JwtAuth   JwtAuthService
	Translate I18nAdapter
}

func (c *Container) Run(ctx context.Context) error {
	// Collect the error from `ListenAndServe` using channel
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Engine.ListenAndServe()
	}()

	// Listen for system signals (SIGINT, SIGTERM)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	defer func(logger *zap.Logger) { _ = logger.Sync() }(c.Logger)

	// Wait for one of three conditions: external context canceled, Server crash, or signal received
	select {
	case <-ctx.Done():
		c.Logger.Info("start context canceled, shutting down")
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			c.Logger.Error("Server exited with error, shutdown", zap.Error(err))
			return err
		}
		c.Logger.Info("Server exited normally, shutdown")
		return nil
	case sig := <-sigCh:
		c.Logger.Info("signal received, shutting down", zap.Stringer("signal", sig))
	}
	return c.Stop(ctx)
}

func (c *Container) Stop(ctx context.Context) error {
	// Call `Shutdown` for graceful shutdown
	stopCtx, cancel := context.WithTimeout(ctx, defaultShutdownTimeout)
	defer cancel()
	if err := c.Engine.Shutdown(stopCtx); err != nil {
		c.Logger.Error("graceful shutdown failed", zap.Error(err))
		return err
	}
	c.Logger.Info("shutdown completed")
	return nil
}
