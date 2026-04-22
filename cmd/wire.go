//go:build wireinject
// +build wireinject

package cmd

import (
	"github.com/google/wire"
	"github.com/tianjinli/dragz/internal/app/middleware"
	"github.com/tianjinli/dragz/internal/bootstrap"
	"github.com/tianjinli/dragz/internal/infra"
	"github.com/tianjinli/dragz/pkg/appkit"
)

func InitContainer() (*appkit.Container, func(), error) {
	wire.Build(
		bootstrap.DependencyProviderSet,
		//repo.DependencyProviderSet,
		//service.DependencyProviderSet,
		//controller.DependencyProviderSet,
		//websocket.DependencyProviderSet,
		middleware.DependencyProviderSet,
		//router.DependencyProviderSet,
		infra.DependencyProviderSet,
	)
	return nil, nil, nil
}
