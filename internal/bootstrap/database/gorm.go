package database

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/tianjinli/dragz/internal/bootstrap/tunnel"
	"github.com/tianjinli/dragz/pkg/appkit"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func NewGormDB(conf *appkit.DatabaseConfig, l *zap.Logger) (*gorm.DB, func(), error) {
	slowThreshold := time.Millisecond
	if conf.SlowThreshold < 10 {
		slowThreshold *= 10
	} else if conf.SlowThreshold > 1_0000 {
		slowThreshold *= 1_0000
	} else {
		slowThreshold *= time.Duration(conf.SlowThreshold)
	}
	var err error
	var conn *tunnel.SshForward
	var db *gorm.DB
	cleanup := func() {
		if db != nil {
			sqlDB, _ := db.DB()
			_ = sqlDB.Close()
		}
		if conn != nil {
			_ = conn.Close()
		}
	}
	source := conf.Sources[conf.Primary]
	if source == nil {
		return db, cleanup, errors.Errorf("not exist source: %s", conf.Primary)
	}
	if source.Proxy != nil {
		conn, err = tunnel.NewSshForward(source.Proxy)
	} else if conf.Proxy != nil {
		conn, err = tunnel.NewSshForward(conf.Proxy)
	}
	if err != nil {
		return db, cleanup, errors.WithStack(err)
	}
	var dbWriter = newGormWriter(conf, l)
	gormConf := &gorm.Config{
		Logger: logger.New(dbWriter, logger.Config{
			SlowThreshold: slowThreshold,
			LogLevel:      parseLevel(conf.LogLevel),
		}),
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   conf.TablePrefix,
			SingularTable: conf.SingleTable,
		},
	}
	switch strings.ToLower(source.Driver) {
	case "mysql":
		db, err = NewMysqlDial(source, l, gormConf, conn)
	case "postgres":
		db, err = NewPostgresDial(source, l, gormConf, conn)
	default:
		err = fmt.Errorf("unsupported dirver: %s", source.Driver)
	}
	return db, cleanup, errors.WithStack(err)
}
