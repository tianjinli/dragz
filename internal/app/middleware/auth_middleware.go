package middleware

import (
	"strings"

	"github.com/tianjinli/dragz/internal/i18n"
	"github.com/tianjinli/dragz/pkg/appkit"

	"github.com/gin-gonic/gin"
)

const bearerToken = "Bearer "

type JwtAuthMiddleware struct {
	JwtAuth    appkit.JwtAuthService
	Translator appkit.I18nAdapter
}

func (m *JwtAuthMiddleware) HandleAuth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.Request.Header.Get("Authorization")
		if (len(authHeader) < len(bearerToken)) || (!strings.HasPrefix(authHeader, bearerToken)) {
			ie := appkit.NewUnauthorized(i18n.ErrBaseUnauthorized)
			m.Translator.RenderError(ctx, ie)
			ctx.Abort()
			return
		}
		authToken := authHeader[len(bearerToken):]
		claims, err := m.JwtAuth.ParseAccessClaims(ctx, authToken)
		if err != nil {
			m.Translator.RenderError(ctx, err)
			ctx.Abort()
			return
		}
		appkit.StoreCustomClaims(ctx, claims)
		ctx.Next()
	}
}
