// Command jobs-runner is the scheduler and worker process. It must start after
// admin-api, which owns the plugin migrations.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"

	"dragz-admin/backend/internal/config"
	"dragz-admin/backend/internal/jobs"
	"dragz-admin/backend/internal/jobs/runner"
	"dragz-admin/backend/internal/security"
	"dragz-admin/backend/internal/store"
)

var configFile = flag.String("f", "etc/jobs-runner.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	logx.Must(c.Validate())

	db, err := store.Open(c.Database.Driver, c.Database.DataSource)
	logx.Must(err)
	defer db.Close()

	var secretBox *security.SecretBox
	if key := strings.TrimSpace(c.Security.ConfigMasterKey); key != "" {
		secretBox, err = security.NewSecretBox(key)
		logx.Must(err)
	}
	repo := store.NewAuthRepository(db)
	executor := jobs.NewExecutor(repo, db, secretBox)
	r, err := runner.NewRunner(c, repo, executor)
	logx.Must(err)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logx.Info("starting jobs-runner")
	if err := r.Run(ctx); err != nil && err != context.Canceled {
		logx.Must(err)
	}
	logx.Info("jobs-runner stopped")
}
