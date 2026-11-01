package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"dragz-admin/backend/internal/pkg/ctxkey"
	pkgi18n "dragz-admin/backend/internal/pkg/i18n"
	"dragz-admin/backend/internal/store"
)

// translationLoader is the store surface the preset-data translator needs.
// AuthRepository implements it; the interface keeps the helper testable.
type translationLoader interface {
	TranslationsByKey(ctx context.Context, lang string, keys []string) (map[string]string, error)
}

// localizePreset rewrites preset rows' display fields from sys_translation in
// the request language. Chinese is the default language stored inline in the
// owning tables, so it short-circuits to a no-op; missing translations fall
// back to the stored text by simply not being applied.
//
// keysOf returns every entity_key an item may have (e.g. a role contributes
// both "role.<code>" and "role.<code>.remark"); apply writes the looked-up
// values back onto the item.
func localizePreset[T any](r *http.Request, loader translationLoader, items []T, keysOf func(T) []string, apply func(item *T, values map[string]string)) []T {
	lang := pkgi18n.Lang(r)
	if lang == pkgi18n.SourceLanguage || len(items) == 0 {
		return items
	}
	seen := make(map[string]bool, len(items))
	keys := make([]string, 0, len(items))
	for i := range items {
		for _, k := range keysOf(items[i]) {
			if k != "" && !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	if len(keys) == 0 {
		return items
	}
	values, err := loader.TranslationsByKey(r.Context(), lang, keys)
	if err != nil || len(values) == 0 {
		return items
	}
	for i := range items {
		apply(&items[i], values)
	}
	return items
}

// localizeRef renders one preset reference (a department, a role, ...) in the
// request language, keyed by prefix+code. An empty ref — a user with no
// department — passes through untouched.
func localizeRef(r *http.Request, loader translationLoader, prefix string, ref store.NamedRef) store.NamedRef {
	if ref.Code == "" {
		return ref
	}
	return localizePreset(r, loader, []store.NamedRef{ref},
		func(item store.NamedRef) []string { return []string{prefix + item.Code} },
		func(item *store.NamedRef, values map[string]string) {
			if s, ok := values[prefix+item.Code]; ok {
				item.Name = s
			}
		})[0]
}

// localizeText renders one stored preset text in the request language.
// Preset rows (seeded menus, roles, departments, configs, jobs, storage)
// store a message ID or an i18n message envelope in their text columns, so
// every reader gets their own language; admin-entered plain text is not a
// message ID and passes through unchanged.
func localizeText(r *http.Request, raw string) string {
	if raw == "" {
		return raw
	}
	translator, _ := r.Context().Value(ctxkey.Translator).(*pkgi18n.Translator)
	if translator == nil {
		return raw
	}
	return translator.LocalizeStored(r, raw)
}

// localizeList localises the given string fields of every object in items,
// including nested maps and slices. It round-trips through JSON because the
// list endpoints return heterogeneous store structs and the walk is uniform.
// Sizes are admin-list scale, so the extra allocation is negligible.
func localizeList(r *http.Request, items any, fields ...string) any {
	wanted := make(map[string]bool, len(fields))
	for _, field := range fields {
		wanted[field] = true
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return items
	}
	var doc any
	if json.Unmarshal(raw, &doc) != nil {
		return items
	}
	var walk func(node any)
	walk = func(node any) {
		switch value := node.(type) {
		case map[string]any:
			for key, item := range value {
				if wanted[key] {
					if text, isText := item.(string); isText {
						value[key] = localizeText(r, text)
						continue
					}
				}
				walk(item)
			}
		case []any:
			for _, item := range value {
				walk(item)
			}
		}
	}
	walk(doc)
	return doc
}

// localizePluginMeta localises the plugin center's name/description. Plugin
// metadata is code-defined (not database rows), so the text columns hold the
// seed-language fallback and the request-language label is looked up by the
// stable plugin ID: "plugin.<id>.name" / "plugin.<id>.desc".
func localizePluginMeta(r *http.Request, pluginID, raw string, name bool) string {
	translator, _ := r.Context().Value(ctxkey.Translator).(*pkgi18n.Translator)
	if translator == nil {
		return raw
	}
	key := "plugin." + pluginID + ".desc"
	if name {
		key = "plugin." + pluginID + ".name"
	}
	if !translator.Has(key) {
		return raw
	}
	return translator.T(r, key)
}
