// Package app is the application runtime: the shared Context every handler
// receives, its construction, and the authorization helpers built on top of
// it.
package app

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"

	"dragz-admin/backend/internal/config"
	"dragz-admin/backend/internal/jobs"
	jobasynq "dragz-admin/backend/internal/jobs/asynq"
	"dragz-admin/backend/internal/metrics"
	"dragz-admin/backend/internal/noticenotify"
	"dragz-admin/backend/internal/pkg/casbinx"
	pkgi18n "dragz-admin/backend/internal/pkg/i18n"
	"dragz-admin/backend/internal/pkg/translate"
	"dragz-admin/backend/internal/pluginstate"
	"dragz-admin/backend/internal/ratelimit"
	"dragz-admin/backend/internal/security"
	"dragz-admin/backend/internal/store"
	"dragz-admin/backend/internal/ws"
	"dragz-admin/backend/platform/billing"
	"dragz-admin/backend/platform/eventbus"
	"dragz-admin/backend/platform/notifier"
	redisstore "github.com/zeromicro/go-zero/core/stores/redis"
)

// Context is the dependency bag every handler receives: configuration, database, tokens, encryption, cache, jobs, translations and the plugin runtime.
type Context struct {
	Config          ConfigView
	AllowDevSecrets bool
	// DB is the RLS-bound application identity. Tenant requests must use it.
	DB *store.DB
	// SchemaDB is the schema-owner identity. Only bootstrap and plugin schema
	// migrations may use it; HTTP handlers never receive it.
	SchemaDB *store.DB
	// PlatformDB is the cross-tenant identity (BYPASSRLS) used by the platform
	// console, membership lookup and background relay.
	PlatformDB *store.DB
	Auth       *store.AuthRepository
	Tenants    *store.TenantRepository
	Tokens     *security.TokenManager
	Secrets    *security.SecretBox
	Cache      *redisstore.Redis
	// Billing verifies and normalizes payment provider webhooks. It is nil when
	// the provider configuration is invalid, which the webhook handler treats as
	// "not accepting callbacks".
	Billing billing.Verifier
	// Notifier is the outbound notification capability contributed by the
	// notify plugin. NewContext installs notifier.Noop; RegisterRoutes replaces
	// it with the live provider after the plugin catalog is up. Other plugins
	// depend on this interface, never on plugins/notify.
	Notifier notifier.Notifier
	// RateLimit enforces per-route request budgets. It is backed by Cache and
	// falls back to a per-process counter when Redis is unavailable.
	RateLimit *ratelimit.Limiter
	// Metrics meters every request. Nil when metrics.enabled is off, which makes
	// both the middleware and GET /api/metrics no-ops.
	Metrics    *metrics.Registry
	Jobs       *jobs.Service
	Translator *pkgi18n.Translator
	// Translate is the free machine-translation client the translation page
	// uses to fill the target column from the Chinese source.
	Translate *translate.Client
	Casbin    *casbinx.Enforcer
	// Plugins answers "is this plugin enabled?" for the route gate. It is
	// injected by api.RegisterRoutes; nil means "everything is enabled", so a
	// test or a misconfigured binary degrades to the pre-plugin behaviour
	// instead of 404ing the whole API.
	Plugins pluginstate.Runtime
	// Bus is the process-local event bus for cross-plugin collaboration.
	// Plugins must not import each other; they publish/subscribe here instead.
	Bus *eventbus.Bus
	// WS fans notification and notice events out to connected browsers.
	// Nil in tests that never publish; handlers skip the push in that case.
	WS *ws.Hub

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
	// TenantBaseDomain is the parent of {slug}.{base} hosts. Empty disables
	// slug subdomains; verified custom domains still resolve.
	TenantBaseDomain string
	// PlatformHosts are exact hosts that do not force a tenant. localhost,
	// 127.0.0.1, and every raw IP are platform hosts even when this is empty.
	PlatformHosts []string
}

