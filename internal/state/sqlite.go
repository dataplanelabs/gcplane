package state

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using a local SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens or creates a SQLite state database at the given path.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open state db %s: %w", path, err)
	}

	if err := createTable(db); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLiteStore{db: db}, nil
}

func createTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS resource_state (
			kind TEXT NOT NULL,
			key TEXT NOT NULL,
			external_id TEXT,
			spec_hash TEXT NOT NULL,
			synced BOOLEAN NOT NULL DEFAULT 0,
			last_sync TEXT NOT NULL,
			error TEXT,
			PRIMARY KEY (kind, key)
		)
	`)
	if err != nil {
		return fmt.Errorf("create state table: %w", err)
	}
	return nil
}

// Get returns the state for a resource, or nil if not tracked.
func (s *SQLiteStore) Get(kind, key string) (*ResourceState, error) {
	row := s.db.QueryRow(
		`SELECT kind, key, external_id, spec_hash, synced, last_sync, error
		 FROM resource_state WHERE kind = ? AND key = ?`, kind, key)

	rs := &ResourceState{}
	var lastSync string
	var errStr sql.NullString

	err := row.Scan(&rs.Kind, &rs.Key, &rs.ExternalID, &rs.SpecHash, &rs.Synced, &lastSync, &errStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get state %s/%s: %w", kind, key, err)
	}

	rs.LastSync, _ = time.Parse(time.RFC3339, lastSync)
	if errStr.Valid {
		rs.Error = errStr.String
	}
	return rs, nil
}

// Put upserts the state for a resource.
func (s *SQLiteStore) Put(rs *ResourceState) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO resource_state (kind, key, external_id, spec_hash, synced, last_sync, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rs.Kind, rs.Key, rs.ExternalID, rs.SpecHash, rs.Synced,
		rs.LastSync.Format(time.RFC3339), nullStr(rs.Error))
	if err != nil {
		return fmt.Errorf("put state %s/%s: %w", rs.Kind, rs.Key, err)
	}
	return nil
}

// List returns all tracked resource states.
func (s *SQLiteStore) List() ([]*ResourceState, error) {
	rows, err := s.db.Query(
		`SELECT kind, key, external_id, spec_hash, synced, last_sync, error FROM resource_state`)
	if err != nil {
		return nil, fmt.Errorf("list states: %w", err)
	}
	defer rows.Close()

	var states []*ResourceState
	for rows.Next() {
		rs := &ResourceState{}
		var lastSync string
		var errStr sql.NullString

		if err := rows.Scan(&rs.Kind, &rs.Key, &rs.ExternalID, &rs.SpecHash, &rs.Synced, &lastSync, &errStr); err != nil {
			return nil, fmt.Errorf("scan state row: %w", err)
		}
		rs.LastSync, _ = time.Parse(time.RFC3339, lastSync)
		if errStr.Valid {
			rs.Error = errStr.String
		}
		states = append(states, rs)
	}
	return states, rows.Err()
}

// Delete removes a tracked resource.
func (s *SQLiteStore) Delete(kind, key string) error {
	_, err := s.db.Exec(`DELETE FROM resource_state WHERE kind = ? AND key = ?`, kind, key)
	if err != nil {
		return fmt.Errorf("delete state %s/%s: %w", kind, key, err)
	}
	return nil
}

// Close releases the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
