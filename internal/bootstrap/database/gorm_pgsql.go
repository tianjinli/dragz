package database

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pkg/errors"
	"github.com/tianjinli/dragz/internal/bootstrap/tunnel"
	"github.com/tianjinli/dragz/pkg/appkit"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewPostgresDial(conf *appkit.SourceConfig, logger *zap.Logger, gormConf *gorm.Config, conn *tunnel.SshForward) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
		conf.Host, conf.Port, conf.User, conf.Password, conf.DbName)
	if conf.Params != "" {
		dsn += " " + conf.Params
	}

	pgxCfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if conn != nil {
		pgxCfg.DialFunc = conn.Dial
	}
	noPass := fmt.Sprintf("postgres://%s:******@tcp(%s:%d)/%s?%s", conf.User, conf.Host, conf.Port, conf.DbName, conf.Params)
	logger.Info("connect to postgres", zap.String("dsn", noPass))
	return gorm.Open(postgres.New(postgres.Config{
		Conn: stdlib.OpenDB(*pgxCfg),
	}), gormConf)
}
