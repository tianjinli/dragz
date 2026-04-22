package database

import (
	"context"
	"fmt"
	"net"

	myx "github.com/go-sql-driver/mysql"
	"github.com/tianjinli/dragz/internal/bootstrap/tunnel"
	"github.com/tianjinli/dragz/pkg/appkit"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func NewMysqlDial(conf *appkit.SourceConfig, logger *zap.Logger, gormConf *gorm.Config, conn *tunnel.SshForward) (db *gorm.DB, err error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", conf.User, conf.Password, conf.Host, conf.Port, conf.DbName)
	if conf.Params != "" {
		dsn += "?" + conf.Params
	}

	if conn != nil {
		myx.RegisterDialContext("tcp", func(ctx context.Context, addr string) (net.Conn, error) {
			return conn.Dial(ctx, "tcp", addr)
		})
	}

	noPass := fmt.Sprintf("mysql://%s:******@tcp(%s:%d)/%s?%s", conf.User, conf.Host, conf.Port, conf.DbName, conf.Params)
	logger.Info("connect to mysql", zap.String("dsn", noPass))
	return gorm.Open(mysql.Open(dsn), gormConf)
}
