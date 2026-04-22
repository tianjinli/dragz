//go:build !nacos && !etcd

package config

import (
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tianjinli/dragz/pkg/appkit"

	"github.com/spf13/viper"
	"github.com/tianjinli/dragz/pkg/config"
)

func LoadBootstrap(v *viper.Viper, conf *appkit.AppConfig) (*appkit.Bootstrap, func(), error) {
	cleanup := func() {}
	file := filepath.Join(conf.Catalog, "config.yaml")
	v.SetConfigFile(file)

	if err := config.ReverseReadInConfig(v); err != nil {
		return nil, cleanup, errors.WithStack(err)
	}
	return &appkit.Bootstrap{}, cleanup, nil
}
