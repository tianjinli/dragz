package config

import (
	_ "embed"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/tianjinli/dragz/pkg/appkit"
	"github.com/tianjinli/dragz/pkg/config"
)

// NewBootstrapConfig returns a bootstrap settings
func NewBootstrapConfig() (*appkit.Bootstrap, func(), error) {
	// Use the global Viper instance to make future merging easier.
	v := viper.GetViper()

	v.SetConfigFile("bootstrap.yaml")
	cleanup := func() {}
	if err := config.ReverseReadInConfig(v); err != nil {
		return nil, cleanup, errors.WithStack(err)
	}

	// Step 1 load bootstrap configuration from environment variables
	appConf := &appkit.AppConfig{}
	if err := v.UnmarshalKey("app", appConf, config.YamlTagDecoder); err != nil {
		return nil, cleanup, errors.WithStack(err)
	}
	appkit.Debug = appConf.Debug
	if appConf.Name == "" {
		appConf.Name = appkit.Name
	}
	if appConf.Profile == "" {
		appConf.Profile = appkit.Profile
	}
	if appConf.Source == "" {
		appConf.Source = appkit.Source
	}
	if appConf.Catalog == "" {
		appConf.Catalog = appkit.Catalog
	}
	bootConf, cleanup, err := LoadBootstrap(v, appConf)
	if err != nil {
		return nil, cleanup, errors.WithStack(err)
	}
	if err = v.Unmarshal(bootConf, config.YamlTagDecoder); err != nil {
		return nil, cleanup, errors.WithStack(err)
	}
	bootConf.App = appConf
	return bootConf, cleanup, nil
}
