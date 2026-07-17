package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type ServerInfo struct {
	ID        int64
	Name      string
	Token     string
	CreatedAt time.Time
}

type ChatInfo struct {
	ChatID    int64
	Title     string
	CreatedAt time.Time
}

type DBStore struct {
	db *sql.DB
}

func InitDB(dbPath string) (*DBStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode and foreign keys for better SQLite performance/behavior
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set database pragmas: %w", err)
	}

	// Create tables
	queries := []string{
		`CREATE TABLE IF NOT EXISTS servers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			token TEXT UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS authorized_chats (
			chat_id INTEGER PRIMARY KEY,
			chat_title TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to run migration query: %w", err)
		}
	}

	return &DBStore{db: db}, nil
}

func (s *DBStore) Close() error {
	return s.db.Close()
}

func (s *DBStore) AddServer(name, token string) error {
	query := `INSERT INTO servers (name, token) VALUES (?, ?)`
	_, err := s.db.Exec(query, name, token)
	if err != nil {
		return fmt.Errorf("failed to add server: %w", err)
	}
	return nil
}

func (s *DBStore) RemoveServer(name string) error {
	query := `DELETE FROM servers WHERE name = ?`
	result, err := s.db.Exec(query, name)
	if err != nil {
		return fmt.Errorf("failed to remove server: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *DBStore) RegenerateServerToken(name, token string) error {
	query := `UPDATE servers SET token = ? WHERE name = ?`
	result, err := s.db.Exec(query, token, name)
	if err != nil {
		return fmt.Errorf("failed to regenerate server token: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *DBStore) RenameServer(oldName, newName string) error {
	query := `UPDATE servers SET name = ? WHERE name = ?`
	result, err := s.db.Exec(query, newName, oldName)
	if err != nil {
		return fmt.Errorf("failed to rename server: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *DBStore) GetServerByToken(token string) (string, error) {
	query := `SELECT name FROM servers WHERE token = ?`
	var name string
	err := s.db.QueryRow(query, token).Scan(&name)
	if err != nil {
		return "", err
	}
	return name, nil
}

func (s *DBStore) ListServers() ([]ServerInfo, error) {
	query := `SELECT id, name, token, created_at FROM servers ORDER BY name ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query servers: %w", err)
	}
	defer rows.Close()

	var servers []ServerInfo
	for rows.Next() {
		var s ServerInfo
		var createdAtStr string
		if err := rows.Scan(&s.ID, &s.Name, &s.Token, &createdAtStr); err != nil {
			return nil, err
		}
		// SQLite returns time as string. Try parsing it.
		t, err := time.Parse("2006-01-02 15:04:05", createdAtStr)
		if err == nil {
			s.CreatedAt = t
		} else {
			// fallback attempt if RFC3339 or other format is used
			t, err = time.Parse(time.RFC3339, createdAtStr)
			if err == nil {
				s.CreatedAt = t
			}
		}
		servers = append(servers, s)
	}
	return servers, nil
}

func (s *DBStore) AddChat(chatID int64, title string) error {
	query := `INSERT OR REPLACE INTO authorized_chats (chat_id, chat_title) VALUES (?, ?)`
	_, err := s.db.Exec(query, chatID, title)
	if err != nil {
		return fmt.Errorf("failed to add authorized chat: %w", err)
	}
	return nil
}

func (s *DBStore) RemoveChat(chatID int64) error {
	query := `DELETE FROM authorized_chats WHERE chat_id = ?`
	result, err := s.db.Exec(query, chatID)
	if err != nil {
		return fmt.Errorf("failed to remove authorized chat: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *DBStore) ListChats() ([]ChatInfo, error) {
	query := `SELECT chat_id, chat_title, created_at FROM authorized_chats ORDER BY created_at ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query authorized chats: %w", err)
	}
	defer rows.Close()

	var chats []ChatInfo
	for rows.Next() {
		var c ChatInfo
		var createdAtStr string
		if err := rows.Scan(&c.ChatID, &c.Title, &createdAtStr); err != nil {
			return nil, err
		}
		t, err := time.Parse("2006-01-02 15:04:05", createdAtStr)
		if err == nil {
			c.CreatedAt = t
		} else {
			t, err = time.Parse(time.RFC3339, createdAtStr)
			if err == nil {
				c.CreatedAt = t
			}
		}
		chats = append(chats, c)
	}
	return chats, nil
}
