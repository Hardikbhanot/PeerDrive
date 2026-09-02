package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

// Share represents a shared file or folder
type Share struct {
	ID        string    // Unique Share ID
	Path      string    // Local path being shared
	RootHash  string    // The root hash from the chunker
	Token     string    // Secret token for direct HTTP download
	IsActive  bool
	CreatedAt time.Time
	ExpiresAt *time.Time
}

func InitDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.createTables(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS shares (
		id TEXT PRIMARY KEY,
		path TEXT NOT NULL,
		root_hash TEXT NOT NULL,
		token TEXT NOT NULL,
		is_active BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL,
		expires_at DATETIME
	);
	
	CREATE TABLE IF NOT EXISTS authorized_peers (
		share_id TEXT NOT NULL,
		peer_id TEXT NOT NULL,
		PRIMARY KEY (share_id, peer_id),
		FOREIGN KEY (share_id) REFERENCES shares(id) ON DELETE CASCADE
	);
	`
	_, err := db.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}
	return nil
}

func (db *DB) CreateShare(share Share) error {
	query := `INSERT INTO shares (id, path, root_hash, token, is_active, created_at, expires_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?)`
	
	var expiresAt interface{}
	if share.ExpiresAt != nil {
		expiresAt = *share.ExpiresAt
	}

	_, err := db.conn.Exec(query, share.ID, share.Path, share.RootHash, share.Token, share.IsActive, share.CreatedAt, expiresAt)
	return err
}

func (db *DB) ListShares() ([]Share, error) {
	query := `SELECT id, path, root_hash, token, is_active, created_at, expires_at FROM shares ORDER BY created_at DESC`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []Share
	for rows.Next() {
		var s Share
		var expiresAt sql.NullTime
		if err := rows.Scan(&s.ID, &s.Path, &s.RootHash, &s.Token, &s.IsActive, &s.CreatedAt, &expiresAt); err != nil {
			return nil, err
		}
		if expiresAt.Valid {
			s.ExpiresAt = &expiresAt.Time
		}
		shares = append(shares, s)
	}
	return shares, nil
}

func (db *DB) GetShare(shareID string) (*Share, error) {
	query := `SELECT id, path, root_hash, token, is_active, created_at, expires_at FROM shares WHERE id = ?`
	var s Share
	var expiresAt sql.NullTime
	
	err := db.conn.QueryRow(query, shareID).Scan(&s.ID, &s.Path, &s.RootHash, &s.Token, &s.IsActive, &s.CreatedAt, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	
	if expiresAt.Valid {
		s.ExpiresAt = &expiresAt.Time
	}
	
	return &s, nil
}

func (db *DB) GetShareByToken(token string) (*Share, error) {
	query := `SELECT id, path, root_hash, token, is_active, created_at, expires_at FROM shares WHERE token = ?`
	var s Share
	var expiresAt sql.NullTime
	
	err := db.conn.QueryRow(query, token).Scan(&s.ID, &s.Path, &s.RootHash, &s.Token, &s.IsActive, &s.CreatedAt, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	
	if expiresAt.Valid {
		s.ExpiresAt = &expiresAt.Time
	}
	
	return &s, nil
}

func (db *DB) AuthorizePeer(shareID string, peerID string) error {
	query := `INSERT OR IGNORE INTO authorized_peers (share_id, peer_id) VALUES (?, ?)`
	_, err := db.conn.Exec(query, shareID, peerID)
	return err
}

func (db *DB) RevokeShare(shareID string) error {
	query := `UPDATE shares SET is_active = 0, token = '' WHERE id = ?`
	_, err := db.conn.Exec(query, shareID)
	return err
}

func (db *DB) IsPeerAuthorized(shareID string, peerID string) (bool, error) {
	query := `SELECT COUNT(1) FROM authorized_peers WHERE share_id = ? AND peer_id = ?`
	var count int
	err := db.conn.QueryRow(query, shareID, peerID).Scan(&count)
	if err != nil {
		return false, err
	}
	
	// Also check if share is active and not expired
	activeQuery := `SELECT is_active, expires_at FROM shares WHERE id = ?`
	var isActive bool
	var expiresAt sql.NullTime
	err = db.conn.QueryRow(activeQuery, shareID).Scan(&isActive, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // Share doesn't exist
		}
		return false, err
	}
	
	if !isActive {
		return false, nil
	}
	
	if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
		return false, nil // Expired
	}

	return count > 0, nil
}
