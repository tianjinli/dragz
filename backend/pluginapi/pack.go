package pluginapi

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// HostPaths locates the monorepo roots pack/assemble need.
type HostPaths struct {
	RepoRoot    string
	BackendRoot string
}

// ResolveHostPaths finds the backend root (directory that contains
// plugins/all/all.go) and the repository root (parent of backend/).
func ResolveHostPaths(start string) (HostPaths, error) {
	if start == "" {
		wd, err := os.Getwd()
		if err != nil {
			return HostPaths{}, err
		}
		start = wd
	}
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "plugins", "all", "all.go")); err == nil {
			return HostPaths{BackendRoot: dir, RepoRoot: filepath.Dir(dir)}, nil
		}
		candidate := filepath.Join(dir, "backend")
		if _, err := os.Stat(filepath.Join(candidate, "plugins", "all", "all.go")); err == nil {
			return HostPaths{BackendRoot: candidate, RepoRoot: dir}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return HostPaths{}, fmt.Errorf("cannot find backend root from %s", start)
		}
		dir = parent
	}
}

// PackOptions controls Pack.
type PackOptions struct {
	Host       HostPaths
	PluginID   string
	OutputZip  string // empty → <cwd>/plugin-<id>-v<ver>.zip
	Manifest   *Manifest
}

// Pack builds a delivery zip from an assembled plugin already living in the
// host tree (backend/plugins/<id> + frontend pages/locales).
func Pack(opts PackOptions) (string, error) {
	id := strings.TrimSpace(opts.PluginID)
	if id == "" {
		return "", fmt.Errorf("pack: plugin id is required")
	}
	pluginDir := filepath.Join(opts.Host.BackendRoot, "plugins", id)
	if _, err := os.Stat(pluginDir); err != nil {
		return "", fmt.Errorf("pack: plugin directory: %w", err)
	}

	manifest, err := resolvePackManifest(pluginDir, opts.Manifest)
	if err != nil {
		return "", err
	}
	if manifest.ID != id {
		return "", fmt.Errorf("pack: manifest id %q does not match -id %q", manifest.ID, id)
	}
	if err := manifest.Validate(); err != nil {
		return "", err
	}

	staging, err := os.MkdirTemp("", "dragz-plugin-pack-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)

	rootName := manifest.PackageDirName()
	pkgRoot := filepath.Join(staging, rootName)
	if err := os.MkdirAll(pkgRoot, 0o755); err != nil {
		return "", err
	}

	if err := WriteManifest(filepath.Join(pkgRoot, ManifestFileName), manifest); err != nil {
		return "", err
	}
	if err := copyDir(pluginDir, filepath.Join(pkgRoot, BackendDirName)); err != nil {
		return "", fmt.Errorf("pack backend: %w", err)
	}
	// Prefer a clean manifest at package root; drop a nested copy if present.
	_ = os.Remove(filepath.Join(pkgRoot, BackendDirName, ManifestFileName))

	for _, component := range manifest.Components {
		pageRel, err := componentToPageDir(component)
		if err != nil {
			return "", err
		}
		src := filepath.Join(opts.Host.RepoRoot, "frontend", "admin", "src", "pages", filepath.FromSlash(pageRel))
		dst := filepath.Join(pkgRoot, FrontendPagesDir, filepath.FromSlash(pageRel))
		if _, err := os.Stat(src); err != nil {
			return "", fmt.Errorf("pack: frontend page for component %q: %w", component, err)
		}
		if err := copyDir(src, dst); err != nil {
			return "", fmt.Errorf("pack frontend pages: %w", err)
		}
	}

	localeDir := filepath.Join(pkgRoot, FrontendLocalesDir)
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		return "", err
	}
	for _, loc := range []string{"en.json", "zh.json"} {
		hostLoc := filepath.Join(opts.Host.RepoRoot, "frontend", "admin", "src", "locales", loc)
		fragment, err := extractPageLocale(hostLoc, id)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(localeDir, loc), fragment, 0o644); err != nil {
			return "", err
		}
	}

	out := strings.TrimSpace(opts.OutputZip)
	if out == "" {
		out = filepath.Join(".", rootName+".zip")
	}
	if err := zipDir(pkgRoot, out, rootName); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(out)
	if err != nil {
		return out, nil
	}
	return abs, nil
}

func resolvePackManifest(pluginDir string, override *Manifest) (Manifest, error) {
	if override != nil {
		return *override, nil
	}
	path := filepath.Join(pluginDir, ManifestFileName)
	return LoadManifest(path)
}

func zipDir(srcDir, zipPath, rootName string) error {
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil && !os.IsExist(err) {
		// Dir may be "."
		_ = err
	}
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()

	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(rootName, rel))
		if d.IsDir() {
			if rel == "." {
				return nil
			}
			_, err := zw.Create(name + "/")
			return err
		}
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	})
}
