// Package errors maps store and validation errors to HTTP status codes and
// i18n message keys, and defines the business error codes.
package errors

const (
	// CodeOK is the success code carried by every 2xx envelope.
	CodeOK = 0
	// CodeBadRequest is a malformed request that is not a field validation failure.
	CodeBadRequest = 40000
	// CodeValidation is a field-level validation failure.
	CodeValidation = 40001
	// CodeUnauthorized is a missing or unusable credential.
	CodeUnauthorized = 40100
	// CodeInvalidLogin is a wrong username or password. It is separate from CodeUnauthorized so the client can distinguish "log in again" from "your session expired".
	CodeInvalidLogin = 40101
	// CodeAccessExpired means the access token is missing, expired or invalid;
	// the client should refresh instead of sending the user back to the login
	// page.
	CodeAccessExpired = 40110
	// CodeNeedRelogin means the session cannot be recovered (the refresh token
	// is gone or revoked); the client should silently re-login.
	CodeNeedRelogin = 40111
	// CodeAccountDisabled is a correct credential on a disabled or locked account.
	CodeAccountDisabled = 40301
	// CodeForbidden is a missing permission code.
	CodeForbidden = 40300
	// CodeNotFound is a missing or soft-deleted row.
	CodeNotFound = 40401
	// CodeConflict is a state conflict, such as editing a notice that is already published.
	CodeConflict = 40902
	// CodeDuplicate is a unique-constraint violation.
	CodeDuplicate = 40903
	// CodeProtected is a row the caller may not modify or delete.
	CodeProtected = 40904
	// CodeLocked is a resource held by another operation.
	CodeLocked = 42301
	// CodeFeatureDisabled is a feature switched off by configuration.
	CodeFeatureDisabled = 50301
	// CodeInternal is an unhandled server-side failure.
	CodeInternal = 50000
)
