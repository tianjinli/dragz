package pluginapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackAssembleRoundTrip(t *testing.T) {
	repo := t.TempDir()
	backend := filepath.Join(repo, "backend")
	pluginDir := filepath.Join(backend, "plugins", "demo")
	mustMkdir(t, pluginDir)
	mustMkdir(t, filepath.Join(backend, "plugins", "all"))
	mustWrite(t, filepath.Join(backend, "plugins", "all", "all.go"), `package all

import (
	"dragz-admin/backend/internal/plugin"
	"dragz-admin/backend/plugins/system"
)

func All() []plugin.Plugin {
	return []plugin.Plugin{
		system.Plugin{},
		// scaffold:insert
	}
}
`)
	mustWrite(t, filepath.Join(pluginDir, "plugin.go"), "package demo\n")
	mustWrite(t, filepath.Join(pluginDir, ManifestFileName), `{
  "id": "demo",
  "name": "Demo",
  "version": "0.1.0",
  "dependsOn": ["system"],
  "engineRange": ">=1.0.0 <2.0.0",
  "components": ["/demo/index"]
}
`)
	pageDir := filepath.Join(repo, "frontend", "admin", "src", "pages", "demo")
	mustMkdir(t, pageDir)
	mustWrite(t, filepath.Join(pageDir, "index.tsx"), "export function DemoPage() { return null }\n")

	routerDir := filepath.Join(repo, "frontend", "admin", "src", "router")
	mustMkdir(t, routerDir)
	mustWrite(t, filepath.Join(routerDir, "page-map.tsx"), `export const pageMap = {
  '/example/index': null,
  '/profile/index': null,
};
`)
	mustWrite(t, filepath.Join(routerDir, "page-map.test.tsx"), `const backendComponents=['/example/index'];
`)

	locDir := filepath.Join(repo, "frontend", "admin", "src", "locales")
	mustMkdir(t, locDir)
	mustWrite(t, filepath.Join(locDir, "en.json"), `{
  "pages": {
    "demo": {
      "title": "Demo",
      "intro": "hello"
    },
    "other": {
      "title": "Other"
    }
  }
}
`)
	mustWrite(t, filepath.Join(locDir, "zh.json"), `{
  "pages": {
    "demo": {
      "title": "演示",
      "intro": "你好"
    }
  }
}
`)

	host := HostPaths{RepoRoot: repo, BackendRoot: backend}
	zipPath := filepath.Join(t.TempDir(), "plugin-demo-v0.1.0.zip")
	out, err := Pack(PackOptions{Host: host, PluginID: "demo", OutputZip: zipPath})
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("zip missing: %v", err)
	}

	// Simulate a clean host that only has system registration.
	repo2 := t.TempDir()
	backend2 := filepath.Join(repo2, "backend")
	mustMkdir(t, filepath.Join(backend2, "plugins", "all"))
	mustWrite(t, filepath.Join(backend2, "plugins", "all", "all.go"), `package all

import (
	"dragz-admin/backend/internal/plugin"
	"dragz-admin/backend/plugins/system"
)

func All() []plugin.Plugin {
	return []plugin.Plugin{
		system.Plugin{},
		// scaffold:insert
	}
}
`)
	mustMkdir(t, filepath.Join(repo2, "frontend", "admin", "src", "pages"))
	mustMkdir(t, filepath.Join(repo2, "frontend", "admin", "src", "router"))
	mustWrite(t, filepath.Join(repo2, "frontend", "admin", "src", "router", "page-map.tsx"), `export const pageMap = {
  '/profile/index': null,
};
`)
	mustWrite(t, filepath.Join(repo2, "frontend", "admin", "src", "router", "page-map.test.tsx"), `const backendComponents=['/system/user/index'];
`)
	mustMkdir(t, filepath.Join(repo2, "frontend", "admin", "src", "locales"))
	mustWrite(t, filepath.Join(repo2, "frontend", "admin", "src", "locales", "en.json"), `{
  "pages": {
    "example": {
      "title": "Example"
    }
  }
}
`)
	mustWrite(t, filepath.Join(repo2, "frontend", "admin", "src", "locales", "zh.json"), `{
  "pages": {
    "example": {
      "title": "示例"
    }
  }
}
`)

	res, err := Assemble(AssembleOptions{
		Host:    HostPaths{RepoRoot: repo2, BackendRoot: backend2},
		ZipPath: out,
		Force:   false,
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if res.Manifest.ID != "demo" {
		t.Fatalf("manifest id = %q", res.Manifest.ID)
	}
	if _, err := os.Stat(filepath.Join(backend2, "plugins", "demo", "plugin.go")); err != nil {
		t.Fatalf("plugin.go: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo2, "frontend", "admin", "src", "pages", "demo", "index.tsx")); err != nil {
		t.Fatalf("page: %v", err)
	}
	allGo, _ := os.ReadFile(filepath.Join(backend2, "plugins", "all", "all.go"))
	if !strings.Contains(string(allGo), "demo.Plugin{}") {
		t.Fatalf("all.go not patched: %s", allGo)
	}
	pm, _ := os.ReadFile(filepath.Join(repo2, "frontend", "admin", "src", "router", "page-map.tsx"))
	if !strings.Contains(string(pm), "'/demo/index'") {
		t.Fatalf("page-map not patched: %s", pm)
	}
	en, _ := os.ReadFile(filepath.Join(repo2, "frontend", "admin", "src", "locales", "en.json"))
	if !strings.Contains(string(en), `"demo"`) || !strings.Contains(string(en), "hello") {
		t.Fatalf("en locale missing demo: %s", en)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
