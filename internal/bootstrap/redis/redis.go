package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/tianjinli/dragz/internal/bootstrap/tunnel"
	"github.com/tianjinli/dragz/pkg/appkit"
	"go.uber.org/zap"
)

func NewRedis(conf *appkit.RedisConfig, logger *zap.Logger) (redis.UniversalClient, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addr := fmt.Sprintf("%s:%d", conf.Host, conf.Port)
	opt := &redis.Options{
		Addr:     addr,
		Password: conf.Password,
		DB:       conf.Db,
	}
	var err error
	var conn *tunnel.SshForward
	var client redis.UniversalClient
	cleanup := func() {
		if client != nil {
			_ = client.Close()
		}
		if conn != nil {
			_ = conn.Close()
		}
	}
	if conf.Proxy != nil {
		conn, err = tunnel.NewSshForward(conf.Proxy)
		if err != nil {
			return nil, cleanup, errors.WithStack(err)
		}
		if conn != nil {
			opt.Dialer = conn.Dial
		}
	}
	client = redis.NewClient(opt)

	noPass := fmt.Sprintf("redis://:******@tcp(%s:%d)/%d", conf.Host, conf.Port, conf.Db)
	logger.Info("connect to redis", zap.String("dsn", noPass))
	return client, cleanup, errors.WithStack(client.Ping(ctx).Err())
}
