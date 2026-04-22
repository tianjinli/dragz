package bootstrap

import (
	"github.com/google/wire"
	"github.com/tianjinli/dragz/internal/bootstrap/config"
	"github.com/tianjinli/dragz/internal/bootstrap/database"
	"github.com/tianjinli/dragz/internal/bootstrap/logger"
	"github.com/tianjinli/dragz/internal/bootstrap/redis"
	"github.com/tianjinli/dragz/pkg/appkit"
)

// DependencyProviderSet application wire set provider
var DependencyProviderSet = wire.NewSet(
	config.NewBootstrapConfig,
	wire.FieldsOf(new(*appkit.Bootstrap), // pointer to struct
		"App", "Server", "Token", "Database", "Redis", "Logger"),

	logger.NewLogger,
	database.NewGormDB,
	redis.NewRedis,

	wire.Struct(new(appkit.Container), "*"),
)
