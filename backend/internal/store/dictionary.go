package store

import (
	"context"
	"strconv"
	"sync"
	"time"

	"dragz-admin/backend/internal/pkg/ctxkey"
	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// DictionaryType is one sys_dict_type row together with its item count.
type DictionaryType struct {
	ID          uint64    `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	DisplayType string    `json:"displayType"`
	Status      int8      `json:"status"`
	IsSystem    bool      `json:"isSystem"`
	Remark      string    `json:"remark"`
	ItemCount   int64     `json:"itemCount"`
	CreatedAt   time.Time `json:"createdAt"`
}

// DictionaryItem is one sys_dict_item row.
type DictionaryItem struct {
	ID         uint64    `json:"id"`
	DictTypeID uint64    `json:"dictTypeId"`
	Label      string    `json:"label"`
	Value      string    `json:"value"`
	Color      string    `json:"color"`
	CSSClass   string    `json:"cssClass"`
	Sort       int       `json:"sort"`
	Status     int8      `json:"status"`
	IsDefault  bool      `json:"isDefault"`
	Remark     string    `json:"remark"`
	CreatedAt  time.Time `json:"createdAt"`
}

// SaveDictionaryTypeInput is the payload shared by create and update of a dictionary type.
type SaveDictionaryTypeInput struct {
	Code        string
	Name        string
	DisplayType string
	Status      int8
	Remark      string
}

// SaveDictionaryItemInput is the payload shared by create and update of a dictionary item.
type SaveDictionaryItemInput struct {
	Label     string
	Value     string
	Color     string
	CSSClass  string
	Sort      int
	Status    int8
	IsDefault bool
	Remark    string
}

// dictCache is the single-process cache behind DictionaryItemsByCode, the
// endpoint every generated business page reads on each render. Entries are
// keyed by tenant and type code (row-level security scopes each query, so the
// key must too) and hold the enabled items in sort order. Every dictionary
// write clears the whole map: admin-scale write frequency makes selective
// invalidation pointless, and a full clear can never leave a stale entry.
//
// The cache is per-process by design. Multi-replica deployments trade at most
// one stale render per replica after a write — the same trade yudao makes with
// its in-process layer before the Redis broadcast, without the broker.
type dictCache struct {
	mu    sync.RWMutex
	items map[string][]DictionaryItem
}

func (c *dictCache) key(tenantID uint64, code string) string {
	return strconv.FormatUint(tenantID, 10) + "\x00" + code
}

func (c *dictCache) get(tenantID uint64, code string) ([]DictionaryItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	items, ok := c.items[c.key(tenantID, code)]
	return items, ok
}

func (c *dictCache) put(tenantID uint64, code string, items []DictionaryItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		c.items = make(map[string][]DictionaryItem)
	}
	c.items[c.key(tenantID, code)] = items
}

func (c *dictCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = nil
}

// ListDictionaryTypes returns dictionary types filtered by keyword and status. A nil status means "any status".
func (r *AuthRepository) ListDictionaryTypes(ctx context.Context, keyword string, status *int8) ([]DictionaryType, error) {
	rows, err := r.queries.ListDictionaryTypesFiltered(ctx, sqlcstore.ListDictionaryTypesFilteredParams{
		KeywordPattern: optionalPattern(keyword),
		Status:         optionalInt16(status),
	})
	if err != nil {
		return nil, err
	}
	items := make([]DictionaryType, 0, len(rows))
	for _, row := range rows {
		items = append(items, DictionaryType{
			ID: uint64(row.ID), Code: row.Code, Name: row.Name, DisplayType: row.DisplayType, Status: int8(row.Status),
			IsSystem: smallIntBool(row.IsSystem), Remark: row.Remark, ItemCount: row.ItemCount, CreatedAt: row.CreatedAt,
		})
	}
	return items, nil
}

// DictionaryTypeByID returns one live type, or ErrNotFound. The item endpoints
// need the type's code to build translation keys; the update path uses it to
// enforce the system-code lock.
func (r *AuthRepository) DictionaryTypeByID(ctx context.Context, dictTypeID uint64) (DictionaryType, error) {
	row, err := r.queries.GetDictionaryTypeByID(ctx, int64(dictTypeID))
	if err != nil {
		return DictionaryType{}, ErrNotFound
	}
	return DictionaryType{
		ID: uint64(row.ID), Code: row.Code, Name: row.Name, DisplayType: row.DisplayType,
		Status: int8(row.Status), IsSystem: smallIntBool(row.IsSystem), Remark: row.Remark,
	}, nil
}

// CreateDictionaryType inserts a type and reports a code clash as ErrDuplicateEntry.
func (r *AuthRepository) CreateDictionaryType(ctx context.Context, input SaveDictionaryTypeInput) (uint64, error) {
	id, err := r.queries.CreateDictionaryType(ctx, sqlcstore.CreateDictionaryTypeParams{
		Code: input.Code, Name: input.Name, DisplayType: input.DisplayType, Status: int16(input.Status), Remark: input.Remark,
	})
	r.invalidateDictCache()
	return uint64(id), wrapDuplicate(err)
}

// UpdateDictionaryType rewrites a type and reports a code clash as ErrDuplicateEntry.
//
// A system dictionary's code is locked: item translation keys
// (dict.<code>.<value>) and the tenant-provisioning template both key off it.
// Renaming a non-system type carries its translation records over — the type's
// own name and, in one prefix move, every item's key — so no translation is
// orphaned by the rename.
func (r *AuthRepository) UpdateDictionaryType(ctx context.Context, dictTypeID uint64, input SaveDictionaryTypeInput) error {
	current, err := r.DictionaryTypeByID(ctx, dictTypeID)
	if err != nil {
		return err
	}
	if current.IsSystem && current.Code != input.Code {
		return ErrDictCodeLocked
	}
	affected, err := r.queries.UpdateDictionaryType(ctx, sqlcstore.UpdateDictionaryTypeParams{
		Code: input.Code, Name: input.Name, DisplayType: input.DisplayType, Status: int16(input.Status), Remark: input.Remark, ID: int64(dictTypeID),
	})
	if err != nil {
		return wrapDuplicate(err)
	}
	if affected != 1 {
		return ErrNotFound
	}
	if current.Code != input.Code {
		if err := r.RekeyEntityTranslations(ctx, "dicttype."+current.Code, "dicttype."+input.Code); err != nil {
			return err
		}
		if _, err := r.queries.RekeyDictTranslationsByPrefix(ctx, sqlcstore.RekeyDictTranslationsByPrefixParams{
			OldPrefix: "dict." + current.Code + ".",
			NewPrefix: "dict." + input.Code + ".",
		}); err != nil {
			return err
		}
	}
	r.invalidateDictCache()
	return nil
}

// DeleteDictionaryType soft deletes a type. Seeded types and types that still have items are refused with ErrDictHasItems.
func (r *AuthRepository) DeleteDictionaryType(ctx context.Context, dictTypeID uint64) error {
	checks, err := r.queries.GetDictionaryTypeDeleteChecks(ctx, int64(dictTypeID))
	if err != nil {
		return err
	}
	if smallIntBool(checks.IsSystem) || checks.ItemCount > 0 {
		return ErrDictHasItems
	}
	affected, err := r.queries.SoftDeleteDictionaryType(ctx, int64(dictTypeID))
	if err != nil {
		return wrapDuplicate(err)
	}
	if affected != 1 {
		return ErrNotFound
	}
	r.invalidateDictCache()
	return nil
}

// ListDictionaryItems returns the items of one type. enabledOnly restricts the result to items the frontend may render.
func (r *AuthRepository) ListDictionaryItems(ctx context.Context, dictTypeID uint64, enabledOnly bool) ([]DictionaryItem, error) {
	if enabledOnly {
		rows, err := r.queries.ListEnabledDictionaryItems(ctx, dictTypeID)
		if err != nil {
			return nil, err
		}
		return mapDictItemsEnabled(rows), nil
	}
	rows, err := r.queries.ListDictionaryItems(ctx, dictTypeID)
	if err != nil {
		return nil, err
	}
	return mapDictItems(rows), nil
}

func mapDictItems(rows []sqlcstore.ListDictionaryItemsRow) []DictionaryItem {
	items := make([]DictionaryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, DictionaryItem{
			ID: uint64(row.ID), DictTypeID: row.DictTypeID, Label: row.Label, Value: row.Value,
			Color: row.Color, CSSClass: row.CssClass, Sort: int(row.Sort), Status: int8(row.Status),
			IsDefault: smallIntBool(row.IsDefault), Remark: row.Remark, CreatedAt: row.CreatedAt,
		})
	}
	return items
}

func mapDictItemsEnabled(rows []sqlcstore.ListEnabledDictionaryItemsRow) []DictionaryItem {
	items := make([]DictionaryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, DictionaryItem{
			ID: uint64(row.ID), DictTypeID: row.DictTypeID, Label: row.Label, Value: row.Value,
			Color: row.Color, CSSClass: row.CssClass, Sort: int(row.Sort), Status: int8(row.Status),
			IsDefault: smallIntBool(row.IsDefault), Remark: row.Remark, CreatedAt: row.CreatedAt,
		})
	}
	return items
}

// DictionaryItemsByCode resolves a type by its code and returns its enabled items, which is how business pages read a dictionary.
//
// This is the hot path (every generated page, every render), so it goes
// through the per-tenant cache. The cached slice is copied before returning:
// the HTTP layer localizes labels in place per request, and a shared slice
// would let one request's language leak into the next.
func (r *AuthRepository) DictionaryItemsByCode(ctx context.Context, code string) ([]DictionaryItem, error) {
	tenantID, _ := ctxkey.TenantIDFrom(ctx)
	if items, ok := r.dictCache.get(tenantID, code); ok {
		return append([]DictionaryItem(nil), items...), nil
	}
	dictTypeID, err := r.queries.GetDictionaryTypeIDByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	items, err := r.ListDictionaryItems(ctx, uint64(dictTypeID), true)
	if err != nil {
		return nil, err
	}
	r.dictCache.put(tenantID, code, items)
	return append([]DictionaryItem(nil), items...), nil
}

// invalidateDictCache drops every cached dictionary. Write paths call it after
// a successful write; a failure leaves the cache untouched, which is correct
// because the data has not changed.
func (r *AuthRepository) invalidateDictCache() {
	r.dictCache.clear()
}

// CreateDictionaryItem inserts an item and reports a duplicate value as ErrDuplicateEntry.
//
// A default flag is written in two steps inside one transaction: every other
// default of the type is cleared first, then the new row lands with its flag.
// uk_dict_item_default is an immediate constraint — PostgreSQL partial unique
// indexes cannot be deferrable — so the clear has to precede the insert or
// two concurrent saves would trip it.
func (r *AuthRepository) CreateDictionaryItem(ctx context.Context, dictTypeID uint64, input SaveDictionaryItemInput) (uint64, error) {
	if !input.IsDefault {
		id, err := r.queries.CreateDictionaryItem(ctx, sqlcstore.CreateDictionaryItemParams{
			DictTypeID: dictTypeID, Label: input.Label, Value: input.Value, Color: input.Color,
			CssClass: input.CSSClass, Sort: int32(input.Sort), Status: int16(input.Status),
			IsDefault: boolSmallInt16(input.IsDefault), Remark: input.Remark,
		})
		r.invalidateDictCache()
		return uint64(id), wrapDuplicate(err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	if _, err := q.ClearDictionaryDefault(ctx, dictTypeID); err != nil {
		return 0, err
	}
	id, err := q.CreateDictionaryItem(ctx, sqlcstore.CreateDictionaryItemParams{
		DictTypeID: dictTypeID, Label: input.Label, Value: input.Value, Color: input.Color,
		CssClass: input.CSSClass, Sort: int32(input.Sort), Status: int16(input.Status),
		IsDefault: boolSmallInt16(input.IsDefault), Remark: input.Remark,
	})
	if err != nil {
		return 0, wrapDuplicate(err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	r.invalidateDictCache()
	return uint64(id), nil
}

// UpdateDictionaryItem rewrites an item and reports a duplicate value as ErrDuplicateEntry.
//
// Two changes are guarded by the reference check: disabling an item, and
// changing its value. Both leave business rows carrying a value no enabled —
// or existing — item explains. Label, color and sort changes are free, since
// historical rows keep rendering through the same item row. A value change
// that passes the check carries the item's translations to the new key.
func (r *AuthRepository) UpdateDictionaryItem(ctx context.Context, itemID uint64, input SaveDictionaryItemInput) error {
	current, err := r.DictionaryItemByID(ctx, itemID)
	if err != nil {
		return err
	}
	dictType, err := r.DictionaryTypeByID(ctx, current.DictTypeID)
	if err != nil {
		return err
	}
	valueChanged := current.Value != input.Value
	disabled := current.Status == 1 && input.Status == 0
	if (valueChanged || disabled) && dictType.Code != "" {
		inUse, err := r.dictItemInUse(ctx, dictType.Code, current.Value)
		if err != nil {
			return err
		}
		if inUse {
			return ErrDictItemInUse
		}
	}
	// Setting this item as the type's default is a two-step write inside one
	// transaction (see CreateDictionaryItem): clear the type's other defaults
	// first — the row being edited included, since its old flag is rewritten
	// by the update below — then apply the update. Un-setting the flag needs
	// no companion step: uk_dict_item_default only constrains rows that keep
	// it, and a type left without a default is a normal state.
	update := func(q *sqlcstore.Queries) error {
		affected, err := q.UpdateDictionaryItem(ctx, sqlcstore.UpdateDictionaryItemParams{
			Label: input.Label, Value: input.Value, Color: input.Color, CssClass: input.CSSClass,
			Sort: int32(input.Sort), Status: int16(input.Status), IsDefault: boolSmallInt16(input.IsDefault),
			Remark: input.Remark, ID: int64(itemID),
		})
		if err != nil {
			return wrapDuplicate(err)
		}
		if affected != 1 {
			return ErrNotFound
		}
		return nil
	}
	if input.IsDefault {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		q := r.qtx(tx)
		if _, err := q.ClearDictionaryDefault(ctx, current.DictTypeID); err != nil {
			return err
		}
		if err := update(q); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	} else if err := update(r.queries); err != nil {
		return err
	}
	if valueChanged {
		if err := r.RekeyEntityTranslations(ctx,
			"dict."+dictType.Code+"."+current.Value,
			"dict."+dictType.Code+"."+input.Value); err != nil {
			return err
		}
	}
	r.invalidateDictCache()
	return nil
}

// DictionaryItemByID returns one live item, or ErrNotFound.
func (r *AuthRepository) DictionaryItemByID(ctx context.Context, itemID uint64) (DictionaryItem, error) {
	row, err := r.queries.GetDictionaryItemByID(ctx, int64(itemID))
	if err != nil {
		return DictionaryItem{}, ErrNotFound
	}
	return DictionaryItem{
		ID: uint64(row.ID), DictTypeID: row.DictTypeID, Label: row.Label, Value: row.Value,
		Color: row.Color, CSSClass: row.CssClass, Sort: int(row.Sort), Status: int8(row.Status),
		IsDefault: smallIntBool(row.IsDefault), Remark: row.Remark,
	}, nil
}

// DeleteDictionaryItem soft deletes one item. An item whose value is still
// referenced by business rows is refused with ErrDictItemInUse: the rows would
// survive without any label to render.
func (r *AuthRepository) DeleteDictionaryItem(ctx context.Context, itemID uint64) error {
	current, err := r.DictionaryItemByID(ctx, itemID)
	if err != nil {
		return err
	}
	dictType, err := r.DictionaryTypeByID(ctx, current.DictTypeID)
	if err != nil {
		return err
	}
	if dictType.Code != "" {
		inUse, err := r.dictItemInUse(ctx, dictType.Code, current.Value)
		if err != nil {
			return err
		}
		if inUse {
			return ErrDictItemInUse
		}
	}
	affected, err := r.queries.SoftDeleteDictionaryItem(ctx, int64(itemID))
	if err != nil {
		return wrapDuplicate(err)
	}
	if affected != 1 {
		return ErrNotFound
	}
	r.invalidateDictCache()
	return nil
}
