package pluginapi

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"dragz-admin/backend/internal/plugin"
)

// Manifest is the on-disk contract for an independently delivered plugin
// package (plugin-<id>-v<semver>/manifest.json).
type Manifest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	DependsOn   []string `json:"dependsOn,omitempty"`
	// EngineRange is a space-separated list of constraints against
	// EngineVersion, e.g. ">=1.0.0 <2.0.0". Empty means DefaultEngineRange.
	EngineRange string   `json:"engineRange"`
	Components  []string `json:"components"`
	Builtin     bool     `json:"builtin,omitempty"`
}

// LoadManifest reads and validates a manifest.json file.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// WriteManifest writes m as indented JSON.
func WriteManifest(path string, m Manifest) error {
	if err := m.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// Validate checks id/version/components and that EngineVersion satisfies
// EngineRange.
func (m Manifest) Validate() error {
	if !plugin.ValidID(m.ID) {
		return fmt.Errorf("manifest: invalid id %q (want ^[a-z][a-z0-9]{2,15}$)", m.ID)
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("manifest: name is required")
	}
	if err := validateSemver(m.Version); err != nil {
		return fmt.Errorf("manifest: version: %w", err)
	}
	rangeExpr := strings.TrimSpace(m.EngineRange)
	if rangeExpr == "" {
		rangeExpr = DefaultEngineRange
	}
	ok, err := SatisfiesEngineRange(EngineVersion, rangeExpr)
	if err != nil {
		return fmt.Errorf("manifest: engineRange: %w", err)
	}
	if !ok {
		return fmt.Errorf("manifest: host engine %s does not satisfy engineRange %q", EngineVersion, rangeExpr)
	}
	if len(m.Components) == 0 {
		return fmt.Errorf("manifest: components must list at least one admin page path")
	}
	for _, c := range m.Components {
		if !strings.HasPrefix(c, "/") || strings.Contains(c, "..") {
			return fmt.Errorf("manifest: invalid component %q (want absolute path like /demo/index)", c)
		}
	}
	for _, dep := range m.DependsOn {
		if !plugin.ValidID(dep) {
			return fmt.Errorf("manifest: invalid dependsOn id %q", dep)
		}
	}
	if m.Builtin {
		return fmt.Errorf("manifest: builtin plugins are not delivered as packages")
	}
	return nil
}

// PackageDirName returns the conventional top-level folder name inside a zip.
func (m Manifest) PackageDirName() string {
	return fmt.Sprintf("plugin-%s-v%s", m.ID, m.Version)
}
