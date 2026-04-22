//go:build nacos && !etcd

package config

import (
	"fmt"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/pkg/errors"
	"github.com/tianjinli/dragz/pkg/appkit"

	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/spf13/viper"
	"github.com/tianjinli/dragz/pkg/config"
	"github.com/tianjinli/dragz/pkg/nacos"
)

type NacosMetadata struct {
	Client config_client.IConfigClient
	Param  vo.ConfigParam
}

func ParseClientAndParam(metadata any) (config_client.IConfigClient, vo.ConfigParam, error) {
	switch v := metadata.(type) {
	case NacosMetadata:
		return v.Client, v.Param, nil
	case *NacosMetadata:
		return v.Client, v.Param, nil

	default:
		return nil, vo.ConfigParam{}, fmt.Errorf("unsupported meta type: %T", metadata)
	}
}

// LoadBootstrap Nacos config based on the active profile first, and finally
// load the runtime config using the service name and active profile.
func LoadBootstrap(v *viper.Viper, conf *appkit.AppConfig) (*appkit.Bootstrap, func(), error) {
	var client config_client.IConfigClient
	cleanup := func() {
		if client != nil {
			client.CloseClient()
		}
	}
	// Step 2 load additional configuration based on bootstrap settings
	cc := &nacos.ClientConf{}
	err := v.UnmarshalKey("nacos", cc, config.YamlTagDecoder)
	if err != nil {
		return nil, cleanup, errors.WithStack(err)
	}
	defaultNamespace := fmt.Sprintf("%s-%s", conf.Name, conf.Profile)
	client, err = cc.NewConfigClient(defaultNamespace)
	if err != nil {
		return nil, cleanup, errors.WithStack(err)
	}
	cc.DataId = "config.yaml"
	param := vo.ConfigParam{DataId: cc.DataId, Group: cc.Group}
	data, err := client.GetConfig(param)
	if err != nil {
		return nil, cleanup, errors.WithStack(err)
	}
	if data == "" {
		return nil, cleanup, fmt.Errorf("config is empty: %s", cc.DataId)
	}
	if err = config.ReverseReadConfig(v, strings.NewReader(data)); err != nil {
		return nil, cleanup, errors.WithStack(err)
	}

	conf.Metadata = &NacosMetadata{Client: client, Param: param}
	return &appkit.Bootstrap{}, cleanup, nil
}
