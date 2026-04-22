package i18n

import "embed"

//go:embed locales/*
var LocalesFS embed.FS

const RootNodeName = "dragz"

const (
	// ErrBaseUnknownError unknown error
	ErrBaseUnknownError = "dragz.base.unknown_error"
	// ErrBaseUnauthorized invalid authorization
	ErrBaseUnauthorized = "dragz.base.unauthorized"
	// ErrBaseInvalidParams request params is invalid
	ErrBaseInvalidParams = "dragz.base.invalid_params"
	// ErrBaseBadRequest bad request error
	ErrBaseBadRequest = "dragz.base.bad_request"
	// ErrBaseQrcodeExpired qrcode is expired
	ErrBaseQrcodeExpired = "dragz.base.qrcode_expired"
	// ErrBaseLocalhostOnly indicates localhost-only access
	ErrBaseLocalhostOnly = "dragz.base.localhost_only"
)

const (
	// ErrTokenMalformed token is malformed
	ErrTokenMalformed = "dragz.token.malformed"
	// ErrTokenNotValidYet token is not valid yet
	ErrTokenNotValidYet = "dragz.token.not_valid_yet"
	// ErrTokenExpired token is expired
	ErrTokenExpired = "dragz.token.expired"
	// ErrTokenUnverifiable token is unverifiable
	ErrTokenUnverifiable = "dragz.token.unverifiable"
	// ErrTokenSignatureInvalid token signature is invalid
	ErrTokenSignatureInvalid = "dragz.token.signature_invalid"
	// ErrTokenInvalidClaims token has invalid claims
	ErrTokenInvalidClaims = "dragz.token.invalid_claims"
	// ErrTokenInvalidIssuer token has invalid issuer
	ErrTokenInvalidIssuer = "dragz.token.invalid_issuer"
	// ErrTokenInvalid token is invalid
	ErrTokenInvalid = "dragz.token.invalid"
)
