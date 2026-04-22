package appkit

import (
	"time"
)

// go build -ldflags "-X github.com/tianjinli/dragz/pkg/appkit.Version=0.0.1 \
// -X github.com/tianjinli/dragz/pkg/appkit.Author=tianjinli@github.com"
var (
	Version = "0.0.1"                // Container version (injection only)
	Author  = "tianjinli@github.com" // Container author (injection only)

	Debug   = false
	Name    = "dragz"  // default server name
	Profile = "local"  // default active profile
	Source  = "file"   // default config source
	Catalog = "assets" // default resources catalog
)

type (
	// AppConfig is bootstrap settings
	AppConfig struct {
		Debug    bool   `yaml:"debug"`   // debug mode (default:false)
		Name     string `yaml:"name"`    // server name (default: dragz)
		Profile  string `yaml:"profile"` // active profile (default: local)
		Source   string `yaml:"source"`  // config source (default: file, options: nacos, etcd)
		Catalog  string `yaml:"catalog"` // resources catalog (default: ./assets)
		Metadata any    `yaml:"-"`       // nacos / etcd client metadata
	}
	// TunnelConfig is tunnel settings
	TunnelConfig struct {
		Scheme   string `yaml:"scheme"`   // tunnel type (noop, ssh)
		Host     string `yaml:"host"`     // tunnel host
		Port     uint16 `yaml:"port"`     // tunnel port
		Username string `yaml:"username"` // tunnel username
		Password string `yaml:"password"` // tunnel password
		//BindHost   string        `yaml:"bind-host"`   // tunnel bind host (default: 127.0.0.1)
		//BindPort   uint16        `yaml:"bind-port"`   // tunnel bind port (default: random)
		Timeout    time.Duration `yaml:"timeout"`     // tunnel connection timeout (default: 10s)
		KnownHosts string        `yaml:"known-hosts"` // ssh known hosts file path (default: ./assets/known_hosts)
		PrivateKey string        `yaml:"private-key"` // ssh private key file path (default: "%USERPROFILE%\.ssh\id_rsa")
		Passphrase string        `yaml:"passphrase"`  // passphrase for private key
	}
	// StaticConfig holds static resource settings.
	StaticConfig struct {
		// Name of the resource
		Name string `yaml:"name"`
		// URI path for access, e.g. "/assets" or "index.html"
		URIPath string `yaml:"uri-path"`
		// Filesystem path, e.g. "/var/www/assets" (dir) or "/var/www/index.html" (file)
		FSPath string `yaml:"fs-path"`
	}
	// ServerConfig is Server settings
	ServerConfig struct {
		Port uint16 // HTTP service port
		// Deprecated: If timeout is 10s but a DB operation takes 31s,
		// this may trigger 3 consecutive timeout responses.
		Timeout   time.Duration  // request timeout (default: 0s - means no timeout)
		BasePath  string         `yaml:"base-path"`  // Base URL path (the trailing '/' will be removed)
		Locale    string         `yaml:"locale"`     // supported locales: en_US, zh_Hans, zh_Hant (default: en_US)
		Expose    *TunnelConfig  `yaml:"expose"`     // expose the web service through an SSH tunnel
		Socks5    *TunnelConfig  `yaml:"socks5"`     // use a remote machine as a network proxy (e.g., HTTP).
		Public    []StaticConfig `yaml:"public"`     // static resources for public access
		Protected []StaticConfig `yaml:"protected"`  // static resources for protected access
		TokenPath string         `yaml:"token-path"` // Internal Access Token generator available only in Debug environment
	}
	// TokenConfig is JWT settings
	TokenConfig struct {
		// YAML fields typically use snake_case to align with most tools and conventions
		AccessSecretKey  string        `yaml:"access-secret-key"`  // secret key for signing access tokens
		AccessExpiresIn  time.Duration `yaml:"access-expires-in"`  // access token expires in (range: 1m - 24h)
		RefreshSecretKey string        `yaml:"refresh-secret-key"` // secret key for signing refresh tokens
		RefreshExpiresIn time.Duration `yaml:"refresh-expires-in"` // refresh token expires in (range: range: 1d/24h - 90d/2160h)
		IssuerUri        string        `yaml:"issuer-uri"`         // token issuer uri, usually your service domain
	}
	// LoggerConfig is logging settings
	LoggerConfig struct {
		Path  string // log file path
		Level string // log level (debug, *info*, warn, error, panic)
	}
	// SourceConfig is datasource connection settings
	SourceConfig struct {
		Driver   string        // datasource driver (default: mysql)
		Host     string        // hostname or IP
		Port     uint16        // port number
		User     string        // username
		Password string        // password
		DbName   string        `yaml:"db-name"` // database name
		Params   string        // connection parameters
		Proxy    *TunnelConfig // optional: overrides DatabaseConfig.Proxy if set
	}
	// DatabaseConfig is GORM settings
	DatabaseConfig struct {
		Primary       string                   // primary datasource name (default: master)
		Sources       map[string]*SourceConfig // multiple data sources keyed by name
		Proxy         *TunnelConfig            // default proxy for all sources
		LogLevel      string                   `yaml:"log-level"`      // log level (info, warn, error, silent), affected by LoggerConfig.Level
		SlowThreshold uint16                   `yaml:"slow-threshold"` // slow query threshold (default: 100ms)
		TablePrefix   string                   `yaml:"table-prefix"`   // table name prefix (default: "")
		SingleTable   bool                     `yaml:"single-table"`   // use single table for all entities (default: false)
		AutoMigrate   bool                     `yaml:"auto-migrate"`   // if true, will create database if not exists, and auto create/update tables (schema migration).
	}
	// RedisConfig is cache settings
	RedisConfig struct {
		Host     string // hostname or IP
		Port     uint16 // port number
		Password string // password
		Db       int    // database index
		Proxy    *TunnelConfig
	}
)

// Bootstrap is runtime settings (only used to unmarshal YAML results)
type Bootstrap struct {
	App      *AppConfig      `yaml:"-"`
	Server   *ServerConfig   `yaml:"Server"`
	Token    *TokenConfig    `yaml:"token"`
	Database *DatabaseConfig `yaml:"database"`
	Redis    *RedisConfig    `yaml:"redis"`
	Logger   *LoggerConfig   `yaml:"Logger"`
}
