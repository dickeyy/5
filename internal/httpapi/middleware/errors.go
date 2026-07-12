package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/httpapi/apierror"
	"github.com/quackdiscord/bot/internal/quack"
)

// bufferedResponseWriter delays a response until the error normalizer can replace unsafe legacy bodies.
type bufferedResponseWriter struct {
	gin.ResponseWriter
	status int
	body   bytes.Buffer
}

// WriteHeader records the intended status without exposing the body to the client yet.
func (w *bufferedResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

// WriteHeaderNow records an implicit successful status.
func (w *bufferedResponseWriter) WriteHeaderNow() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
}

// Write buffers response bytes for safe normalization.
func (w *bufferedResponseWriter) Write(body []byte) (int, error) {
	w.WriteHeaderNow()
	return w.body.Write(body)
}

// WriteString buffers response text for safe normalization.
func (w *bufferedResponseWriter) WriteString(body string) (int, error) {
	w.WriteHeaderNow()
	return w.body.WriteString(body)
}

// Status reports the recorded response status.
func (w *bufferedResponseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// Size reports the buffered response size.
func (w *bufferedResponseWriter) Size() int { return w.body.Len() }

// Written reports whether the handler has selected a response status or body.
func (w *bufferedResponseWriter) Written() bool { return w.status != 0 || w.body.Len() > 0 }

// ErrorEnvelope normalizes every failure into Quack's stable error contract and discards unsafe legacy details.
func ErrorEnvelope(c *gin.Context) {
	original := c.Writer
	buffered := &bufferedResponseWriter{ResponseWriter: original}
	c.Writer = buffered
	c.Next()

	status := buffered.Status()
	body := buffered.body.Bytes()
	if status >= http.StatusBadRequest {
		var structured apierror.Response
		if err := json.Unmarshal(body, &structured); err != nil || structured.Error.Code == "" {
			code, message := apierror.Default(status)
			requestID, correlationID := quack.TraceIDsFromContext(c.Request.Context())
			structured = apierror.Response{Error: apierror.Detail{
				Code:          code,
				Message:       message,
				RequestID:     requestID,
				CorrelationID: correlationID,
			}}
		}
		body, _ = json.Marshal(structured)
		original.Header().Set("Content-Type", "application/json; charset=utf-8")
	}

	original.Header().Del("Content-Length")
	original.WriteHeader(status)
	if len(body) > 0 {
		_, _ = original.Write(body)
	}
}
