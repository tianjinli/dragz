//go:build !nacos && !etcd

package setting

import (
	"github.com/fsnotify/fsnotify"
	"github.com/tianjinli/dragz/pkg/appkit"
	"go.uber.org/zap"
)

// loadAndWatch load the setting from viper and watch for changes
func (s *settingManager[T]) loadAndWatch(filename string, _ any) (appkit.SettingManager[T], error) {
	s.viper.SetConfigFile(filename)
	s.viper.SetConfigType("yaml")

	if err := s.viper.ReadInConfig(); err != nil {
		return nil, err
	}

	s.reloadSetting(filename)

	s.viper.WatchConfig()
	s.viper.OnConfigChange(func(e fsnotify.Event) {
		s.logger.Info("reload setting", zap.String("file", e.Name))
		s.reloadSetting(e.Name)
	})
	return s, nil
}
