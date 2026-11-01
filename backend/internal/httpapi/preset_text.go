package httpapi

import (
	"context"
	"errors"
	"net/http"

	pkgi18n "dragz-admin/backend/internal/pkg/i18n"
	"dragz-admin/backend/internal/store"
)

// Preset entities (menus, permission buttons, roles, departments, positions,
// jobs, storage configs) keep their source-language text in their own table and
// every other language in sys_translation. localizePreset is the read side of
// that split; this is the write side, and it implements the rule the operator
// sees: what you type lands in the field you are looking at.
//
// So a save made in Chinese rewrites the owning columns as it always has, while
// a save made in another language leaves the Chinese alone and files what was
// typed as that entity's translation. A create is the deliberate exception: the
// new entity has no other-language text yet, so the typed value also stays in
// its column, which keeps the entity readable in both languages.

// presetTextStore is the store surface a preset save needs. AuthRepository
// implements it; the interface keeps the helper testable.
type presetTextStore interface {
	PresetSourceRow(ctx context.Context, namespace string, id uint64) (store.PresetSourceRow, error)
	RekeyEntityTranslations(ctx context.Context, oldKey, newKey string) error
	CreateTranslation(ctx context.Context, entityKey, lang, value string) error
	// DeleteTranslationForEntity reports whether a record was removed. A save
	// ignores the flag: clearing a language that has no text is a normal save.
	DeleteTranslationForEntity(ctx context.Context, entityKey, lang string) (bool, error)
}

// presetTextEdit describes the translatable texts of one preset save.
type presetTextEdit struct {
	// Namespace names the owning table, as v_preset_source_row does.
	Namespace string
	// Prefix is the entity_key namespace: menu and perm share sys_menu.
	Prefix string
	// ID is the row being rewritten; 0 means this save creates it.
	ID uint64
	// Code is the code the save writes, Name and Remark what was typed.
	Code   string
	Name   string
	Remark string
	// HasRemark marks an entity whose remark column is translatable too.
	HasRemark bool
}

// presetSave is a prepared save: what the owning columns must hold, and what to
// do with sys_translation once the row is written.
type presetSave struct {
	auth     presetTextStore
	edit     presetTextEdit
	lang     string
	previous store.PresetSourceRow
	// known is false for a create and for a row that is already gone, which
	// both mean nothing to preserve and nothing to refile.
	known bool
}

// preparePresetSave reads the row a save is about to rewrite. It runs before the
// store call, because a save made in another language keeps the text that is
// already in the columns.
func preparePresetSave(auth presetTextStore, r *http.Request, edit presetTextEdit) (*presetSave, error) {
	save := &presetSave{auth: auth, edit: edit, lang: pkgi18n.Lang(r)}
	if edit.ID == 0 {
		return save, nil
	}
	previous, err := auth.PresetSourceRow(r.Context(), edit.Namespace, edit.ID)
	if errors.Is(err, store.ErrSourceEntityNotFound) {
		// The row is gone; the save itself reports that.
		return save, nil
	}
	if err != nil {
		return nil, err
	}
	save.previous, save.known = previous, true
	return save, nil
}

// SourceName is the text the entity's name column must hold.
func (s *presetSave) SourceName() string {
	return s.sourceText(s.edit.Name, s.previous.Name)
}

// SourceRemark is the text the entity's remark column must hold. Only entities
// whose edit set HasRemark have one.
func (s *presetSave) SourceRemark() string {
	return s.sourceText(s.edit.Remark, s.previous.Remark)
}

func (s *presetSave) sourceText(typed, stored string) string {
	if s.lang == pkgi18n.SourceLanguage || !s.known {
		return typed
	}
	return stored
}

// keys names the entity_keys of the save's texts under one code, in the same
// order as their values.
func (s *presetSave) keys(code string) []string {
	prefix := s.edit.Prefix + "." + code
	if !s.edit.HasRemark {
		return []string{prefix}
	}
	return []string{prefix, prefix + ".remark"}
}

// typed lists the texts the operator submitted, name first and remark after.
func (s *presetSave) typed() []string {
	if !s.edit.HasRemark {
		return []string{s.edit.Name}
	}
	return []string{s.edit.Name, s.edit.Remark}
}

// Apply aligns sys_translation with the save. Call it after the row is written:
// the source-language save already did everything through the owning columns.
func (s *presetSave) Apply(ctx context.Context) error {
	keys := s.keys(s.edit.Code)
	if s.known && s.previous.Code != "" && s.previous.Code != s.edit.Code {
		// Translations belong to the entity, not to the code they were filed
		// under, so a code edit carries them along.
		previous := s.keys(s.previous.Code)
		for i, key := range previous {
			if err := s.auth.RekeyEntityTranslations(ctx, key, keys[i]); err != nil {
				return err
			}
		}
	}
	if s.lang == pkgi18n.SourceLanguage {
		return nil
	}
	values := s.typed()
	for i, key := range keys {
		if values[i] == "" {
			// An empty text drops the record, so the entity falls back to its
			// source language instead of showing blanks.
			if _, err := s.auth.DeleteTranslationForEntity(ctx, key, s.lang); err != nil {
				return err
			}
			continue
		}
		if err := s.auth.CreateTranslation(ctx, key, s.lang, values[i]); err != nil {
			return err
		}
	}
	return nil
}
