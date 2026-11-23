package pluginapi

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// AssembleOptions controls Assemble.
type AssembleOptions struct {
	Host      HostPaths
	ZipPath   string
	Force     bool // overwrite existing plugins/<id> and frontend pages
	SkipPatch bool // for tests: copy files only
}

// AssembleResult summarizes what was written into the host tree.
type AssembleResult struct {
	Manifest  Manifest
	PluginDir string
}

// Assemble unpacks a delivery zip into the host monorepo: backend/plugins/<id>,
// plugins/all registration, frontend pages, page-map, and locale fragments.
func Assemble(opts AssembleOptions) (AssembleResult, error) {
	if strings.TrimSpace(opts.ZipPath) == "" {
		return AssembleResult{}, fmt.Errorf("assemble: zip path is required")
	}
	staging, err := os.MkdirTemp("", "dragz-plugin-assemble-*")
	if err != nil {
		return AssembleResult{}, err
	}
	defer os.RemoveAll(staging)

	pkgRoot, err := unzipPlugin(opts.ZipPath, staging)
	if err != nil {
		return AssembleResult{}, err
	}
	manifest, err := LoadManifest(filepath.Join(pkgRoot, ManifestFileName))
	if err != nil {
		return AssembleResult{}, err
	}

	pluginDir := filepath.Join(opts.Host.BackendRoot, "plugins", manifest.ID)
	if _, err := os.Stat(pluginDir); err == nil && !opts.Force {
		return AssembleResult{}, fmt.Errorf("assemble: plugins/%s already exists (pass -force to overwrite)", manifest.ID)
	} else if err == nil && opts.Force {
		if err := os.RemoveAll(pluginDir); err != nil {
			return AssembleResult{}, err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return AssembleResult{}, err
	}

	backendSrc := filepath.Join(pkgRoot, BackendDirName)
	if _, err := os.Stat(backendSrc); err != nil {
		return AssembleResult{}, fmt.Errorf("assemble: package missing %s/: %w", BackendDirName, err)
	}
	if err := copyDir(backendSrc, pluginDir); err != nil {
		return AssembleResult{}, fmt.Errorf("assemble backend: %w", err)
	}
	if err := WriteManifest(filepath.Join(pluginDir, ManifestFileName), manifest); err != nil {
		return AssembleResult{}, err
	}

	for _, component := range manifest.Components {
		pageRel, err := componentToPageDir(component)
		if err != nil {
			return AssembleResult{}, err
		}
		src := filepath.Join(pkgRoot, FrontendPagesDir, filepath.FromSlash(pageRel))
		dst := filepath.Join(opts.Host.RepoRoot, "frontend", "admin", "src", "pages", filepath.FromSlash(pageRel))
		if _, err := os.Stat(src); err != nil {
			return AssembleResult{}, fmt.Errorf("assemble: package missing frontend page for %q: %w", component, err)
		}
		if _, err := os.Stat(dst); err == nil && !opts.Force {
			return AssembleResult{}, fmt.Errorf("assemble: frontend page %s already exists (pass -force)", pageRel)
		}
		if err := os.RemoveAll(dst); err != nil && !os.IsNotExist(err) {
			return AssembleResult{}, err
		}
		if err := copyDir(src, dst); err != nil {
			return AssembleResult{}, fmt.Errorf("assemble frontend: %w", err)
		}
	}

	for _, loc := range []string{"en.json", "zh.json"} {
		fragPath := filepath.Join(pkgRoot, FrontendLocalesDir, loc)
		fragment, err := os.ReadFile(fragPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return AssembleResult{}, err
		}
		hostLoc := filepath.Join(opts.Host.RepoRoot, "frontend", "admin", "src", "locales", loc)
		if err := mergePageLocale(hostLoc, fragment, manifest.ID); err != nil {
			return AssembleResult{}, fmt.Errorf("assemble locales %s: %w", loc, err)
		}
	}

	if !opts.SkipPatch {
		if err := PatchAllGo(filepath.Join(opts.Host.BackendRoot, "plugins", "all", "all.go"), manifest.ID); err != nil {
			return AssembleResult{}, err
		}
		exportName := exportNameFromID(manifest.ID)
		for _, component := range manifest.Components {
			pageRel, err := componentToPageDir(component)
			if err != nil {
				return AssembleResult{}, err
			}
			if err := PatchPageMap(
				filepath.Join(opts.Host.RepoRoot, "frontend", "admin", "src", "router", "page-map.tsx"),
				filepath.Join(opts.Host.RepoRoot, "frontend", "admin", "src", "router", "page-map.test.tsx"),
				component,
				pageRel,
				exportName,
			); err != nil {
				return AssembleResult{}, err
			}
		}
	}

	return AssembleResult{Manifest: manifest, PluginDir: pluginDir}, nil
}

func unzipPlugin(zipPath, staging string) (pkgRoot string, err error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var top string
	for _, f := range r.File {
		name := filepath.Clean(filepath.FromSlash(f.Name))
		if name == "." || strings.HasPrefix(name, "..") {
			return "", fmt.Errorf("assemble: unsafe zip entry %q", f.Name)
		}
		parts := strings.Split(name, string(os.PathSeparator))
		if len(parts) == 0 {
			continue
		}
		if top == "" {
			top = parts[0]
		} else if parts[0] != top {
			return "", fmt.Errorf("assemble: zip must contain a single top-level directory")
		}
		target := filepath.Join(staging, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		if err := writeZipFile(f, target); err != nil {
			return "", err
		}
	}
	if top == "" {
		return "", fmt.Errorf("assemble: empty zip")
	}
	root := filepath.Join(staging, top)
	if _, err := os.Stat(filepath.Join(root, ManifestFileName)); err != nil {
		return "", fmt.Errorf("assemble: %s missing under %s", ManifestFileName, top)
	}
	return root, nil
}

func writeZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

// PatchAllGo registers plugins/<id> in plugins/all/all.go (idempotent).
func PatchAllGo(path, id string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	src := strings.ReplaceAll(string(data), "\r\n", "\n")
	if strings.Contains(src, id+".Plugin{}") && strings.Contains(src, "plugins/"+id+"\"") {
		return nil
	}
	importLine := fmt.Sprintf("\t\"dragz-admin/backend/plugins/%s\"\n", id)
	if !strings.Contains(src, "plugins/"+id+"\"") {
		importBlock := regexp.MustCompile(`(?s)(import \(\n)(.*?)(\n\))`)
		m := importBlock.FindStringSubmatch(src)
		if m == nil {
			return fmt.Errorf("cannot find import block in %s", path)
		}
		imports := m[2]
		if !strings.HasSuffix(imports, "\n") {
			imports += "\n"
		}
		imports += importLine
		src = importBlock.ReplaceAllString(src, m[1]+imports+m[3])
	}
	marker := "\t\t// scaffold:insert"
	if !strings.Contains(src, marker) {
		return fmt.Errorf("%s: missing // scaffold:insert marker", path)
	}
	if !strings.Contains(src, id+".Plugin{}") {
		insert := fmt.Sprintf("\t\t%s.Plugin{},\n%s", id, marker)
		src = strings.Replace(src, marker, insert, 1)
	}
	return os.WriteFile(path, []byte(src), 0o644)
}

// PatchPageMap inserts a lazy page-map entry and updates the component test list.
func PatchPageMap(pageMapPath, testPath, component, pageImportRel, exportName string) error {
	pm, err := os.ReadFile(pageMapPath)
	if err != nil {
		return err
	}
	if bytes.Contains(pm, []byte("'"+component+"'")) {
		// already mapped
	} else {
		entry := fmt.Sprintf("  '%s': named(() => import('@/pages/%s'), '%s'),\n", component, filepath.ToSlash(pageImportRel), exportName)
		const anchor = "  '/profile/index':"
		if !bytes.Contains(pm, []byte(anchor)) {
			return fmt.Errorf("page-map.tsx: cannot find insertion anchor %q", anchor)
		}
		pm = bytes.Replace(pm, []byte(anchor), []byte(entry+anchor), 1)
		if err := os.WriteFile(pageMapPath, pm, 0o644); err != nil {
			return err
		}
	}

	if testPath == "" {
		return nil
	}
	testData, err := os.ReadFile(testPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	s := string(testData)
	if strings.Contains(s, "'"+component+"'") {
		return nil
	}
	const needle = "const backendComponents=["
	i := strings.Index(s, needle)
	if i < 0 {
		return nil
	}
	j := strings.Index(s[i:], "];")
	if j < 0 {
		return nil
	}
	closeAt := i + j
	s = s[:closeAt] + ",'" + component + "'" + s[closeAt:]
	return os.WriteFile(testPath, []byte(s), 0o644)
}

func exportNameFromID(id string) string {
	var b strings.Builder
	upper := true
	for _, r := range id {
		if r == '_' || r == '-' {
			upper = true
			continue
		}
		if upper {
			if r >= 'a' && r <= 'z' {
				b.WriteByte(byte(r - 'a' + 'A'))
			} else {
				b.WriteRune(r)
			}
			upper = false
		} else {
			b.WriteRune(r)
		}
	}
	b.WriteString("Page")
	return b.String()
}
