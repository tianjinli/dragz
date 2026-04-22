package infra

import (
	"github.com/google/wire"
	"github.com/tianjinli/dragz/internal/infra/engine"
	"github.com/tianjinli/dragz/internal/infra/jwtauth"
	"github.com/tianjinli/dragz/internal/infra/qrcode"
	"github.com/tianjinli/dragz/internal/infra/translate"
)

// DependencyProviderSet infra wire set provider
var DependencyProviderSet = wire.NewSet(
	engine.NewEngineService,
	jwtauth.NewJwtAuthService,
	qrcode.NewQrcodeService,
	translate.NewI18nAdapter,
)
