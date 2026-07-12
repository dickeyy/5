package platform

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/config"
	"github.com/quackdiscord/bot/internal/httpapi/apierror"
	"github.com/quackdiscord/bot/internal/httpapi/middleware"
)

// EndpointPolicy applies the documented route-class rate and replay contract
// to every dashboard-facing guild/member endpoint. Feature registrars may keep
// narrower protection; distinct classes prevent cross-feature interference.
func EndpointPolicy(primitives Primitives, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !validateEndpointQuery(c) {
			return
		}
		class, policy, applicable := endpointRatePolicy(c.Request.Method, c.Request.URL.Path, cfg)
		if !applicable {
			c.Next()
			return
		}
		if !applyEndpointRate(c, primitives.RateLimits, class, policy, cfg.Auth.SessionCookieName) {
			return
		}
		if class == "case-create" {
			evidence := cfg.RateLimits.Evidence
			if !applyEndpointRate(c, primitives.RateLimits, "evidence", RateLimit{Maximum: evidence.Maximum, Window: time.Duration(evidence.WindowSeconds) * time.Second}, cfg.Auth.SessionCookieName) {
				return
			}
		}
		if isEndpointWrite(c.Request.Method) {
			protect := primitives.Idempotency.Protect("dashboard-write:"+class, time.Duration(cfg.RateLimits.IdempotencyTTLHours)*time.Hour, func(c *gin.Context) string {
				return endpointWriteSubject(c, cfg.Auth.SessionCookieName)
			})
			protect(c)
			return
		}
		c.Next()
	}
}

// validateEndpointQuery applies one defensive pagination boundary before any
// list handler or optional module can silently coerce malformed input.
func validateEndpointQuery(c *gin.Context) bool {
	if c.Request.Method != http.MethodGet {
		return true
	}
	if raw, exists := c.GetQuery("limit"); exists {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			apierror.Write(c, http.StatusBadRequest, apierror.CodeValidation, "limit must be between 1 and 100")
			return false
		}
	}
	if raw, exists := c.GetQuery("offset"); exists {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 || offset > 100000 {
			apierror.Write(c, http.StatusBadRequest, apierror.CodeValidation, "offset must be between 0 and 100000")
			return false
		}
	}
	for _, name := range []string{"before_id", "cursor"} {
		if raw, exists := c.GetQuery(name); exists && len(raw) > 256 {
			apierror.Write(c, http.StatusBadRequest, apierror.CodeValidation, name+" is too long")
			return false
		}
	}
	return true
}

// applyEndpointRate consumes one route-class allowance before any feature or
// idempotency handler executes.
func applyEndpointRate(c *gin.Context, limiter *RateLimiter, class string, policy RateLimit, cookieName string) bool {
	decision, err := limiter.Allow(c.Request.Context(), class+":"+endpointSubject(c, cookieName), policy)
	if err != nil {
		if errors.Is(err, ErrUnavailable) {
			apierror.Write(c, http.StatusServiceUnavailable, apierror.CodeDependency, "rate-limit service unavailable")
		} else {
			apierror.Write(c, http.StatusInternalServerError, apierror.CodeInternal, "rate-limit configuration failed")
		}
		return false
	}
	c.Header("RateLimit-Limit", strconv.Itoa(policy.Maximum))
	c.Header("RateLimit-Remaining", strconv.Itoa(decision.Remaining))
	if !decision.Allowed {
		retrySeconds := int(decision.RetryAfter.Round(time.Second).Seconds())
		if retrySeconds < 1 {
			retrySeconds = 1
		}
		c.Header("Retry-After", strconv.Itoa(retrySeconds))
		apierror.Write(c, http.StatusTooManyRequests, apierror.CodeRateLimited, "rate limit exceeded")
		return false
	}
	return true
}

// endpointRatePolicy maps every applicable route to one configured class.
func endpointRatePolicy(method, path string, cfg config.Config) (string, RateLimit, bool) {
	if !strings.HasPrefix(path, "/guilds") && !strings.HasPrefix(path, "/members/me") {
		return "", RateLimit{}, false
	}
	selected := cfg.RateLimits.MemberRead
	class := "member-read"
	if isEndpointWrite(method) {
		selected = cfg.RateLimits.TemplateWrite
		class = "authenticated-write"
	}
	if method == http.MethodPost && strings.Contains(path, "/cases") && !strings.Contains(path, "/appeal") {
		selected = cfg.RateLimits.CaseCreate
		class = "case-create"
	}
	if strings.Contains(path, "/retry") || strings.Contains(path, "/reversals") {
		selected = cfg.RateLimits.Retry
		class = "action-recovery"
	}
	if strings.Contains(path, "/evidence") {
		selected = cfg.RateLimits.Evidence
		class = "evidence"
	}
	return class, RateLimit{Maximum: selected.Maximum, Window: time.Duration(selected.WindowSeconds) * time.Second}, true
}

// endpointSubject keeps credentials within the shared hashed Redis boundary.
func endpointSubject(c *gin.Context, cookieName string) string {
	credential := middleware.ExtractSessionID(c, cookieName)
	return credential + ":" + c.Param("discordGuildID") + ":" + c.Param("guildID")
}

// endpointWriteSubject prevents replay across methods and route templates.
func endpointWriteSubject(c *gin.Context, cookieName string) string {
	return endpointSubject(c, cookieName) + ":" + c.Request.Method + ":" + c.FullPath()
}

// isEndpointWrite identifies browser and adapter mutations, including PUT.
func isEndpointWrite(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
