package main

import (
	"strings"
	"testing"
)

func TestRunRejectsUnknownMigrationDirection(t *testing.T) {
	for _, args := range [][]string{nil, {"sideways"}, {"up", "down"}} {
		err := run(args)
		if err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Fatalf("run(%v) expected usage error, got %v", args, err)
		}
	}
}
