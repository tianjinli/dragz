package nacos

import (
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

type ClientConf struct {
	ServerHost  string `yaml:"server-host"`  // required,default:127.0.0.1
	ServerPort  uint64 `yaml:"server-port"`  // required,default:8848(>1024)
	ContextPath string `yaml:"context-path"` // 默认 mapstructure 才支持 - dash 连字符
	Username    string `yaml:"username"`     // required,default:nacos
	Password    string `yaml:"password"`     // required,default:nacos
	Namespace   string `yaml:"namespace"`    // required,default:${app.profile}
	Group       string `yaml:"group"`        // required,default:DEFAULT_GROUP
	DataId      string `yaml:"data-id"`      // required,auto set
	TimeoutMs   uint64 `yaml:"timeout-ms"`   // required,default:5000(>1000)
}

func (c *ClientConf) newClientParam(defaultNamespace string) vo.NacosClientParam {
	if len(c.ServerHost) < 3 {
		c.ServerHost = "127.0.0.1"
	}
	if c.ServerPort < 1024 {
		c.ServerPort = 8848
	}
	if c.TimeoutMs < 1000 {
		c.TimeoutMs = 5000
	}
	if c.Username == "" {
		c.Username = "nacos"
	}
	if c.Password == "" {
		c.Password = "nacos"
	}
	if c.Namespace == "" {
		c.Namespace = defaultNamespace
	}
	if c.Group == "" {
		c.Group = "DEFAULT_GROUP"
	}
	sc := []constant.ServerConfig{
		*constant.NewServerConfig(c.ServerHost, c.ServerPort, constant.WithContextPath(c.ContextPath)),
	}

	cc := constant.NewClientConfig(
		constant.WithNamespaceId(c.Namespace),
		constant.WithTimeoutMs(c.TimeoutMs),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithUsername(c.Username),
		constant.WithPassword(c.Password),
	)

	return vo.NacosClientParam{
		ClientConfig:  cc,
		ServerConfigs: sc,
	}
}

// NewConfigClient returns a Nacos config client.
func (c *ClientConf) NewConfigClient(defaultNamespace string) (config_client.IConfigClient, error) {
	return clients.NewConfigClient(c.newClientParam(defaultNamespace))
}

// NewNamingClient returns a Nacos naming client.
func (c *ClientConf) NewNamingClient(defaultNamespace string) (naming_client.INamingClient, error) {
	return clients.NewNamingClient(c.newClientParam(defaultNamespace))
}
