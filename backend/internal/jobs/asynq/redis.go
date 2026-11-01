package jobasynq

import (
	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// RedisOpt projects the project's Redis configuration onto the asynq connection options.
func RedisOpt(conf redis.RedisConf) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     conf.Host,
		Password: conf.Pass,
	}
}
