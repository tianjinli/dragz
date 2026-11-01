// Command admin-api is the HTTP server. It builds the application context,
// runs the plugin startup (bootstrap DDL, seed, migrations, button sync) and
// then serves the admin API.
package main

import (
	"flag"
	"fmt"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"

	"dragz-admin/backend/api"
	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/config"
)

var configFile = flag.String("f", "etc/admin-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	logx.Must(c.Validate())

	appContext, err := app.NewContext(c)
	logx.Must(err)
	defer appContext.Close()

	// WithNotFoundHandler makes an unmatched path answer with the same
	// envelope as a route whose plugin is disabled.
	server := rest.MustNewServer(c.RestConf, rest.WithNotFoundHandler(api.NotFoundHandler(appContext)))
	defer server.Stop()

	api.RegisterRoutes(server, appContext)
	fmt.Printf("Starting %s at %s:%d\n", c.Name, c.Host, c.Port)
	server.Start()
}
