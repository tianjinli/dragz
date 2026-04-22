//go:build nacos && !etcd

package setting

import (
	"fmt"
	"strings"

	"github.com/tianjinli/dragz/pkg/appkit"

	"github.com/tianjinli/dragz/internal/bootstrap/config"
	"go.uber.org/zap"
)

// loadAndWatch load the setting from viper and watch for changes
func (s *settingManager[T]) loadAndWatch(dataId string, metadata any) (appkit.SettingManager[T], error) {
	client, param, err := config.ParseClientAndParam(metadata)
	if err != nil {
		return nil, err
	}
	s.viper.SetConfigType("yaml")

	param.DataId = dataId
	data, err := client.GetConfig(param)
	if err != nil {
		return nil, err
	}
	if data == "" {
		return nil, fmt.Errorf("config is empty: %s", param.DataId)
	}
	if err = s.viper.ReadConfig(strings.NewReader(data)); err != nil {
		return nil, err
	}
	s.reloadSetting("")

	param.OnChange = func(namespace, group, dataId, data string) {
		s.logger.Info("reload setting", zap.String("nacos", dataId))
		if err = s.viper.ReadConfig(strings.NewReader(data)); err != nil {
			s.logger.Warn("reload setting failed", zap.String("nacos", dataId), zap.Error(err))
			return
		}
		s.reloadSetting("")
	}
	return s, client.ListenConfig(param)
}
