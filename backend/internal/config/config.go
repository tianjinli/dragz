package config

import (
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

// Config is the root configuration loaded from the YAML file passed via -f.
// It embeds go-zero's rest.RestConf (host, port, timeouts, logging) and adds the
// platform-specific sections below.
type Config struct {
	rest.RestConf
	Database  DatabaseConf
	Redis     redis.RedisConf
	Auth      AuthConf
	Bootstrap BootstrapConf
	Security  SecurityConf
	Asynq     AsynqConf
	Translate TranslateConf
	// AllowDevSecrets permits known development default secrets (local only).
	AllowDevSecrets bool `json:",optional"`
}

// TranslateConf configures the free machine-translation backends behind the
// translation management page. Providers are tried in order until one answers;
// an empty list falls back to mymemory then google. TimeoutMs bounds a single
// provider attempt, so the worst case for one request is TimeoutMs × providers.
//
// No endpoint needs a key, which is the point: a fresh install translates its
// first menu name without any setup. The trade-off is best-effort quality and a
// daily quota per IP, so every failure surfaces to the operator as "type the
// translation yourself" rather than a wrong column.
type TranslateConf struct {
	Providers []string `json:",optional"`
	TimeoutMs int      `json:",default=8000"`
}

// AsynqConf configures the background job worker pool. Queues maps a queue name
// to its priority weight; a higher weight means more workers drain that queue.
type AsynqConf struct {
	Concurrency int            `json:",default=10"`
	Queues      map[string]int `json:",optional"`
}

// DatabaseConf selects and locates the SQL store.
type DatabaseConf struct {
	// Driver is the SQL driver name. Currently only "postgres" is implemented.
	// Reserved for future MySQL (and other) store implementations.
	Driver     string `json:",default=postgres"`
	DataSource string
}

// AuthConf configures token issuance. AccessSecret signs JWTs, AccessExpire and
// RefreshExpire are lifetimes in seconds, and the Cookie* fields control the
// refresh-token cookie set on login.
type AuthConf struct {
	AccessSecret      string
	AccessExpire      int64
	RefreshExpire     int64
	Issuer            string
	RefreshCookieName string
	CookieSecure      bool
	CookieDomain      string
}

// BootstrapConf holds the initial administrator credentials, used only when the
// user table is still empty. Once an admin exists these values are ignored.
type BootstrapConf struct {
	AdminUsername string
	AdminPassword string
}

// SecurityConf holds the master key that encrypts secrets stored in the
// database (OAuth client secrets, storage access keys, webhook signing keys).
// Rotating it makes every previously encrypted value unreadable.
type SecurityConf struct {
	ConfigMasterKey string
}

var (
	defaultAccessSecret  = "development-only-replace-before-production-123456"
	defaultConfigMaster  = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	defaultBootstrapPass = "Admin12345!"
)

// Validate checks driver support and rejects known development secrets unless AllowDevSecrets is set.
func (c Config) Validate() error {
	driver := strings.ToLower(strings.TrimSpace(c.Database.Driver))
	if driver == "" {
		driver = "postgres"
	}
	switch driver {
	case "postgres", "postgresql":
	default:
		return fmt.Errorf("unsupported database driver %q (supported: postgres); see docs/NORMS_GAP.md for multi-db playbook", c.Database.Driver)
	}
	if c.AllowDevSecrets {
		return nil
	}
	if c.Auth.AccessSecret == "" || c.Auth.AccessSecret == defaultAccessSecret {
		return fmt.Errorf("Auth.AccessSecret must be set to a non-default value (or set AllowDevSecrets: true for local dev)")
	}
	if len(c.Auth.AccessSecret) < 32 {
		return fmt.Errorf("Auth.AccessSecret must be at least 32 characters")
	}
	if c.Security.ConfigMasterKey == "" || c.Security.ConfigMasterKey == defaultConfigMaster {
		return fmt.Errorf("Security.ConfigMasterKey must be set to a non-default value (or set AllowDevSecrets: true for local dev)")
	}
	if c.Bootstrap.AdminPassword == defaultBootstrapPass {
		return fmt.Errorf("Bootstrap.AdminPassword must not use the development default (or set AllowDevSecrets: true for local dev)")
	}
	return nil
}
