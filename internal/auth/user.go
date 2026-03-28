package auth

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
	LastLoginAt  time.Time `json:"lastLoginAt,omitempty"`
}

type UserJSON struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	CreatedAt   string `json:"createdAt"`
	LastLoginAt string `json:"lastLoginAt,omitempty"`
}

func (u *User) ToJSON() UserJSON {
	return UserJSON{
		ID:          u.ID,
		Username:    u.Username,
		Role:        u.Role,
		CreatedAt:   u.CreatedAt.Format(time.RFC3339),
		LastLoginAt: u.LastLoginAt.Format(time.RFC3339),
	}
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func ValidatePassword(password string) bool {
	return len(password) >= 8
}
