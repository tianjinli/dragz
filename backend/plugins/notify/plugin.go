// Package notify ships outbound notification channels: SMTP email, Aliyun
// SMS and Tencent Cloud SMS, with channel config, templates and delivery logs.
//
// It is a compiled-in plugin and ships DISABLED: nothing exists in the
// database until an administrator enables it in 系统设置 → 插件中心. Secrets
// are encrypted with the host SecretBox and never stored in plain text.
package notify

import (
	"embed"
	"io/fs"
	"path"
	"strings"

	"dragz-admin/backend/internal/plugin"
	"dragz-admin/backend/platform/notifier"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Plugin implements plugin.Plugin and plugin.Migrator.
type Plugin struct{}

func (Plugin) Meta() plugin.Meta {
	return plugin.Meta{
		ID:          notifier.PluginID,
		Name:        "外部通知通道",
		Version:     "0.1.0",
		Description: "SMTP 邮件、阿里云短信、腾讯云短信：通道配置、消息模板、发送日志",
		DependsOn:   []string{"system"},
		Order:       20,
		Builtin:     false,
		Menus: []plugin.Menu{
			{
				Code:       "notify_page",
				ParentCode: "messages",
				Name:       "通知通道",
				NameEn:     "Notification Channels",
				RouteName:  "NotifyPage",
				RoutePath:  "/notify",
				Component:  "/notify/index",
				Icon:       "lucide:send",
				Sort:       910,
			},
		},
		DropTables: []string{"notify_channel", "notify_template", "notify_log"},
	}
}

// Routes returns the declarative table. The notification capability itself is
// installed by api.wireNotifier at startup; see notifier.go for the adapter.
func (p Plugin) Routes() []plugin.Route {
	return Routes()
}

func (Plugin) Migrations() []plugin.Migration {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		panic("notify: read embedded migrations: " + err.Error())
	}
	migrations := make([]plugin.Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		base := strings.TrimSuffix(entry.Name(), ".sql")
		version, name, found := strings.Cut(base, "_")
		if !found || version == "" {
			panic("notify: migration " + entry.Name() + " must be named <version>_<name>.sql")
		}
		body, err := migrationFS.ReadFile(path.Join("migrations", entry.Name()))
		if err != nil {
			panic("notify: read migration " + entry.Name() + ": " + err.Error())
		}
		migrations = append(migrations, plugin.Migration{Version: version, Name: name, SQL: string(body)})
	}
	return migrations
}
