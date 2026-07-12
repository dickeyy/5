package model

import (
	"strings"
	"testing"
)

func TestOperationalRedactionRecursesAcrossCredentialsContentAndPayloads(t *testing.T) {
	raw := `{"safe":"count","nested":{"oauth_token":"oauth-secret","cookie":"cookie-secret","session_id":"session-secret","webhook_url":"https://discord.invalid/webhook-secret","member_content":"private words","action_payload":{"reason":"private payload"}},"items":[{"authorization":"Bearer secret"}]}`
	redacted := RedactAuditMetadata(raw)
	for _, secret := range []string{"oauth-secret", "cookie-secret", "session-secret", "webhook-secret", "private words", "private payload", "Bearer secret"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("redaction exposed %q in %s", secret, redacted)
		}
	}
	if !strings.Contains(redacted, `"safe":"count"`) {
		t.Fatalf("redaction removed safe aggregate field: %s", redacted)
	}
}
