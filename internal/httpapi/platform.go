package httpapi

import (
	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/config"
	"github.com/quackdiscord/bot/internal/httpapi/middleware"
)

// PlatformRegistrar validates and installs the shared HTTP security and contract middleware for integration-owned routers.
type PlatformRegistrar struct {
	cfg config.Config
}

// NewPlatformRegistrar constructs the reusable QP-B platform registrar.
func NewPlatformRegistrar(cfg config.Config) (*PlatformRegistrar, error) {
	if err := middleware.ValidateSecurityConfig(cfg); err != nil {
		return nil, err
	}
	return &PlatformRegistrar{cfg: cfg}, nil
}

// Register installs middleware in the required order before feature routes are registered.
func (p *PlatformRegistrar) Register(r *gin.Engine) {
	r.Use(middleware.RequestContext)
	r.Use(middleware.ErrorEnvelope)
	r.Use(gin.Recovery())
	r.Use(middleware.Logger)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.CORS(p.cfg.API.CORSAllowedOrigins))
	r.Use(middleware.BodyLimit(p.cfg.API.MaxBodyBytes))
	r.Use(middleware.CSRF(p.cfg.Auth, p.cfg.API.CORSAllowedOrigins))
}
