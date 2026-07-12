package store

import (
	"errors"
	"sync"

	"gorm.io/gorm"
)

// ErrAuditImmutable rejects application-level mutation or deletion of permanent audit history.
var ErrAuditImmutable = errors.New("audit entries are append-only")

var auditCallbackMu sync.Mutex

func installAuditImmutability(db *gorm.DB) {
	if db == nil {
		return
	}
	auditCallbackMu.Lock()
	defer auditCallbackMu.Unlock()
	if db.Callback().Update().Get("quack:audit_append_only") == nil {
		_ = db.Callback().Update().Before("gorm:update").Register("quack:audit_append_only", rejectAuditMutation)
	}
	if db.Callback().Delete().Get("quack:audit_append_only") == nil {
		_ = db.Callback().Delete().Before("gorm:delete").Register("quack:audit_append_only", rejectAuditMutation)
	}
}

func rejectAuditMutation(db *gorm.DB) {
	if db != nil && db.Statement != nil && db.Statement.Table == "audit_log_entries" {
		db.AddError(ErrAuditImmutable)
	}
}
