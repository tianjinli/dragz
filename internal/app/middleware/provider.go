package middleware

import "github.com/google/wire"

// DependencyProviderSet middleware wire set provider
var DependencyProviderSet = wire.NewSet(
	wire.Struct(new(JwtAuthMiddleware), "*"),
)
