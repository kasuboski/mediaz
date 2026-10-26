package sqlite

import (
	"database/sql"
	"time"
)

// Conversion helpers for the configuration CRUD group (indexers, indexer
// sources, download clients, quality definitions/profiles/items). They bridge
// sqlc's default SQLite shapes (int64, sql.Null*) and the frozen domain shapes
// in pkg/storage/model (int32, pointers).

// cfgNullString converts an optional string into a nullable SQL value.
func cfgNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// cfgStringPtr converts a nullable SQL string into an optional string.
func cfgStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}

// cfgNullInt64FromInt32 converts an optional int32 into a nullable SQL value.
func cfgNullInt64FromInt32(v *int32) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

// cfgInt32Ptr converts a nullable SQL integer into an optional int32.
func cfgInt32Ptr(ni sql.NullInt64) *int32 {
	if !ni.Valid {
		return nil
	}
	v := int32(ni.Int64)
	return &v
}

// cfgTimePtr converts a nullable SQL timestamp into an optional time.
func cfgTimePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	t := nt.Time
	return &t
}
