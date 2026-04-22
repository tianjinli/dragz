package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gin-contrib/gzip"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
	"github.com/tianjinli/dragz/internal/app/middleware"
	"github.com/tianjinli/dragz/internal/bootstrap/tunnel"
	"github.com/tianjinli/dragz/internal/i18n"
	"github.com/tianjinli/dragz/pkg/appkit"
	"go.uber.org/zap"
)

const ISO8601Time = "2006-01-02T15:04:05.000Z0700"

type engineService struct {
	logger         *zap.Logger
	server         *http.Server
	rootGroup      *gin.RouterGroup
	publicGroup    *gin.RouterGroup
	protectedGroup *gin.RouterGroup
	sshExpose      *tunnel.SshExpose
	sshSocks5      *tunnel.SshSocks5
}

func NewEngineService(
	logger *zap.Logger,
	conf *appkit.ServerConfig,
	config *appkit.TokenConfig,
	middle *middleware.JwtAuthMiddleware,
) (appkit.EngineService, func(), error) {
	var loggerFunc, recoveryFunc gin.HandlerFunc
	if appkit.Debug {
		gin.SetMode(gin.DebugMode)
		loggerFunc = gin.Logger()
		recoveryFunc = gin.Recovery()
	} else {
		gin.SetMode(gin.ReleaseMode)
		loggerFunc = ginzap.Ginzap(logger, ISO8601Time, true)
		recoveryFunc = ginzap.RecoveryWithZap(logger, true)
	}
	webEngine := gin.New()
	webEngine.ForwardedByClientIP = true
	webEngine.Use(loggerFunc, recoveryFunc)
	webEngine.Use(gzip.Gzip(gzip.DefaultCompression))
	listenAddr := fmt.Sprintf(":%d", conf.Port)

	handleVersion := func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"version": appkit.Version,
			"author":  appkit.Author,
			"debug":   appkit.Debug,
			"name":    appkit.Name,
			"time":    time.Now(),
			"realip":  ctx.ClientIP(),
		})
	}
	webEngine.GET("/", handleVersion)
	handleGenerate := func(ctx *gin.Context) {
		ip := ctx.ClientIP()
		if ip != "127.0.0.1" && ip != "::1" {
			ie := appkit.NewForbidden(i18n.ErrBaseLocalhostOnly)
			middle.Translator.RenderError(ctx, ie)
			return
		}

		var claims jwt.MapClaims
		if err := ctx.ShouldBindJSON(&claims); err != nil {
			middle.Translator.RenderError(ctx, err)
			return
		}
		claims["env"] = appkit.Name
		jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		accessToken, err := jwtToken.SignedString([]byte(config.AccessSecretKey))
		if err != nil {
			middle.Translator.RenderError(ctx, err)
			return
		}
		logger.Warn("Permanent access token created", zap.Any("claims", claims))
		ctx.String(http.StatusOK, accessToken)
	}
	if appkit.Debug && conf.TokenPath != "" {
		webEngine.POST(conf.TokenPath, handleGenerate)
	}

	rootGroup := webEngine.Group(conf.BasePath)
	publicGroup := rootGroup.Group("/")
	protectedGroup := rootGroup.Group("/")
	protectedGroup.Use(middle.HandleAuth())
	engine := &engineService{
		logger: logger,
		server: &http.Server{
			Addr:    listenAddr,
			Handler: webEngine,
		},
		rootGroup:      rootGroup,
		publicGroup:    publicGroup,
		protectedGroup: protectedGroup,
	}
	cleanup := func() {
		if engine.sshExpose != nil {
			_ = engine.sshExpose.Close()
		}
		if engine.sshSocks5 != nil {
			_ = engine.sshSocks5.Close()
		}
	}
	for _, item := range conf.Public {
		fi, err := os.Stat(item.FSPath)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			publicGroup.Static(item.URIPath, item.FSPath)
		} else {
			publicGroup.StaticFile(item.URIPath, item.FSPath)
		}
	}
	for _, item := range conf.Protected {
		fi, err := os.Stat(item.FSPath)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			protectedGroup.Static(item.URIPath, item.FSPath)
		} else {
			protectedGroup.StaticFile(item.URIPath, item.FSPath)
		}
	}
	var err error
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err = middle.Translator.RegisterTranslations(v); err != nil {
			return nil, cleanup, errors.WithStack(err)
		}
		// resolve translations from embedded locales
		if err = middle.Translator.ResolveTranslations("", i18n.RootNodeName); err != nil {
			return nil, cleanup, errors.WithStack(err)
		}
	}
	if conf.Expose != nil {
		engine.sshExpose, err = tunnel.NewSshExpose(conf.Expose, conf.Port)
		if err != nil {
			return nil, cleanup, errors.WithStack(err)
		}
	}
	if conf.Socks5 != nil {
		engine.sshSocks5, err = tunnel.NewSshSocks5(conf.Socks5)
		if err != nil {
			return nil, cleanup, errors.WithStack(err)
		}
	}

	return engine, cleanup, errors.WithStack(err)
}

func (s *engineService) ListenAndServe() error {
	s.logger.Info("Server listening on", zap.String("addr", s.server.Addr))
	return s.server.ListenAndServe()
}

func (s *engineService) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *engineService) Socks5URL() *url.URL {
	if s.sshSocks5 == nil || s.sshSocks5.Addr() == nil {
		return nil
	}
	proxyAddr := s.sshSocks5.Addr().String()
	proxyURL, err := url.Parse("socks5://" + proxyAddr)
	if err != nil {
		return nil
	}

	return proxyURL
}

func (s *engineService) RootGroup() *gin.RouterGroup {
	return s.rootGroup
}

func (s *engineService) PublicGroup() *gin.RouterGroup {
	return s.publicGroup
}

func (s *engineService) ProtectedGroup() *gin.RouterGroup {
	return s.protectedGroup
}
