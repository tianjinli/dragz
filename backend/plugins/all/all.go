// Package all registers every compiled-in plugin. Adding a plugin = adding
// one import + one slice entry here (the new-plugin scaffold appends
// automatically at the //scaffold:insert marker).
package all

import (
	"dragz-admin/backend/internal/plugin"
	"dragz-admin/backend/plugins/example"
	"dragz-admin/backend/plugins/system"
)

// All returns the full plugin list in registration order; the kernel
// re-orders by DependsOn.
func All() []plugin.Plugin {
	return []plugin.Plugin{
		system.Plugin{},
		example.Plugin{},
		// scaffold:insert
	}
}
