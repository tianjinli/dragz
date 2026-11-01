// Package kernel assembles plugins into a running application: it validates
// metadata, orders plugins by dependency, aggregates route declarations and
// registers them onto a go-zero rest server. The kernel knows NOTHING about
// individual plugins.
package kernel

import (
	"fmt"
	"sort"
	"strings"

	"dragz-admin/backend/internal/plugin"
)

// Bound pairs an aggregated route with the plugin that declared it.
type Bound struct {
	PluginID string
	Route    plugin.Route
}

// Registry is the assembled, validated view of all plugins.
type Registry struct {
	ordered []plugin.Plugin
	bounds  []Bound
}

// Plugins returns the plugins in dependency order.
func (r *Registry) Plugins() []plugin.Plugin { return r.ordered }

// Routes returns all routes in plugin order (declaration order preserved
// within each plugin).
func (r *Registry) Routes() []Bound { return r.bounds }

// Metas returns every plugin's metadata in dependency order. It is a pure
// projection: no database access, so cmd/gen-api and the golden test keep
// working without a live stack.
func (r *Registry) Metas() []plugin.Meta {
	metas := make([]plugin.Meta, 0, len(r.ordered))
	for _, p := range r.ordered {
		metas = append(metas, p.Meta())
	}
	return metas
}

// MigrationSources collects the migrations of every plugin that implements
// plugin.Migrator, in dependency order. Plugins without migrations are
// omitted rather than reported as empty sources.
func (r *Registry) MigrationSources() []plugin.MigrationSource {
	sources := make([]plugin.MigrationSource, 0, len(r.ordered))
	for _, p := range r.ordered {
		migrator, ok := p.(plugin.Migrator)
		if !ok {
			continue
		}
		sources = append(sources, plugin.MigrationSource{
			PluginID:   p.Meta().ID,
			Migrations: migrator.Migrations(),
		})
	}
	return sources
}

// Build validates and assembles plugins into a Registry.
func Build(plugins []plugin.Plugin) (*Registry, error) {
	byID := make(map[string]plugin.Plugin, len(plugins))
	ids := make([]string, 0, len(plugins))
	for _, p := range plugins {
		meta := p.Meta()
		if !plugin.ValidID(meta.ID) {
			return nil, fmt.Errorf("plugin %q: ID must match ^[a-z][a-z0-9]{2,15}$", meta.ID)
		}
		if _, dup := byID[meta.ID]; dup {
			return nil, fmt.Errorf("duplicate plugin ID %q", meta.ID)
		}
		byID[meta.ID] = p
		ids = append(ids, meta.ID)
	}

	// Validate dependencies.
	for _, id := range ids {
		for _, dep := range byID[id].Meta().DependsOn {
			if _, ok := byID[dep]; !ok {
				return nil, fmt.Errorf("plugin %q depends on unknown plugin %q", id, dep)
			}
		}
	}

	// Kahn topological sort; among ready nodes pick (Order, ID) smallest so
	// the result is deterministic.
	remaining := make(map[string]bool, len(ids))
	for _, id := range ids {
		remaining[id] = true
	}
	ordered := make([]plugin.Plugin, 0, len(ids))
	for len(remaining) > 0 {
		var candidates []string
		for id := range remaining {
			ready := true
			for _, dep := range byID[id].Meta().DependsOn {
				if remaining[dep] {
					ready = false
					break
				}
			}
			if ready {
				candidates = append(candidates, id)
			}
		}
		if len(candidates) == 0 {
			return nil, fmt.Errorf("dependency cycle among plugins: %s", strings.Join(sortedKeys(remaining), ", "))
		}
		sort.Slice(candidates, func(i, j int) bool {
			mi, mj := byID[candidates[i]].Meta(), byID[candidates[j]].Meta()
			if mi.Order != mj.Order {
				return mi.Order < mj.Order
			}
			return mi.ID < mj.ID
		})
		ordered = append(ordered, byID[candidates[0]])
		delete(remaining, candidates[0])
	}

	// Aggregate routes; reject duplicate METHOD+PATH.
	registry := &Registry{ordered: ordered}
	seen := make(map[string]string)
	for _, p := range ordered {
		meta := p.Meta()
		for _, route := range p.Routes() {
			key := route.Method + " " + route.Path
			if owner, dup := seen[key]; dup {
				return nil, fmt.Errorf("plugin %q: duplicate route %s (already declared by %q)", meta.ID, key, owner)
			}
			seen[key] = meta.ID
			registry.bounds = append(registry.bounds, Bound{PluginID: meta.ID, Route: route})
		}
	}

	// A permission code becomes a single sys_menu button row keyed by the
	// code, so two plugins must never declare the same one: the startup sync
	// would otherwise silently hand one plugin's grants to the other.
	if err := validatePermissionOwnership(registry.bounds); err != nil {
		return nil, err
	}
	return registry, nil
}

// validatePermissionOwnership rejects a permission code declared by more than
// one plugin. Within one plugin a code legitimately guards several routes
// (e.g. system:user:query on both list and detail).
func validatePermissionOwnership(bounds []Bound) error {
	owner := make(map[string]string)
	for _, bound := range bounds {
		codes := make([]string, 0, len(bound.Route.AnyOf)+1)
		if bound.Route.Permission != "" {
			codes = append(codes, bound.Route.Permission)
		}
		codes = append(codes, bound.Route.AnyOf...)
		for _, code := range codes {
			if previous, dup := owner[code]; dup && previous != bound.PluginID {
				return fmt.Errorf(
					"permission code %q declared by both plugin %q and plugin %q",
					code, previous, bound.PluginID,
				)
			}
			owner[code] = bound.PluginID
		}
	}
	return nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
