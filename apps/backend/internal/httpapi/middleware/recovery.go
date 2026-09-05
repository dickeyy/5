package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/httpapi/apierror"
)

// Recovery contains handler panics and emits a traceable error without dumping
// the request, cookies, body, or arbitrary panic value into operational logs.
func Recovery(c *gin.Context) {
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.ErrorContext(c.Request.Context(), "HTTP handler panicked",
				"panic_type", fmt.Sprintf("%T", recovered), "stack", string(debug.Stack()))
			apierror.Write(c, http.StatusInternalServerError, apierror.CodeInternal, "The request could not be completed")
		}
	}()
	c.Next()
}
