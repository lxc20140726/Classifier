package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("not found")

func boolToInt(v bool) int {
	if v {
		return 1
	}

	return 0
}

func intToBool(v int) bool {
	return v != 0
}

func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}

	return sql.NullString{String: s, Valid: true}
}

func nullableJSON(v json.RawMessage) sql.NullString {
	if len(v) == 0 {
		return sql.NullString{}
	}

	return sql.NullString{String: string(v), Valid: true}
}

func parseDBTime(v any) (time.Time, error) {
	switch t := v.(type) {
	case time.Time:
		return t.UTC(), nil
	case string:
		return parseTimeString(t)
	case []byte:
		return parseTimeString(string(t))
	case nil:
		return time.Time{}, nil
	default:
		return time.Time{}, fmt.Errorf("parseDBTime unsupported type: %T", v)
	}
}

func parseTimeString(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("parseTimeString unsupported format: %q", value)
}
