package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringSlice is a []string stored as JSON text in SQLite/PostgreSQL TEXT columns.
type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("marshal StringSlice: %w", err)
	}
	return string(b), nil
}

func (s *StringSlice) Scan(src interface{}) error {
	if src == nil {
		*s = nil
		return nil
	}
	var raw string
	switch v := src.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("unsupported type for StringSlice: %T", src)
	}
	if raw == "" {
		*s = nil
		return nil
	}
	return json.Unmarshal([]byte(raw), s)
}
