package platform

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/httpapi/apierror"
)

const idempotencyKeyHeader = "Idempotency-Key"

// captureWriter delays a feature response until its idempotency result is durably recorded.
type captureWriter struct {
	gin.ResponseWriter
	status int
	body   bytes.Buffer
}

// WriteHeader records the intended status without sending it before idempotency completion.
func (w *captureWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

// WriteHeaderNow records the implicit successful status.
func (w *captureWriter) WriteHeaderNow() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
}

// Write buffers handler output until Redis records the original result.
func (w *captureWriter) Write(body []byte) (int, error) {
	w.WriteHeaderNow()
	return w.body.Write(body)
}

// WriteString buffers handler text until Redis records the original result.
func (w *captureWriter) WriteString(body string) (int, error) {
	w.WriteHeaderNow()
	return w.body.WriteString(body)
}

// Status reports the buffered status.
func (w *captureWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// Size reports the buffered response length.
func (w *captureWriter) Size() int { return w.body.Len() }

// Written reports whether a handler selected a status or wrote a body.
func (w *captureWriter) Written() bool { return w.status != 0 || w.body.Len() > 0 }

// SubjectFunc returns a non-secret actor or guild scope used only as hashed limiter input.
type SubjectFunc func(*gin.Context) string

// Limit installs a fail-closed rate limit for one endpoint class.
func (l *RateLimiter) Limit(class string, limit RateLimit, subject SubjectFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(class) == "" || subject == nil {
			apierror.Write(c, http.StatusInternalServerError, apierror.CodeInternal, "rate-limit configuration failed")
			return
		}
		decision, err := l.Allow(c.Request.Context(), class+":"+subject(c), limit)
		if err != nil {
			if errors.Is(err, ErrUnavailable) {
				apierror.Write(c, http.StatusServiceUnavailable, apierror.CodeDependency, "rate-limit service unavailable")
				return
			}
			apierror.Write(c, http.StatusInternalServerError, apierror.CodeInternal, "rate-limit configuration failed")
			return
		}
		c.Header("RateLimit-Limit", strconv.Itoa(limit.Maximum))
		c.Header("RateLimit-Remaining", strconv.Itoa(decision.Remaining))
		if !decision.Allowed {
			retrySeconds := int(decision.RetryAfter.Round(time.Second).Seconds())
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(retrySeconds))
			apierror.Write(c, http.StatusTooManyRequests, apierror.CodeRateLimited, "rate limit exceeded")
			return
		}
		c.Next()
	}
}

// ClientIPSubject scopes public OAuth limits to Gin's canonical client address.
func ClientIPSubject(c *gin.Context) string {
	if c == nil {
		return "unknown"
	}
	return fmt.Sprintf("ip:%s", c.ClientIP())
}

// Protect requires an idempotency key and returns the original/in-progress result without executing a write twice.
func (s *IdempotencyStore) Protect(class string, ttl time.Duration, subject SubjectFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(class) == "" || subject == nil || ttl <= 0 {
			apierror.Write(c, http.StatusInternalServerError, apierror.CodeInternal, "idempotency configuration failed")
			return
		}
		key := strings.TrimSpace(c.GetHeader(idempotencyKeyHeader))
		if key == "" || len(key) > 256 {
			apierror.Write(c, http.StatusBadRequest, apierror.CodeValidation, "a valid Idempotency-Key header is required")
			return
		}
		scope := class + ":" + subject(c) + ":" + c.Request.Method + ":" + c.Request.URL.EscapedPath()
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, (4<<20)+1))
		if err != nil || len(body) > 4<<20 {
			apierror.Write(c, http.StatusBadRequest, apierror.CodeValidation, "request body is unavailable or too large")
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		digest := sha256.Sum256(append([]byte(c.Request.URL.RawQuery+"\x00"+c.ContentType()+"\x00"), body...))
		result, err := s.Begin(c.Request.Context(), scope, key, ttl, hex.EncodeToString(digest[:]))
		if err != nil {
			if errors.Is(err, ErrUnavailable) {
				apierror.Write(c, http.StatusServiceUnavailable, apierror.CodeDependency, "idempotency service unavailable")
				return
			}
			apierror.Write(c, http.StatusInternalServerError, apierror.CodeInternal, "idempotency configuration failed")
			return
		}
		switch result.State {
		case IdempotencyConflict:
			apierror.Write(c, http.StatusConflict, apierror.CodeConflict, "Idempotency-Key was already used with a different request")
			return
		case IdempotencyInProgress:
			c.Header("Retry-After", "1")
			apierror.Write(c, http.StatusConflict, apierror.CodeConflict, "an identical request is still in progress")
			return
		case IdempotencyComplete:
			c.Abort()
			c.Header("Idempotency-Replayed", "true")
			c.Header("Content-Type", "application/json; charset=utf-8")
			c.Status(result.StatusCode)
			_, _ = c.Writer.Write(result.Body)
			return
		case IdempotencyAcquired:
			// Continue below as the single lease owner.
		default:
			apierror.Write(c, http.StatusServiceUnavailable, apierror.CodeDependency, "idempotency service unavailable")
			return
		}

		original := c.Writer
		captured := &captureWriter{ResponseWriter: original}
		c.Writer = captured
		defer func() { c.Writer = original }()
		c.Next()
		c.Writer = original
		if err := s.Complete(c.Request.Context(), scope, key, result.LeaseToken, captured.Status(), captured.body.Bytes(), ttl); err != nil {
			apierror.Write(c, http.StatusServiceUnavailable, apierror.CodeDependency, "idempotency result could not be recorded")
			return
		}
		original.WriteHeader(captured.Status())
		if captured.body.Len() > 0 {
			_, _ = original.Write(captured.body.Bytes())
		}
	}
}
