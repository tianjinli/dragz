package appkit

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

type I18nAdapter interface {
	RegisterTranslations(validate *validator.Validate) (err error)
	// ResolveTranslations resolve translations from catalog
	ResolveTranslations(localesDir string, rootNode string) error
	// GetTranslator detected `Accept-Language` header and get translator
	GetTranslator(lang string) (ut.Translator, bool)
	// RenderError render error to gin context, support i18n.I18nError and validator.ValidationErrors
	RenderError(ctx *gin.Context, err error)
}

type TransType int

const (
	TransIgnore TransType = iota - 1 // ignore translation
	TransNormal
	TransCardinal
	TransOrdinal
	TransRange
)

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

//func (r ErrorResponse) MarshalJSON() ([]byte, error) {
//	type Alias ErrorResponse
//	aux := struct {
//		Alias
//		Display string `json:"error,omitempty"`
//	}{
//		Alias: Alias(r),
//	}
//
//	if r.Error != nil {
//		aux.Display = r.Error.Error()
//	}
//	return json.Marshal(aux)
//}

// I18nError represents an error with i18n support.
type I18nError struct {
	StatusCode    int       // HTTP status code
	MessageKey    string    // used for translation, add in i18n/en/en.go
	MessageParams []string  // format arguments for the message key
	MessagePlural TransType // translation type (ignore, normal, cardinal, ordinal, range)
	InnerError    error
}

// newI18nError creates a new i18n.I18nError with the given HTTP status code, i18n key, and arguments.
func newI18nError(code int, i18n string) *I18nError {
	return &I18nError{
		StatusCode: code,
		MessageKey: i18n,
	}
}

func (e *I18nError) Error() string {
	return fmt.Sprintf("StatusCode: %d, MessageKey: %s, MessageParams: %v, MessagePlural: %d, InnerError: %v",
		e.StatusCode, e.MessageKey, e.MessageParams, e.MessagePlural, e.InnerError)
}

// WithParams sets the params for the i18n.I18nError.
func (e *I18nError) WithParams(params ...any) *I18nError {
	strs := make([]string, len(params))
	for i, p := range params {
		switch v := p.(type) {
		case string:
			strs[i] = v
		case fmt.Stringer: // 支持实现了 String() 的类型
			strs[i] = v.String()
		default:
			strs[i] = fmt.Sprint(v) // 兜底转换
		}
	}
	e.MessageParams = strs
	return e
}

// WithPlural sets the plural/trans type for the i18n.I18nError.
func (e *I18nError) WithPlural(plural TransType) *I18nError {
	e.MessagePlural = plural
	return e
}

// WithError sets the inner error for the i18n.I18nError.
func (e *I18nError) WithError(err error) *I18nError {
	e.InnerError = err
	return e
}

type Callback func(key interface{}, num float64, digits uint64, param string) (string, error)

func (e *I18nError) transCO(trans ut.Translator, callback Callback) (string, error) {
	if len(e.MessageParams) < 3 {
		return fmt.Sprintf("'%s' has 3 params in %s", e.MessageKey, trans.Locale()), nil
	}
	num, _ := strconv.ParseFloat(e.MessageParams[0], 64)
	digits, _ := strconv.ParseUint(e.MessageParams[1], 10, 32)
	return callback(e.MessageKey, num, digits, e.MessageParams[2])
}

func (e *I18nError) Translate(trans ut.Translator) any {
	var message string
	var err error
	switch e.MessagePlural {
	case TransNormal:
		message, err = trans.T(e.MessageKey, e.MessageParams...)
	case TransCardinal:
		message, err = e.transCO(trans, trans.C)
	case TransOrdinal:
		message, err = e.transCO(trans, trans.O)
	case TransRange:
		if len(e.MessageParams) < 6 {
			message = fmt.Sprintf("'%s' has 6 params in %s", e.MessageKey, trans.Locale())
		} else {
			num1, _ := strconv.ParseFloat(e.MessageParams[0], 64)
			digits1, _ := strconv.ParseUint(e.MessageParams[1], 10, 32)
			num2, _ := strconv.ParseFloat(e.MessageParams[2], 64)
			digits2, _ := strconv.ParseUint(e.MessageParams[3], 10, 32)
			message, err = trans.R(e.MessageKey, num1, digits1, num2, digits2, e.MessageParams[4], e.MessageParams[5])
		}
	case TransIgnore:
		fallthrough
	default:
		message = strings.Join(e.MessageParams, "\n")
	}
	if err != nil && message == "" {
		message = fmt.Sprintf("'%s' not found in %s", e.MessageKey, trans.Locale())
	}
	response := ErrorResponse{Code: e.MessageKey, Message: message}
	if e.InnerError != nil {
		response.Error = e.InnerError.Error()
	}
	return response
}

// NewBadRequest 400
func NewBadRequest(i18n string) *I18nError {
	return newI18nError(http.StatusBadRequest, i18n)
}

// NewUnauthorized 401
func NewUnauthorized(i18n string) *I18nError {
	return newI18nError(http.StatusUnauthorized, i18n)
}

// NewForbidden 403
func NewForbidden(i18n string) *I18nError {
	return newI18nError(http.StatusForbidden, i18n)
}

// NewNotFound 404
func NewNotFound(i18n string) *I18nError {
	return newI18nError(http.StatusNotFound, i18n)
}

// NewRequestTimeout 408
func NewRequestTimeout(i18n string) *I18nError {
	return newI18nError(http.StatusRequestTimeout, i18n)
}

// NewConflict 409
func NewConflict(i18n string) *I18nError {
	return newI18nError(http.StatusConflict, i18n)
}

// NewInternalServerError 500
func NewInternalServerError(i18n string) *I18nError {
	return newI18nError(http.StatusInternalServerError, i18n)
}

// NewServiceUnavailable 503
func NewServiceUnavailable(i18n string) *I18nError {
	return newI18nError(http.StatusServiceUnavailable, i18n)
}