// NewContext wires the application together: database, Redis, secret box, bootstrap administrator, job client, translator and enforcer. Authorization is deliberately not loaded here; see the comment at the end of the function.
func NewContext(c config.Config) (*Context, error) {
	db, err := store.Open(c.Database.Driver, c.Database.DataSource)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	schemaDB := db
	if dsn := c.Database.MigrateDataSource; dsn != "" && dsn != c.Database.DataSource {
		schemaDB, err = store.Open(c.Database.Driver, dsn)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("open schema database: %w", err)
		}
	}
	platformDB := db
	if dsn := c.Database.PlatformDataSource; dsn != "" && dsn != c.Database.DataSource {
		platformDB, err = store.Open(c.Database.Driver, dsn)
		if err != nil {
			_ = db.Close()
			if schemaDB != db {
				_ = schemaDB.Close()
			}
			return nil, fmt.Errorf("open platform database: %w", err)
		}
	}
	if !c.AllowDevSecrets {
		bypasses, role, roleErr := store.RoleBypassesRLS(context.Background(), db)
		if roleErr != nil {
			_ = db.Close()
			if schemaDB != db {
				_ = schemaDB.Close()
			}
			if platformDB != db && platformDB != schemaDB {
				_ = platformDB.Close()
			}
			return nil, fmt.Errorf("verify application database role: %w", roleErr)
		}
		if bypasses {
			_ = db.Close()
			if schemaDB != db {
				_ = schemaDB.Close()
			}
			if platformDB != db && platformDB != schemaDB {
				_ = platformDB.Close()
			}
			return nil, fmt.Errorf("database application role %q is a superuser or has BYPASSRLS; use a separate non-bypass role for Database.DataSource", role)
		}
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
	authRepo.SetPlatformDB(platformDB)
	baseDomain, platformHosts := tenantHostConfig(c.TenantHost)
	tenants := store.NewTenantRepository(db)
	tenants.SetPlatformDB(platformDB)
	tenants.SetBaseDomain(baseDomain)
	// Seed credentials are a schema-owner operation: sys_user_role is a
	// tenant table and an empty database has no request tenant yet.
	if err := bootstrapAdmin(store.NewAuthRepository(schemaDB), c.Bootstrap); err != nil {
		_ = db.Close()
		if schemaDB != db {
			_ = schemaDB.Close()
		}
		if platformDB != db && platformDB != schemaDB {
			_ = platformDB.Close()
		}
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
	hub, err := ws.NewHub(c.Redis.Host, c.Redis.Pass)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("start notification hub: %w", err)
	}
	billingVerifier, err := billing.NewVerifier(
		c.Billing.Provider, c.Billing.WebhookSecret, c.Billing.SignatureHeader,
		c.Billing.EventIDHeader, time.Duration(c.Billing.ToleranceSeconds)*time.Second,
	)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize billing provider: %w", err)
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
			TenantBaseDomain:  baseDomain,
			PlatformHosts:     platformHosts,
		},
		AllowDevSecrets: c.AllowDevSecrets,
		DB:              db,
		SchemaDB:        schemaDB,
		PlatformDB:      platformDB,
		Auth:            authRepo,
		Tenants:         tenants,
		Tokens:          security.NewTokenManager(c.Auth.AccessSecret, c.Auth.Issuer, c.Auth.AccessExpire, c.Auth.RefreshExpire),
		Secrets:         secretBox,
		Cache:           cache,
		RateLimit:       ratelimit.New(cache),
		Metrics:         metricsFor(c),
		Billing:         billingVerifier,
		Notifier:        notifier.Noop{},
		Jobs:            jobService,
		Translator:      translator,
		Translate:       translateClient,
		Casbin:          enforcer,
		Bus:             eventbus.New(),
		WS:              hub,
	}
	appCtx.Defer(hub.Close)
	appCtx.Defer(noticenotify.StartDispatcher(authRepo, hub, 30*time.Second))
	// Authorization is NOT loaded here. The policy query reads sys_plugin,
	// which only exists after the plugin bootstrap DDL runs (it is not in
	// migrations/001_system_init.sql). api.RegisterRoutes reloads the enforcer
	// after the plugin startup, before the server starts accepting requests.
	return appCtx, nil
}

// tenantHostConfig normalizes the host map. localhost and 127.0.0.1 are
// always platform hosts so a direct API call does not pick up a tenant.
func tenantHostConfig(c config.TenantHostConf) (string, []string) {
	seen := map[string]struct{}{}
	hosts := make([]string, 0, len(c.PlatformHosts)+2)
	add := func(host string) {
		normalized := store.NormalizeHost(host)
		if normalized == "" {
			return
		}
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
		hosts = append(hosts, normalized)
	}
	add("localhost")
	add("127.0.0.1")
	for _, host := range c.PlatformHosts {
		add(host)
	}
	return store.NormalizeHost(c.BaseDomain), hosts
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

// metricsFor builds the request meter only when the deployment asked for it.
// A nil registry is the "off" signal both the middleware and the scrape
// endpoint branch on, so there is no second switch to keep in sync.
func metricsFor(c config.Config) *metrics.Registry {
	if !c.Metrics.Enabled {
		return nil
	}
	return metrics.New()
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
	if c.PlatformDB != nil && c.PlatformDB != c.DB && c.PlatformDB != c.SchemaDB {
		_ = c.PlatformDB.Close()
	}
	if c.SchemaDB != nil && c.SchemaDB != c.DB {
		_ = c.SchemaDB.Close()
	}
	if c.DB != nil {
		_ = c.DB.Close()
	}
}
