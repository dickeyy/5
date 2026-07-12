// Package apierror defines Quack's stable, safe HTTP error contract.
package apierror

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/quack"
)

// Code is a stable machine-readable HTTP failure classification.
type Code string

const (
	CodeValidation     Code = "validation_failed"
	CodeAuthentication Code = "authentication_required"
	CodeReauthenticate Code = "reauthentication_required"
	CodeAuthorization  Code = "authorization_denied"
	CodeNotFound       Code = "not_found"
	CodeConflict       Code = "conflict"
	CodeRateLimited    Code = "rate_limited"
	CodeCSRF           Code = "csrf_rejected"
	CodeOrigin         Code = "origin_rejected"
	CodeBodyTooLarge   Code = "body_too_large"
	CodeDependency     Code = "dependency_unavailable"
	CodeInternal       Code = "internal_error"
)

// Response is the stable top-level error envelope returned by every HTTP adapter.
type Response struct {
	Error Detail `json:"error"`
}

// Detail contains safe client-facing failure data and trace identifiers.
type Detail struct {
	Code          Code   `json:"code"`
	Message       string `json:"message"`
	RequestID     string `json:"request_id"`
	CorrelationID string `json:"correlation_id"`
}

// Write terminates a Gin request with a safe structured error response.
func Write(c *gin.Context, status int, code Code, message string) {
	requestID, correlationID := quack.TraceIDsFromContext(c.Request.Context())
	c.AbortWithStatusJSON(status, Response{Error: Detail{
		Code:          code,
		Message:       message,
		RequestID:     requestID,
		CorrelationID: correlationID,
	}})
}

// Default returns a stable code and safe message for a legacy status response.
func Default(status int) (Code, string) {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return CodeValidation, "request validation failed"
	case http.StatusUnauthorized:
		return CodeAuthentication, "authentication required"
	case http.StatusForbidden:
		return CodeAuthorization, "access denied"
	case http.StatusNotFound:
		return CodeNotFound, "resource not found"
	case http.StatusConflict:
		return CodeConflict, "request conflicts with current state"
	case http.StatusRequestEntityTooLarge:
		return CodeBodyTooLarge, "request body is too large"
	case http.StatusTooManyRequests:
		return CodeRateLimited, "rate limit exceeded"
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return CodeDependency, "dependency unavailable"
	default:
		return CodeInternal, "request failed"
	}
}
