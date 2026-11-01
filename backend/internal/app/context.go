// Package app is the application runtime: the shared Context every handler
// receives, its construction, and the authorization helpers built on top of
// it.
package app

import (
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"

	"dragz-admin/backend/internal/config"
	"dragz-admin/backend/internal/jobs"
	jobasynq "dragz-admin/backend/internal/jobs/asynq"
	"dragz-admin/backend/internal/pkg/casbinx"
	pkgi18n "dragz-admin/backend/internal/pkg/i18n"
	"dragz-admin/backend/internal/pkg/translate"
	"dragz-admin/backend/internal/pluginstate"
	"dragz-admin/backend/internal/security"
	"dragz-admin/backend/internal/store"
	redisstore "github.com/zeromicro/go-zero/core/stores/redis"
)

// Context is the dependency bag every handler receives: configuration, database, tokens, encryption, cache, jobs, translations and the plugin runtime.
type Context struct {
	Config          ConfigView
	AllowDevSecrets bool
	DB              *store.DB
	Auth            *store.AuthRepository
	Tokens          *security.TokenManager
	Secrets         *security.SecretBox
	Cache           *redisstore.Redis
	Jobs            *jobs.Service
	Translator      *pkgi18n.Translator
	// Translate is the free machine-translation client the translation page
	// uses to fill the target column from the Chinese source.
	Translate *translate.Client
	Casbin    *casbinx.Enforcer
	// Plugins answers "is this plugin enabled?" for the route gate. It is
	// injected by api.RegisterRoutes; nil means "everything is enabled", so a
	// test or a misconfigured binary degrades to the pre-plugin behaviour
	// instead of 404ing the whole API.
	Plugins pluginstate.Runtime

	// closers holds the cleanups registered through Defer, run by Close.
	closers []func()
}

// ConfigView is the slice of configuration the HTTP layer needs, kept separate so handlers never see the whole config tree.
type ConfigView struct {
	AccessSecret      string
	AccessExpire      int64
	RefreshExpire     int64
	Issuer            string
	RefreshCookieName string
	CookieSecure      bool
	CookieDomain      string
}

// NewContext wires the application together: database, Redis, secret box, bootstrap administrator, job client, translator and enforcer. Authorization is deliberately not loaded here; see the comment at the end of the function.
func NewContext(c config.Config) (*Context, error) {
	db, err := store.Open(c.Database.Driver, c.Database.DataSource)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	cache, err := redisstore.NewRedis(c.Redis)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	secretBox, err := security.NewSecretBox(c.Security.ConfigMasterKey)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize config encryption: %w", err)
	}
	authRepo := store.NewAuthRepository(db)
	if err := bootstrapAdmin(authRepo, c.Bootstrap); err != nil {
		_ = db.Close()
		return nil, err
	}

	jobClient := jobasynq.NewClient(c.Redis)
	jobService := jobs.NewService(authRepo, jobClient)

	translator, err := pkgi18n.NewTranslator()
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize i18n: %w", err)
	}
	// Built unconditionally: it holds no connection and reaches out only when a
	// translation is asked for, so a deployment that never uses it pays nothing.
	translateClient := translate.New(c.Translate.Providers, time.Duration(c.Translate.TimeoutMs)*time.Millisecond)
	enforcer, err := casbinx.NewEnforcer()
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize casbin: %w", err)
	}
	appCtx := &Context{
		Config: ConfigView{
			AccessSecret:      c.Auth.AccessSecret,
			AccessExpire:      c.Auth.AccessExpire,
			RefreshExpire:     c.Auth.RefreshExpire,
			Issuer:            c.Auth.Issuer,
			RefreshCookieName: c.Auth.RefreshCookieName,
			CookieSecure:      c.Auth.CookieSecure,
			CookieDomain:      c.Auth.CookieDomain,
		},
		AllowDevSecrets: c.AllowDevSecrets,
		DB:              db,
		Auth:            authRepo,
		Tokens:          security.NewTokenManager(c.Auth.AccessSecret, c.Auth.Issuer, c.Auth.AccessExpire, c.Auth.RefreshExpire),
		Secrets:         secretBox,
		Cache:           cache,
		Jobs:            jobService,
		Translator:      translator,
		Translate:       translateClient,
		Casbin:          enforcer,
	}
	// Authorization is NOT loaded here. The policy query reads sys_plugin,
	// which only exists after the plugin bootstrap DDL runs (it is not in
	// migrations/001_system_init.sql). api.RegisterRoutes reloads the enforcer
	// after the plugin startup, before the server starts accepting requests.
	return appCtx, nil
}

func bootstrapAdmin(repo *store.AuthRepository, c config.BootstrapConf) error {
	count, err := repo.CountUsers()
	if err != nil {
		return fmt.Errorf("count bootstrap users: %w", err)
	}
	if count > 0 {
		return nil
	}
	if c.AdminUsername == "" || c.AdminPassword == "" {
		return fmt.Errorf("bootstrap admin credentials are required for an empty database")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}
	if err := repo.CreateBootstrapAdmin(c.AdminUsername, string(hash)); err != nil {
		return fmt.Errorf("create bootstrap admin: %w", err)
	}
	logx.Infof("created bootstrap administrator %q; change its password immediately", c.AdminUsername)
	return nil
}

// Defer registers a cleanup function that runs when the context is closed, in
// reverse registration order. It exists so background goroutines started by the
// wiring layer (for example the plugin-state watcher) are stopped on shutdown
// without every caller having to thread a cancellation context through.
func (c *Context) Defer(fn func()) {
	if fn == nil {
		return
	}
	c.closers = append(c.closers, fn)
}

// Close stops the registered cleanups, then releases the job client and the
// database pool.
func (c *Context) Close() {
	for i := len(c.closers) - 1; i >= 0; i-- {
		c.closers[i]()
	}
	c.closers = nil
	if c.Jobs != nil {
		_ = c.Jobs.Close()
	}
	if c.DB != nil {
		_ = c.DB.Close()
	}
}
