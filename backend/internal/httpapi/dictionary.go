package httpapi

import (
	"net/http"
	"regexp"
	"strings"

	"dragz-admin/backend/internal/app"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/store"
)

type saveDictionaryTypeRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	DisplayType string `json:"displayType"`
	Status      int8   `json:"status"`
	Remark      string `json:"remark"`
}

type saveDictionaryItemRequest struct {
	Label     string `json:"label"`
	Value     string `json:"value"`
	Color     string `json:"color"`
	CSSClass  string `json:"cssClass"`
	Sort      int    `json:"sort"`
	Status    int8   `json:"status"`
	IsDefault bool   `json:"isDefault"`
	Remark    string `json:"remark"`
}

func listDictionaryTypesHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, valid := optionalStatus(r.URL.Query().Get("status"))
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.dict_status_invalid")
			return
		}
		items, err := ctx.Auth.ListDictionaryTypes(r.Context(), r.URL.Query().Get("keyword"), status)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.dict_list_failed")
			return
		}
		items = localizePreset(r, ctx.Auth, items,
			func(item store.DictionaryType) []string { return []string{"dicttype." + item.Code} },
			func(item *store.DictionaryType, values map[string]string) {
				if s, ok := values["dicttype."+item.Code]; ok {
					item.Name = s
				}
			})
		ok(w, r, items)
	}
}

func createDictionaryTypeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveDictionaryTypeRequest
		if !decode(w, r, &req) || !validateDictionaryType(w, r, req) {
			return
		}
		id, err := ctx.Auth.CreateDictionaryType(r.Context(), dictionaryTypeInput(req))
		if err != nil {
			writeDictionaryError(w, r, err)
			return
		}
		ok(w, r, map[string]uint64{"id": id})
	}
}

func updateDictionaryTypeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.dict_id_invalid")
			return
		}
		var req saveDictionaryTypeRequest
		if !decode(w, r, &req) || !validateDictionaryType(w, r, req) {
			return
		}
		if err := ctx.Auth.UpdateDictionaryType(r.Context(), id, dictionaryTypeInput(req)); err != nil {
			writeDictionaryError(w, r, err)
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func deleteDictionaryTypeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.dict_id_invalid")
			return
		}
		if err := ctx.Auth.DeleteDictionaryType(r.Context(), id); err != nil {
			writeDictionaryError(w, r, err)
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

// localizeDictItems rewrites item labels (or a type's name) in the request
// language. Keys follow v_preset_entity: dicttype.<code> for the type name,
// dict.<code>.<value> for an item's label. Missing translations fall back to
// the stored Chinese label by simply not being applied.
func localizeDictItems(r *http.Request, loader translationLoader, typeCode string, items []store.DictionaryItem) []store.DictionaryItem {
	return localizePreset(r, loader, items,
		func(item store.DictionaryItem) []string { return []string{"dict." + typeCode + "." + item.Value} },
		func(item *store.DictionaryItem, values map[string]string) {
			if s, ok := values["dict."+typeCode+"."+item.Value]; ok {
				item.Label = s
			}
		})
}

func listDictionaryItemsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.dict_id_invalid")
			return
		}
		items, err := ctx.Auth.ListDictionaryItems(r.Context(), id, false)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.dict_items_load_failed")
			return
		}
		dictType, err := ctx.Auth.DictionaryTypeByID(r.Context(), id)
		if err == nil && dictType.Code != "" {
			items = localizeDictItems(r, ctx.Auth, dictType.Code, items)
		}
		ok(w, r, items)
	}
}

func dictionaryItemsByCodeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := pathvarValue(r, "code")
		items, err := ctx.Auth.DictionaryItemsByCode(r.Context(), code)
		if err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.dict.not_found")
			return
		}
		items = localizeDictItems(r, ctx.Auth, code, items)
		ok(w, r, items)
	}
}

func createDictionaryItemHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dictTypeID, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.dict_id_invalid")
			return
		}
		var req saveDictionaryItemRequest
		if !decode(w, r, &req) || !validateDictionaryItem(w, r, req) {
			return
		}
		id, err := ctx.Auth.CreateDictionaryItem(r.Context(), dictTypeID, dictionaryItemInput(req))
		if err != nil {
			writeDictionaryError(w, r, err)
			return
		}
		ok(w, r, map[string]uint64{"id": id})
	}
}

func updateDictionaryItemHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.dict_item_id_invalid")
			return
		}
		var req saveDictionaryItemRequest
		if !decode(w, r, &req) || !validateDictionaryItem(w, r, req) {
			return
		}
		if err := ctx.Auth.UpdateDictionaryItem(r.Context(), id, dictionaryItemInput(req)); err != nil {
			writeDictionaryError(w, r, err)
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func deleteDictionaryItemHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.dict_item_id_invalid")
			return
		}
		if err := ctx.Auth.DeleteDictionaryItem(r.Context(), id); err != nil {
			writeDictionaryError(w, r, err)
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func validateDictionaryType(w http.ResponseWriter, r *http.Request, req saveDictionaryTypeRequest) bool {
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" || len(req.Code) > 128 || len(req.Name) > 128 {
		fail(w, r, http.StatusBadRequest, 40001, "errors.dict_fields_invalid")
		return false
	}
	// The code is the middle segment of item translation keys
	// (dict.<code>.<value>): a dot would make "dict.a.b" ambiguous between a
	// type "a" item and a type "a.b" name, so codes are snake_case only.
	if !dictCodePattern.MatchString(req.Code) {
		fail(w, r, http.StatusBadRequest, 40001, "errors.dict_code_invalid")
		return false
	}
	if !validDictionaryDisplayType(req.DisplayType) {
		fail(w, r, http.StatusBadRequest, 40001, "errors.dict_display_type_invalid")
		return false
	}
	return req.Status == 0 || req.Status == 1
}

// dictCodePattern constrains dictionary type codes to lowercase snake_case:
// the code is interpolated into translation keys and a prefix-matching rekey,
// so anything beyond letters, digits and underscores is off-limits.
var dictCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func validDictionaryDisplayType(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "dropdown", "segmented", "checkbox":
		return true
	default:
		return false
	}
}

func validateDictionaryItem(w http.ResponseWriter, r *http.Request, req saveDictionaryItemRequest) bool {
	if strings.TrimSpace(req.Label) == "" || strings.TrimSpace(req.Value) == "" || len(req.Label) > 128 || len(req.Value) > 255 {
		fail(w, r, http.StatusBadRequest, 40001, "errors.dict_item_fields_invalid")
		return false
	}
	return req.Status == 0 || req.Status == 1
}

func dictionaryTypeInput(req saveDictionaryTypeRequest) store.SaveDictionaryTypeInput {
	return store.SaveDictionaryTypeInput{
		Code: strings.TrimSpace(req.Code), Name: strings.TrimSpace(req.Name),
		DisplayType: normalizedDictionaryDisplayType(req.DisplayType), Status: req.Status, Remark: strings.TrimSpace(req.Remark),
	}
}

func normalizedDictionaryDisplayType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "segmented"
	}
	return value
}

func dictionaryItemInput(req saveDictionaryItemRequest) store.SaveDictionaryItemInput {
	return store.SaveDictionaryItemInput{
		Label: strings.TrimSpace(req.Label), Value: strings.TrimSpace(req.Value),
		Color: strings.TrimSpace(req.Color), CSSClass: strings.TrimSpace(req.CSSClass),
		Sort: req.Sort, Status: req.Status, IsDefault: req.IsDefault, Remark: strings.TrimSpace(req.Remark),
	}
}

func writeDictionaryError(w http.ResponseWriter, r *http.Request, err error) {
	failHTTP(w, r, apierrors.MapDictionary(err))
}
