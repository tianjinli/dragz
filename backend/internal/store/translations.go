package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// translationPattern turns a user keyword into an ILIKE pattern; an empty
// keyword becomes "" which the queries treat as "no filter".
func translationPattern(keyword string) string {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return ""
	}
	return "%" + keyword + "%"
}

// The entity_key namespaces of sys_translation, each one produced by a branch
// of v_preset_entity:
//   menu.<menu code>          page/directory rows of sys_menu
//   perm.<permission code>    button rows of sys_menu (written by the sync)
//   role.<role code>[.remark] sys_role
//   dept.<department code>    sys_department
//   post.<position code>      sys_position (its remark column too)
//   job.<job code>            sys_scheduled_job
//   storage.<storage type>    sys_storage_config

// ErrUnknownEntityKey is returned for an entity_key in a namespace that has no
// source column to write to.
var ErrUnknownEntityKey = errors.New("unknown translation entity")

// ErrSourceEntityNotFound is returned when the row an entity_key addresses no
// longer exists — it was deleted, or its code changed under us.
var ErrSourceEntityNotFound = errors.New("translation source entity not found")

// splitTranslationKey breaks "role.admin.remark" into the namespace, the code
// the owning table is addressed by, and whether the target is the remark
// column instead of the name. Namespaces all end in a dot, so the first one
// that matches is the only one that can.
func splitTranslationKey(entityKey string) (namespace, code string, remark bool, ok bool) {
	index := strings.Index(entityKey, ".")
	if index <= 0 || index == len(entityKey)-1 {
		return "", "", false, false
	}
	namespace, code = entityKey[:index], entityKey[index+1:]
	if strings.HasSuffix(code, ".remark") {
		code = strings.TrimSuffix(code, ".remark")
		remark = true
	}
	if code == "" {
		return "", "", false, false
	}
	return namespace, code, remark, true
}

// writeSourceText updates the source-language column one entity_key addresses
// and reports how many rows it touched. Adding a translatable namespace means
// a UNION in v_preset_entity plus one case here.
func (r *AuthRepository) writeSourceText(ctx context.Context, namespace, code, value string, remark bool) (int64, error) {
	switch namespace {
	case "menu", "perm":
		return r.queries.SetMenuSourceText(ctx, sqlcstore.SetMenuSourceTextParams{Value: value, Code: code})
	case "role":
		if remark {
			return r.queries.SetRoleSourceRemark(ctx, sqlcstore.SetRoleSourceRemarkParams{Value: value, Code: code})
		}
		return r.queries.SetRoleSourceText(ctx, sqlcstore.SetRoleSourceTextParams{Value: value, Code: code})
	case "dept":
		return r.queries.SetDepartmentSourceText(ctx, sqlcstore.SetDepartmentSourceTextParams{Value: value, Code: code})
	case "post":
		if remark {
			return r.queries.SetPositionSourceRemark(ctx, sqlcstore.SetPositionSourceRemarkParams{Value: value, Code: code})
		}
		return r.queries.SetPositionSourceText(ctx, sqlcstore.SetPositionSourceTextParams{Value: value, Code: code})
	case "job":
		return r.queries.SetJobSourceText(ctx, sqlcstore.SetJobSourceTextParams{Value: value, Code: code})
	case "storage":
		return r.queries.SetStorageSourceText(ctx, sqlcstore.SetStorageSourceTextParams{Value: value, Code: code})
	}
	return 0, ErrUnknownEntityKey
}

// SetSourceText writes the source-language text of one preset entity back into
// the table that owns it, which is how the translation page edits Chinese:
// the owning column is the source, so sys_translation never holds a zh row.
func (r *AuthRepository) SetSourceText(ctx context.Context, entityKey, value string) error {
	namespace, code, remark, ok := splitTranslationKey(entityKey)
	if !ok {
		return ErrUnknownEntityKey
	}
	affected, err := r.writeSourceText(ctx, namespace, code, value, remark)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrSourceEntityNotFound
	}
	return nil
}

// PresetSourceRow is what the owning table holds for one preset entity: the
// code its entity_keys are built from and the source-language text of its
// translatable columns.
type PresetSourceRow struct {
	Code   string
	Name   string
	Remark string
}

