package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"dragz-admin/backend/internal/menuperm"
	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// MenuCodeIndex maps sys_menu.code to id for every live non-button menu. It is
// the lookup the button sync uses to attach a permission to its page.
func (r *PluginRepository) MenuCodeIndex(ctx context.Context) (map[string]uint64, error) {
	rows, err := r.queries.ListMenuCodeIDs(ctx)
	if err != nil {
		return nil, err
	}
	index := make(map[string]uint64, len(rows))
	for _, row := range rows {
		index[row.Code] = uint64(row.ID)
	}
	return index, nil
}

// SyncPermissionMenus makes sys_menu buttons match the compiled-in route table.
//
// It runs inside one transaction: every declared code is upserted, buttons whose
// route disappeared are soft deleted, and the built-in super administrator role
// is re-granted everything. A partial run is impossible, which matters because
// the previous state is the only record of what an administrator granted.
//
// The sync is additive for grants: soft-deleting a button leaves its
// sys_role_menu rows untouched, so re-adding a route restores its grants.
func (r *PluginRepository) SyncPermissionMenus(ctx context.Context, buttons []menuperm.Button) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := syncPermissionMenus(ctx, tx, buttons); err != nil {
		return err
	}
	return tx.Commit()
}

// syncPermissionMenus is the body of SyncPermissionMenus, bound to a caller
// supplied transaction so a test can run the whole sync against a scratch
// schema and roll it back.
func syncPermissionMenus(ctx context.Context, tx *Tx, buttons []menuperm.Button) error {
	q := sqlcstore.New(tx)

	existing, err := q.ListButtonMenuCodes(ctx)
	if err != nil {
		return fmt.Errorf("list existing buttons: %w", err)
	}
	live := make(map[string]bool, len(existing))
	for _, code := range existing {
		live[code] = true
	}

	for _, button := range buttons {
		// Revive first: an existing row (live, or retired by an earlier sync)
		// keeps its id, and therefore its grants. Only a code the sync has
		// never seen needs an INSERT.
		_, err := q.RevivePermissionButton(ctx, sqlcstore.RevivePermissionButtonParams{
			ParentID:    button.ParentID,
			Method:      button.Method,
			Path:        button.Path,
			IsSensitive: boolSmallInt16(button.Sensitive),
			PluginID:    button.PluginID,
			Code:        button.Code,
		})
		if errors.Is(err, sql.ErrNoRows) {
			err = q.InsertPermissionButton(ctx, sqlcstore.InsertPermissionButtonParams{
				ParentID:    button.ParentID,
				Name:        button.Name,
				Code:        button.Code,
				Method:      button.Method,
				Path:        button.Path,
				IsSensitive: boolSmallInt16(button.Sensitive),
				Sort:        int32(button.Sort),
				PluginID:    button.PluginID,
			})
			if err != nil {
				// uk_menu_code is unique among live rows, so a duplicate here
				// means a page or directory already owns the code. Never take
				// that row over.
				if errors.Is(wrapDuplicate(err), ErrDuplicateEntry) {
					return fmt.Errorf("%w: %s", ErrMenuCodeTaken, button.Code)
				}
				return fmt.Errorf("sync permission %q: %w", button.Code, err)
			}
		} else if err != nil {
			return fmt.Errorf("sync permission %q: %w", button.Code, err)
		}
		// Seed the English label into sys_translation. Absent-on-conflict:
		// once an administrator edits the translation (or a previous run
		// wrote it) the compiled-in default never overwrites it. Skipped for
		// codes without a derived English label (raw-code fallbacks and
		// MenuName overrides, where the plugin seeds its own).
		if en := button.NameEn; en != "" && en != button.Name && en != button.Code {
			if err := q.UpsertTranslationIfAbsent(ctx, sqlcstore.UpsertTranslationIfAbsentParams{
				EntityKey: "perm." + button.Code,
				Lang:      "en",
				Value:     en,
			}); err != nil {
				return fmt.Errorf("seed translation %q: %w", button.Code, err)
			}
		}
		delete(live, button.Code)
	}

	for code := range live {
		if err := q.SoftDeleteStaleButton(ctx, code); err != nil {
			return fmt.Errorf("retire permission %q: %w", code, err)
		}
	}

	if err := q.GrantAllButtonsToSystemRole(ctx); err != nil {
		return fmt.Errorf("grant permissions to the super administrator role: %w", err)
	}
	return nil
}
