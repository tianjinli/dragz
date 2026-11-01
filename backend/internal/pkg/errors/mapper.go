package errors

import (
	"database/sql"
	"errors"
	"net/http"

	"dragz-admin/backend/internal/store"
)

// HTTPError is the transport-level shape of a failure: status, business code, i18n message key and optional template data.
type HTTPError struct {
	Status  int
	Code    int
	Message string
	Data    map[string]any
}

func subjectData(subjectKey string) map[string]any {
	return map[string]any{"SubjectKey": subjectKey}
}

// MapRBAC maps a role or menu error. subjectKey names the object in the message so the UI can say what was not found or was protected.
func MapRBAC(err error, subjectKey string) HTTPError {
	data := subjectData(subjectKey)
	if err == nil {
		return HTTPError{Status: http.StatusInternalServerError, Code: CodeInternal, Message: "errors.rbac.failed", Data: data}
	}
	if errors.Is(err, store.ErrDuplicateEntry) {
		return HTTPError{Status: http.StatusConflict, Code: CodeDuplicate, Message: "errors.rbac.duplicate", Data: data}
	}
	if errors.Is(err, store.ErrMenuParentInvalid) {
		return HTTPError{Status: http.StatusBadRequest, Code: CodeValidation, Message: "errors.menu_parent_invalid"}
	}
	if errors.Is(err, store.ErrMenuButtonManaged) {
		return HTTPError{Status: http.StatusConflict, Code: CodeProtected, Message: "errors.menu_button_managed"}
	}
	if errors.Is(err, store.ErrProtected) || errors.Is(err, store.ErrInUse) ||
		errors.Is(err, store.ErrHasChildren) || errors.Is(err, store.ErrHasDescendant) ||
		errors.Is(err, store.ErrMenuHasChildren) || errors.Is(err, store.ErrMenuCycle) ||
		errors.Is(err, store.ErrMenuDescendant) || errors.Is(err, store.ErrRoleProtected) {
		return HTTPError{Status: http.StatusConflict, Code: CodeProtected, Message: "errors.rbac.protected", Data: data}
	}
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, store.ErrNotFound) {
		return HTTPError{Status: http.StatusNotFound, Code: CodeNotFound, Message: "errors.rbac.not_found", Data: data}
	}
	return HTTPError{Status: http.StatusInternalServerError, Code: CodeInternal, Message: "errors.rbac.failed", Data: data}
}

// MapUserSave maps an account error. A missing user is reported as protected rather than not found, because the built-in administrator and a deleted user are indistinguishable to the caller.
func MapUserSave(err error) HTTPError {
	if errors.Is(err, store.ErrDuplicateEntry) {
		return HTTPError{Status: http.StatusConflict, Code: CodeDuplicate, Message: "errors.user.duplicate"}
	}
	if errors.Is(err, store.ErrUserProtected) || errors.Is(err, store.ErrNotFound) {
		return HTTPError{Status: http.StatusConflict, Code: CodeConflict, Message: "errors.user.protected"}
	}
	return HTTPError{Status: http.StatusInternalServerError, Code: CodeInternal, Message: "errors.user.save_failed"}
}

// MapDictionary maps a dictionary error.
func MapDictionary(err error) HTTPError {
	if errors.Is(err, store.ErrDuplicateEntry) {
		return HTTPError{Status: http.StatusConflict, Code: CodeDuplicate, Message: "errors.dict.duplicate"}
	}
	if errors.Is(err, store.ErrDictHasItems) || errors.Is(err, store.ErrProtected) {
		return HTTPError{Status: http.StatusConflict, Code: CodeProtected, Message: "errors.dict.protected"}
	}
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
		return HTTPError{Status: http.StatusNotFound, Code: CodeNotFound, Message: "errors.dict.not_found"}
	}
	return HTTPError{Status: http.StatusInternalServerError, Code: CodeInternal, Message: "errors.dict.failed"}
}

// MapSettings maps a configuration, provider or storage error.
func MapSettings(err error) HTTPError {
	if errors.Is(err, store.ErrReadonly) {
		return HTTPError{Status: http.StatusConflict, Code: CodeProtected, Message: "errors.settings.readonly"}
	}
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
		return HTTPError{Status: http.StatusNotFound, Code: CodeNotFound, Message: "errors.settings.not_found"}
	}
	if errors.Is(err, store.ErrInUse) || errors.Is(err, store.ErrDefaultOrInUse) {
		return HTTPError{Status: http.StatusConflict, Code: CodeProtected, Message: "errors.settings.protected"}
	}
	return HTTPError{Status: http.StatusInternalServerError, Code: CodeInternal, Message: "errors.settings.failed"}
}

// MapNotice maps an announcement error.
func MapNotice(err error) HTTPError {
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
		return HTTPError{Status: http.StatusNotFound, Code: CodeNotFound, Message: "errors.notice.not_found"}
	}
	if errors.Is(err, store.ErrNotEditable) ||
		errors.Is(err, store.ErrCannotPublish) || errors.Is(err, store.ErrCannotRevoke) {
		return HTTPError{Status: http.StatusConflict, Code: CodeConflict, Message: "errors.notice.conflict"}
	}
	return HTTPError{Status: http.StatusInternalServerError, Code: CodeInternal, Message: "errors.notice.failed"}
}

// MapStore is a generic sentinel mapper for domains without a specialized mapper.
func MapStore(err error, notFoundKey, failedKey string) HTTPError {
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
		return HTTPError{Status: http.StatusNotFound, Code: CodeNotFound, Message: notFoundKey}
	}
	if errors.Is(err, store.ErrDuplicateEntry) {
		return HTTPError{Status: http.StatusConflict, Code: CodeDuplicate, Message: "errors.rbac.duplicate"}
	}
	return HTTPError{Status: http.StatusInternalServerError, Code: CodeInternal, Message: failedKey}
}