// PresetSourceRow reads the row a preset save is about to rewrite. namespace is
// the owning table (menu covers permission buttons too); an id that no longer
// exists reports ErrSourceEntityNotFound.
func (r *AuthRepository) PresetSourceRow(ctx context.Context, namespace string, id uint64) (PresetSourceRow, error) {
	row, err := r.queries.GetPresetSourceRow(ctx, sqlcstore.GetPresetSourceRowParams{
		Namespace: namespace,
		ID:        int64(id),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return PresetSourceRow{}, ErrSourceEntityNotFound
	}
	if err != nil {
		return PresetSourceRow{}, err
	}
	return PresetSourceRow{Code: row.Code, Name: row.Source, Remark: row.Remark}, nil
}

// RekeyEntityTranslations carries the translation records of one entity over
// when the code its entity_key is built from changes. Keys whose destination
// is already taken are left behind rather than colliding.
func (r *AuthRepository) RekeyEntityTranslations(ctx context.Context, oldKey, newKey string) error {
	if oldKey == newKey {
		return nil
	}
	_, err := r.queries.RekeyTranslations(ctx, sqlcstore.RekeyTranslationsParams{OldKey: oldKey, NewKey: newKey})
	return err
}

// TranslationsByKey loads preset-data translations for one language, keyed by
// entity_key. Chinese is the default language and lives in the owning tables'
// text columns, so callers only query for non-default languages.
func (r *AuthRepository) TranslationsByKey(ctx context.Context, lang string, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	rows, err := r.queries.ListTranslationsByKey(ctx, sqlcstore.ListTranslationsByKeyParams{
		Lang:    lang,
		Column2: keys,
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.EntityKey] = row.Value
	}
	return out, nil
}

// TranslationRow is one editable translation record shown on the translation
// management page: the translated value plus the Chinese source text it is
// derived from.
type TranslationRow struct {
	ID        int64  `json:"id"`
	EntityKey string `json:"entityKey"`
	Source    string `json:"source"`
	Value     string `json:"value"`
	Lang      string `json:"lang"`
	UpdatedAt string `json:"updatedAt"`
}

// ListTranslations returns one page of translation records with the Chinese
// source text joined in for display. An empty lang lists every language.
func (r *AuthRepository) ListTranslations(ctx context.Context, lang, keyword string, page, pageSize int) ([]TranslationRow, int64, error) {
	pattern := translationPattern(keyword)
	rows, err := r.queries.ListTranslationRows(ctx, sqlcstore.ListTranslationRowsParams{
		Lang:       lang,
		Keyword:    pattern,
		PageSize:   int32(pageSize),
		PageOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.queries.CountTranslationRows(ctx, sqlcstore.CountTranslationRowsParams{
		Lang:    lang,
		Keyword: pattern,
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]TranslationRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, TranslationRow{
			ID:        row.ID,
			EntityKey: row.EntityKey,
			Source:    row.Source,
			Value:     row.Value,
			Lang:      row.Lang,
			UpdatedAt: row.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return items, total, nil
}

// ListSourceTranslations returns one page of preset entities in the source
// language: each row carries the owning table's text as its value, so the
// translation page lists Chinese records exactly like it lists the translated
// ones. These rows are edited through SetSourceText, not sys_translation, so
// the generated id is a display key only.
func (r *AuthRepository) ListSourceTranslations(ctx context.Context, keyword string, page, pageSize int) ([]TranslationRow, int64, error) {
	rows, err := r.queries.ListSourceEntities(ctx, sqlcstore.ListSourceEntitiesParams{
		Keyword:    translationPattern(keyword),
		PageSize:   int32(pageSize),
		PageOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if len(rows) > 0 {
		total = int64(rows[0].Total)
	}
	items := make([]TranslationRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, TranslationRow{
			ID:        int64(row.ID),
			EntityKey: row.EntityKey,
			Source:    row.Source,
			Value:     row.Value,
			Lang:      row.Lang,
			UpdatedAt: row.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return items, total, nil
}

// ListEntityTranslations is what the translation page lists: one row per preset
// entity, carrying the Chinese text its owning table holds and, next to it, the
// translation in lang. Both columns are editable in one save, so the page never
// depends on the interface language to decide what it is editing.
//
// Rows whose translation is missing keep an empty Value instead of dropping out
// of the list — otherwise a Chinese-only entity could never be given an English
// text from this page. The id stays the display key ListSourceEntities
// generates: an edit is addressed by entity_key, never by id.
func (r *AuthRepository) ListEntityTranslations(ctx context.Context, lang, keyword string, page, pageSize int) ([]TranslationRow, int64, error) {
	rows, err := r.queries.ListSourceEntities(ctx, sqlcstore.ListSourceEntitiesParams{
		Keyword:    translationPattern(keyword),
		PageSize:   int32(pageSize),
		PageOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if len(rows) > 0 {
		total = int64(rows[0].Total)
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.EntityKey)
	}
	values, err := r.TranslationsByKey(ctx, lang, keys)
	if err != nil {
		return nil, 0, err
	}
	items := make([]TranslationRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, TranslationRow{
			ID:        int64(row.ID),
			EntityKey: row.EntityKey,
			Source:    row.Source,
			Value:     values[row.EntityKey],
			Lang:      lang,
			UpdatedAt: row.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return items, total, nil
}

// TranslationCandidate is a preset entity that still lacks a translation in
// the requested language; it is what the "new translation" form offers.
type TranslationCandidate struct {
	EntityKey string `json:"entityKey"`
	Source    string `json:"source"`
}

// UntranslatedEntities lists preset entities without a translation in lang.
func (r *AuthRepository) UntranslatedEntities(ctx context.Context, lang, keyword string) ([]TranslationCandidate, error) {
	rows, err := r.queries.ListUntranslatedEntities(ctx, sqlcstore.ListUntranslatedEntitiesParams{
		Lang:    lang,
		Column2: translationPattern(keyword),
	})
	if err != nil {
		return nil, err
	}
	items := make([]TranslationCandidate, 0, len(rows))
	for _, row := range rows {
		items = append(items, TranslationCandidate{EntityKey: row.EntityKey, Source: row.Source})
	}
	return items, nil
}

// CreateTranslation inserts (or refreshes) the translation of one entity.
func (r *AuthRepository) CreateTranslation(ctx context.Context, entityKey, lang, value string) error {
	return r.queries.CreateTranslation(ctx, sqlcstore.CreateTranslationParams{
		EntityKey: entityKey,
		Lang:      lang,
		Value:     value,
	})
}

// ErrTranslationNotFound is returned when the record addressed by an edit no
// longer exists.
var ErrTranslationNotFound = errors.New("translation not found")

// ErrTranslationConflict is returned when an edit moves a record to a language
// the same entity already has a translation in.
var ErrTranslationConflict = errors.New("translation already exists for this language")

// UpdateTranslation rewrites one translation record, including its language,
// so a record filed under the wrong language is a normal edit. An empty value
// removes the record instead, so the entity falls back to its Chinese source.
func (r *AuthRepository) UpdateTranslation(ctx context.Context, id int64, lang, value string) error {
	if value == "" {
		removed, err := r.queries.DeleteTranslationByID(ctx, id)
		if err != nil {
			return err
		}
		if removed == 0 {
			return ErrTranslationNotFound
		}
		return nil
	}
	current, err := r.queries.GetTranslationByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTranslationNotFound
	}
	if err != nil {
		return err
	}
	if current.Lang != lang {
		// Moving to another language must not silently overwrite the
		// translation that already lives there.
		occupant, err := r.queries.TranslationExists(ctx, sqlcstore.TranslationExistsParams{
			EntityKey: current.EntityKey,
			Lang:      lang,
		})
		switch {
		case err == nil && occupant != id:
			return ErrTranslationConflict
		case err != nil && !errors.Is(err, sql.ErrNoRows):
			return err
		}
	}
	affected, err := r.queries.UpdateTranslation(ctx, sqlcstore.UpdateTranslationParams{
		ID:    id,
		Lang:  lang,
		Value: value,
	})
	if err != nil {
		// A racing insert can still win the unique constraint.
		if errors.Is(wrapDuplicate(err), ErrDuplicateEntry) {
			return ErrTranslationConflict
		}
		return err
	}
	if affected == 0 {
		return ErrTranslationNotFound
	}
	return nil
}

// DeleteTranslationForEntity drops an entity's translation in lang, if it has
// one, and reports whether a record was actually removed. Clearing a text is
// how the entity falls back to its source language, so the save path ignores
// the flag — deleting nothing there is a normal save, not a failure. The delete
// endpoint reads it instead: with rows identified by entity key, a no-op would
// otherwise answer "ok" for a translation that is not there.
func (r *AuthRepository) DeleteTranslationForEntity(ctx context.Context, entityKey, lang string) (bool, error) {
	removed, err := r.queries.DeleteTranslation(ctx, sqlcstore.DeleteTranslationParams{
		EntityKey: entityKey,
		Lang:      lang,
	})
	if err != nil {
		return false, err
	}
	return removed > 0, nil
}
