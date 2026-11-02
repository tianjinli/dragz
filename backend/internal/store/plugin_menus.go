package store

import (
	"context"
	"errors"
	"fmt"

	"dragz-admin/backend/internal/menuperm"
)

// menuRow is the slice of sys_menu the declared-menu sync needs to decide
// between an insert and an update.
type menuRow struct {
	id       uint64
	pluginID string
}

// SyncPluginMenus upserts the menus a plugin declares in Meta.Menus.
//
// This is the id-free counterpart of SyncPermissionMenus. A plugin names its
// menus by code and never writes sys_menu.id, so the primary key is always
// allocated by the sequence and two plugins cannot collide on it. What used to
// be a silent `ON CONFLICT DO NOTHING` skip inside a migration is now either a
// successful upsert or a loud error.
//
// Ownership: a code that already exists under a different plugin_id — core
// menus included, since their plugin_id is empty — is refused instead of taken
// over, so a copied-and-pasted menu code fails at enable time with the two
// owners named.
//
// Retirement: a live menu owned by one of pluginIDs but absent from the batch
// is soft deleted, which makes the declaration the whole truth. That is safe
// because the admin UI never writes plugin_id (CreateMenu leaves it empty), so
// a hand-made menu can never fall into a plugin's retirement scope. A retired
// menu that comes back is revived as the same row, keeping its id and grants.
//
// Fields: the structure (parent, type, route) is re-applied on every call; the
// presentation (name, icon, sort, visible, keep_alive) is written on first
// insert only, so an administrator's rename and reorder survive a re-enable.
//
// The whole batch runs in one transaction, so a failure halfway leaves nothing
// behind and the caller can fail before the plugin is flipped on.
func (r *PluginRepository) SyncPluginMenus(ctx context.Context, pluginIDs []string, nodes []menuperm.MenuNode) error {
	ordered, err := menuperm.OrderMenus(nodes)
	if err != nil {
		return err
	}
	if len(pluginIDs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := syncPluginMenus(ctx, tx, pluginIDs, ordered); err != nil {
		return err
	}
	return tx.Commit()
}

// syncPluginMenus is the body of SyncPluginMenus, bound to a caller supplied
// transaction so a test can run it against a scratch schema and roll it back.
func syncPluginMenus(ctx context.Context, tx *Tx, pluginIDs []string, ordered []menuperm.MenuNode) error {
	existing, err := listMenuOwners(ctx, tx)
	if err != nil {
		return err
	}
	retired, err := listRetiredMenus(ctx, tx, pluginIDs)
	if err != nil {
		return err
	}
	ids := make(map[string]uint64, len(existing)+len(ordered))
	for code, row := range existing {
		ids[code] = row.id
	}

	for _, node := range ordered {
		row, found := existing[node.Code]
		if found {
			if row.pluginID != node.PluginID {
				return fmt.Errorf("%w: %q belongs to plugin %q, not %q",
					ErrMenuCodeTaken, node.Code, row.pluginID, node.PluginID)
			}
			if err := updatePluginMenu(ctx, tx, row.id, node); err != nil {
				return fmt.Errorf("update plugin menu %q: %w", node.Code, err)
			}
			continue
		}
		// A code this plugin declared before and then dropped comes back as the
		// same row: its id, and therefore every grant pointing at it, survives.
		if id, wasRetired := retired[node.PluginID][node.Code]; wasRetired {
			if err := revivePluginMenu(ctx, tx, id, node); err != nil {
				return fmt.Errorf("revive plugin menu %q: %w", node.Code, err)
			}
			ids[node.Code] = id
			continue
		}
		id, err := insertPluginMenu(ctx, tx, node)
		if err != nil {
			return fmt.Errorf("insert plugin menu %q: %w", node.Code, err)
		}
		ids[node.Code] = id
	}

	// Parent ids come second: a parent may have been inserted in the same
	// batch, so it has no id yet while the loop above is running.
	for _, node := range ordered {
		var parentID uint64
		if node.ParentCode != "" {
			id, ok := ids[node.ParentCode]
			if !ok {
				return fmt.Errorf("%w: menu %q declares parent %q, which does not exist",
					ErrMenuParentMissing, node.Code, node.ParentCode)
			}
			parentID = id
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE sys_menu SET parent_id = $2, updated_at = NOW()
			 WHERE id = $1 AND deleted_at IS NULL`,
			ids[node.Code], parentID,
		); err != nil {
			return fmt.Errorf("link plugin menu %q to parent: %w", node.Code, err)
		}
	}
	// Retirements come last so a parent that is about to disappear cannot be
	// referenced by a child that was just linked to it.
	if err := retireUndeclared(ctx, tx, pluginIDs, ordered); err != nil {
		return err
	}
	return nil
}

// retireUndeclared soft deletes the live menus a plugin owns but no longer
// declares. Grants are left in place, exactly like the button sync, so a menu
// that comes back is revived with its permissions intact.
//
// Rows are collected before any UPDATE runs: iterating an open result set while
// writing to the same table would deadlock a single connection.
func retireUndeclared(ctx context.Context, tx *Tx, pluginIDs []string, ordered []menuperm.MenuNode) error {
	declared := make(map[string]map[string]bool, len(pluginIDs))
	for _, node := range ordered {
		if declared[node.PluginID] == nil {
			declared[node.PluginID] = make(map[string]bool)
		}
		declared[node.PluginID][node.Code] = true
	}

	for _, pluginID := range pluginIDs {
		rows, err := tx.QueryContext(ctx,
			`SELECT id, code FROM sys_menu
			 WHERE type <> 'button' AND plugin_id = $1 AND deleted_at IS NULL`,
			pluginID,
		)
		if err != nil {
			return fmt.Errorf("list menus of plugin %q: %w", pluginID, err)
		}
		type stale struct {
			id   uint64
			code string
		}
		var doomed []stale
		for rows.Next() {
			var id int64
			var code string
			if err := rows.Scan(&id, &code); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan menu of plugin %q: %w", pluginID, err)
			}
			if !declared[pluginID][code] {
				doomed = append(doomed, stale{id: uint64(id), code: code})
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return fmt.Errorf("iterate menus of plugin %q: %w", pluginID, err)
		}
		_ = rows.Close()

		for _, row := range doomed {
			if _, err := tx.ExecContext(ctx,
				`UPDATE sys_menu SET deleted_at = NOW(), status = 0, updated_at = NOW()
				 WHERE id = $1 AND deleted_at IS NULL`,
				row.id,
			); err != nil {
				return fmt.Errorf("retire menu %q of plugin %q: %w", row.code, pluginID, err)
			}
		}
	}
	return nil
}

// listRetiredMenus maps pluginID -> code -> id for the soft deleted menus the
// batch's plugins own, so a re-declared menu can be revived instead of
// re-inserted. Codes owned by other plugins are excluded on purpose: their
// tombstones are not a claim on the code, which the partial unique index has
// already released.
func listRetiredMenus(ctx context.Context, tx *Tx, pluginIDs []string) (map[string]map[string]uint64, error) {
	retired := make(map[string]map[string]uint64, len(pluginIDs))
	for _, pluginID := range pluginIDs {
		rows, err := tx.QueryContext(ctx,
			`SELECT id, code FROM sys_menu
			 WHERE type <> 'button' AND plugin_id = $1 AND deleted_at IS NOT NULL
			 ORDER BY id`,
			pluginID,
		)
		if err != nil {
			return nil, fmt.Errorf("list retired menus of plugin %q: %w", pluginID, err)
		}
		for rows.Next() {
			var id int64
			var code string
			if err := rows.Scan(&id, &code); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scan retired menu of plugin %q: %w", pluginID, err)
			}
			if retired[pluginID] == nil {
				retired[pluginID] = make(map[string]uint64)
			}
			retired[pluginID][code] = uint64(id)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("iterate retired menus of plugin %q: %w", pluginID, err)
		}
		_ = rows.Close()
	}
	return retired, nil
}

// listMenuOwners maps every live non-button menu code to its row and owner.
// Buttons are excluded: they are owned by the route-table sync and a menu code
// must never resolve to one.
func listMenuOwners(ctx context.Context, tx *Tx) (map[string]menuRow, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, code, plugin_id FROM sys_menu
		 WHERE type <> 'button' AND deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("list menu owners: %w", err)
	}
	defer func() { _ = rows.Close() }()

	owners := make(map[string]menuRow)
	for rows.Next() {
		var id int64
		var code, pluginID string
		if err := rows.Scan(&id, &code, &pluginID); err != nil {
			return nil, fmt.Errorf("scan menu owner: %w", err)
		}
		owners[code] = menuRow{id: uint64(id), pluginID: pluginID}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate menu owners: %w", err)
	}
	return owners, nil
}

// revivePluginMenu brings a retired row back. The id is deliberately preserved:
// sys_role_menu still points at it, so a menu that disappears and returns keeps
// its permission grants instead of silently losing them.
func revivePluginMenu(ctx context.Context, tx *Tx, id uint64, node menuperm.MenuNode) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE sys_menu
		 SET deleted_at = NULL, status = 1, parent_id = 0, type = $2,
		     route_name = $3, route_path = $4, component = $5, redirect = $6,
		     icon = $7, updated_at = NOW()
		 WHERE id = $1`,
		id,
		menuperm.NormalizeType(node.Type),
		node.RouteName,
		node.RoutePath,
		node.Component,
		node.Redirect,
		node.Icon,
	); err != nil {
		return err
	}
	// The label may never have been written (the row could predate NameEn), so
	// seed it again, still absent-on-conflict.
	if node.NameEn != "" && node.NameEn != node.Name {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sys_translation (entity_key, lang, value)
			 VALUES ($1, 'en', $2)
			 ON CONFLICT (entity_key, lang) DO NOTHING`,
			"menu."+node.Code, node.NameEn,
		); err != nil {
			return fmt.Errorf("seed translation %q: %w", node.Code, err)
		}
	}
	return nil
}

// insertPluginMenu creates the row without an id, which is the whole point:
// BIGSERIAL allocates it, so it cannot clash with another plugin's menu.
func insertPluginMenu(ctx context.Context, tx *Tx, node menuperm.MenuNode) (uint64, error) {
	var id int64
	err := tx.QueryRowContext(ctx,
		`INSERT INTO sys_menu
		   (parent_id, type, name, code, route_name, route_path, component,
		    redirect, icon, sort, status, visible, keep_alive, plugin_id)
		 VALUES (0, $1, $2, $3, $4, $5, $6, $7, $8, $9, 1, $10, $11, $12)
		 RETURNING id`,
		menuperm.NormalizeType(node.Type),
		node.Name,
		node.Code,
		node.RouteName,
		node.RoutePath,
		node.Component,
		node.Redirect,
		node.Icon,
		int32(node.Sort),
		boolSmallInt16(!node.Hidden),
		boolSmallInt16(node.KeepAlive),
		node.PluginID,
	).Scan(&id)
	if err != nil {
		// uk_menu_code is unique among live rows. A collision here means a
		// permission button or a row created while this batch was running took
		// the code; either way this plugin may not have it.
		if errors.Is(wrapDuplicate(err), ErrDuplicateEntry) {
			return 0, fmt.Errorf("%w: %q is already used by a permission button", ErrMenuCodeTaken, node.Code)
		}
		return 0, err
	}

	// Seed the English label. Absent-on-conflict, exactly like the button sync,
	// so an administrator-edited translation is never clobbered.
	if node.NameEn != "" && node.NameEn != node.Name {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sys_translation (entity_key, lang, value)
			 VALUES ($1, 'en', $2)
			 ON CONFLICT (entity_key, lang) DO NOTHING`,
			"menu."+node.Code, node.NameEn,
		); err != nil {
			return 0, fmt.Errorf("seed translation %q: %w", node.Code, err)
		}
	}
	return uint64(id), nil
}

// updatePluginMenu re-applies the structural fields. Name, icon, sort and the
// visibility flags are deliberately left alone: they are the administrator's
// after the first insert.
func updatePluginMenu(ctx context.Context, tx *Tx, id uint64, node menuperm.MenuNode) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE sys_menu
		 SET type = $2, route_name = $3, route_path = $4, component = $5,
		     redirect = $6, icon = $7, updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`,
		id,
		menuperm.NormalizeType(node.Type),
		node.RouteName,
		node.RoutePath,
		node.Component,
		node.Redirect,
		node.Icon,
	)
	return err
}
