package store

import (
	"context"
	"time"

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

// CreateDictionaryType inserts a type and reports a code clash as ErrDuplicateEntry.
func (r *AuthRepository) CreateDictionaryType(ctx context.Context, input SaveDictionaryTypeInput) (uint64, error) {
	id, err := r.queries.CreateDictionaryType(ctx, sqlcstore.CreateDictionaryTypeParams{
		Code: input.Code, Name: input.Name, DisplayType: input.DisplayType, Status: int16(input.Status), Remark: input.Remark,
	})
	return uint64(id), wrapDuplicate(err)
}

// UpdateDictionaryType rewrites a type and reports a code clash as ErrDuplicateEntry.
func (r *AuthRepository) UpdateDictionaryType(ctx context.Context, dictTypeID uint64, input SaveDictionaryTypeInput) error {
	affected, err := r.queries.UpdateDictionaryType(ctx, sqlcstore.UpdateDictionaryTypeParams{
		Code: input.Code, Name: input.Name, DisplayType: input.DisplayType, Status: int16(input.Status), Remark: input.Remark, ID: int64(dictTypeID),
	})
	if err != nil {
		return wrapDuplicate(err)
	}
	if affected != 1 {
		return ErrNotFound
	}
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
func (r *AuthRepository) DictionaryItemsByCode(ctx context.Context, code string) ([]DictionaryItem, error) {
	dictTypeID, err := r.queries.GetDictionaryTypeIDByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	return r.ListDictionaryItems(ctx, uint64(dictTypeID), true)
}

// CreateDictionaryItem inserts an item and reports a duplicate value as ErrDuplicateEntry.
func (r *AuthRepository) CreateDictionaryItem(ctx context.Context, dictTypeID uint64, input SaveDictionaryItemInput) (uint64, error) {
	id, err := r.queries.CreateDictionaryItem(ctx, sqlcstore.CreateDictionaryItemParams{
		DictTypeID: dictTypeID, Label: input.Label, Value: input.Value, Color: input.Color,
		CssClass: input.CSSClass, Sort: int32(input.Sort), Status: int16(input.Status),
		IsDefault: boolSmallInt16(input.IsDefault), Remark: input.Remark,
	})
	return uint64(id), wrapDuplicate(err)
}

// UpdateDictionaryItem rewrites an item and reports a duplicate value as ErrDuplicateEntry.
func (r *AuthRepository) UpdateDictionaryItem(ctx context.Context, itemID uint64, input SaveDictionaryItemInput) error {
	affected, err := r.queries.UpdateDictionaryItem(ctx, sqlcstore.UpdateDictionaryItemParams{
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

// DeleteDictionaryItem soft deletes one item.
func (r *AuthRepository) DeleteDictionaryItem(ctx context.Context, itemID uint64) error {
	affected, err := r.queries.SoftDeleteDictionaryItem(ctx, int64(itemID))
	if err != nil {
		return wrapDuplicate(err)
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}
