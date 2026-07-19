package testutil

import (
	"testing"
)

// SetTestConfig encapsulates the set test config rule so callers share one consistent package implementation.
func SetTestConfig(t testing.TB) {
	t.Helper()

}
