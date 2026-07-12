package main

import "testing"

func TestCheckScopeRejectsCollisionAndPostMigrationDirectCommands(t *testing.T) {
	if err := checkScope([]string{"--v4", "case,warn", "--v5", "case"}); err == nil {
		t.Fatal("expected command collision")
	}
	if err := checkScope([]string{"--v4", "warn", "--v5", "case", "--after-migration"}); err == nil {
		t.Fatal("expected legacy command rejection")
	}
	if err := checkScope([]string{"--v4", "ticket", "--v5", "case"}); err != nil {
		t.Fatalf("unexpected isolated scopes failure: %v", err)
	}
}
