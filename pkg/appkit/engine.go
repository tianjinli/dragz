package appkit

import (
	"context"
	"net/url"

	"github.com/gin-gonic/gin"
)

type RouterService interface {
	RegisterPublic(base *gin.RouterGroup)
	RegisterProtected(base *gin.RouterGroup)
}

type EngineService interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error

	// Socks5URL returns the SOCKS5 proxy URL used for debugging.
	// It is only available when the SOCKS5 proxy is enabled.
	// Use case example: Alipay, UnionPay, WeChatPay, etc.
	Socks5URL() *url.URL

	RootGroup() *gin.RouterGroup
	PublicGroup() *gin.RouterGroup
	ProtectedGroup() *gin.RouterGroup
}
