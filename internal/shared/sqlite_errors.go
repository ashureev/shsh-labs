// Package shared provides common utilities used across the codebase.
//
//nolint:revive // "shared" is an intentional package name for cross-cutting helpers.
package shared

import "strings"

// IsSQLiteBusyError checks if the error is a SQLITE_BUSY error.
// This occurs when the database is locked by another connection.
func IsSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "SQLITE_BUSY")
}

// IsSQLiteLockedError checks if the error is a "database is locked" error.
// This is another form of SQLite concurrency error.
func IsSQLiteLockedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "database is locked")
}

// IsSQLiteConflictError checks if the error is either a SQLITE_BUSY
// or "database is locked" error. These are both SQLite concurrency
// errors that typically warrant retry logic.
func IsSQLiteConflictError(err error) bool {
	if err == nil {
		return false
	}
	return IsSQLiteBusyError(err) || IsSQLiteLockedError(err)
}
