package auth

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID                  uuid.UUID
	Email               string
	EmailVerifiedAt     *time.Time
	Username            string
	Status              string
	FailedLoginAttempts int
	LockedUntil         *time.Time
	CreatedAt           time.Time
}

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
}

type RegisterInput struct {
	Email    string
	Username string
	Password string
	FullName string
}

type LoginInput struct {
	Email    string
	Password string
}
