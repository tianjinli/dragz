package httpapi

import (
	"context"
	"net/http"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/httpapi/svc"
	"dragz-admin/backend/internal/pkg/ctxkey"
	pkgi18n "dragz-admin/backend/internal/pkg/i18n"
)

func logicContext(ctx *app.Context) *svc.ServiceContext {
	return svc.NewServiceContext(ctx)
}

// localizeHook injects the request-scoped Translator into the request
// context so localized() works without any package-level state. Applied to
// every route (public and protected) by kernel.Register.
func localizeHook(ctx *app.Context) func(next http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if ctx.Translator != nil {
				r = r.WithContext(context.WithValue(r.Context(), ctxkey.Translator, ctx.Translator))
			}
			next(w, r)
		}
	}
}

func localized(r *http.Request, message string, templateData ...map[string]any) string {
	translator, _ := r.Context().Value(ctxkey.Translator).(*pkgi18n.Translator)
	if translator != nil {
		return translator.T(r, message, templateData...)
	}
	return message
}

// localizedStored renders a DB text column that holds a message ID or a
// pkgi18n.StoredMessage envelope (see i18n.StoredMessage).
func localizedStored(r *http.Request, raw string) string {
	translator, _ := r.Context().Value(ctxkey.Translator).(*pkgi18n.Translator)
	if translator != nil {
		return translator.LocalizeStored(r, raw)
	}
	return raw
}

func withUserContext(r *http.Request, userID uint64, username string) *http.Request {
	requestContext := context.WithValue(r.Context(), ctxkey.UserID, userID)
	requestContext = context.WithValue(requestContext, ctxkey.Username, username)
	return r.WithContext(requestContext)
}

func withTokenScopes(r *http.Request, scopes []string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxkey.TokenScopes, scopes))
}
