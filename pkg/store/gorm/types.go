package gormstore

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONRawMessage wraps json.RawMessage to implement sql.Scanner and driver.Valuer.
// Handles cross-database JSON storage:
// - PostgreSQL (pgx): returns string from Scan, expects string from Value
// - MySQL/SQLite: returns []byte from Scan, accepts []byte or string from Value
// - MSSQL: returns []byte from Scan, requires string from Value (rejects varbinary)
type JSONRawMessage json.RawMessage

// Scan implements sql.Scanner for database deserialization.
func (j *JSONRawMessage) Scan(value interface{}) error {
	if value == nil {
		*j = JSONRawMessage(`{}`)
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("gormstore: cannot scan %T into JSONRawMessage", value)
	}
	if len(bytes) == 0 {
		*j = JSONRawMessage(`{}`)
		return nil
	}
	if !json.Valid(bytes) {
		return fmt.Errorf("gormstore: invalid JSON: %s", string(bytes))
	}
	*j = bytes
	return nil
}

// Value implements driver.Valuer for database serialization.
// Returns string for cross-database compatibility (MSSQL rejects varbinary in text columns).
func (j JSONRawMessage) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return string(j), nil
}
