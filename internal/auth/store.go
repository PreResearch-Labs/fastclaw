package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type UserStore struct {
	db *sql.DB
}

func NewUserStore(dataDir string) (*UserStore, error) {
	dbPath := filepath.Join(dataDir, "users.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open users db: %w", err)
	}

	store := &UserStore{db: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return store, nil
}

func (s *UserStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			created_at DATETIME NOT NULL,
			last_login_at DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	`)
	return err
}

func (s *UserStore) Close() error {
	return s.db.Close()
}

func (s *UserStore) Create(username, password, role string) (*User, error) {
	if !ValidatePassword(password) {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	id := generateID()
	now := time.Now()

	_, err = s.db.Exec(
		`INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, username, hash, role, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return &User{
		ID:           id,
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		CreatedAt:    now,
	}, nil
}

func (s *UserStore) GetByUsername(username string) (*User, error) {
	u := &User{}
	var lastLogin sql.NullTime
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, role, created_at, last_login_at FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &lastLogin)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastLogin.Valid {
		u.LastLoginAt = lastLogin.Time
	}
	return u, nil
}

func (s *UserStore) GetByID(id string) (*User, error) {
	u := &User{}
	var lastLogin sql.NullTime
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, role, created_at, last_login_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &lastLogin)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastLogin.Valid {
		u.LastLoginAt = lastLogin.Time
	}
	return u, nil
}

func (s *UserStore) List() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, password_hash, role, created_at, last_login_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var lastLogin sql.NullTime
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &lastLogin); err != nil {
			return nil, err
		}
		if lastLogin.Valid {
			u.LastLoginAt = lastLogin.Time
		}
		users = append(users, u)
	}
	return users, nil
}

func (s *UserStore) UpdatePassword(id, password string) error {
	if !ValidatePassword(password) {
		return fmt.Errorf("password must be at least 8 characters")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	return err
}

func (s *UserStore) UpdateLastLogin(id string) error {
	_, err := s.db.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, time.Now(), id)
	return err
}

func (s *UserStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *UserStore) Count() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *UserStore) EnsureDefaultAdmin() (string, error) {
	count, err := s.Count()
	if err != nil {
		return "", err
	}
	if count > 0 {
		return "", nil
	}

	password := os.Getenv("FASTCLAW_ADMIN_PASSWORD")
	if password == "" {
		b := make([]byte, 16)
		rand.Read(b)
		password = hex.EncodeToString(b)
	}

	_, err = s.Create("admin", password, "admin")
	if err != nil {
		return "", err
	}
	return password, nil
}
