package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/security"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountLocked      = errors.New("account locked")
	ErrEmailTaken         = errors.New("email already taken")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrInvalidToken       = errors.New("invalid token")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateUser(ctx context.Context, email, username, passwordHash, fullName string) (*User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var u User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, username, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, username, status, created_at
	`, email, username, passwordHash).Scan(&u.ID, &u.Email, &u.Username, &u.Status, &u.CreatedAt)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO profiles (user_id, full_name) VALUES ($1, $2)
	`, u.ID, fullName)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO location_settings (user_id) VALUES ($1)
	`, u.ID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (*User, string, error) {
	var u User
	var hash string
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, email_verified_at, username, status, failed_login_attempts, locked_until, password_hash, created_at
		FROM users WHERE email = $1 AND deleted_at IS NULL
	`, email).Scan(&u.ID, &u.Email, &u.EmailVerifiedAt, &u.Username, &u.Status, &u.FailedLoginAttempts, &u.LockedUntil, &hash, &u.CreatedAt)
	if err != nil {
		return nil, "", err
	}
	return &u, hash, nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, email_verified_at, username, status, created_at
		FROM users WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&u.ID, &u.Email, &u.EmailVerifiedAt, &u.Username, &u.Status, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash, ip, ua string, expires time.Time) (*Session, error) {
	var s Session
	var ipArg any
	if ip = strings.TrimSpace(ip); ip != "" {
		ipArg = ip
	}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO sessions (user_id, token_hash, ip_address, user_agent, expires_at)
		VALUES ($1, $2, $3::inet, $4, $5)
		RETURNING id, user_id, token_hash, expires_at
	`, userID, tokenHash, ipArg, ua, expires).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt)
	return &s, err
}

func (r *Repository) GetSessionByToken(ctx context.Context, tokenHash string) (*Session, *User, error) {
	var s Session
	var u User
	err := r.pool.QueryRow(ctx, `
		SELECT s.id, s.user_id, s.expires_at,
			u.id, u.email, u.username, u.status, u.created_at,
			COALESCE(p.onboarding_completed, false)
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now() AND u.deleted_at IS NULL
	`, tokenHash).Scan(&s.ID, &s.UserID, &s.ExpiresAt, &u.ID, &u.Email, &u.Username, &u.Status, &u.CreatedAt, &u.OnboardingCompleted)
	if err != nil {
		return nil, nil, err
	}
	return &s, &u, nil
}

func (r *Repository) RevokeSession(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		UPDATE sessions SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL
		RETURNING user_id
	`, tokenHash).Scan(&userID)
	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

// RevokeAllUserSessions marks every active session for the user as revoked.
// Used so "online" (any active session) clears correctly after logout/login.
func (r *Repository) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sessions SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID)
	return err
}

func (r *Repository) ClearPresenceOnLogout(ctx context.Context, userID uuid.UUID) {
	_, _ = r.pool.Exec(ctx, `
		UPDATE location_settings
		SET live_sharing_enabled = false, updated_at = now()
		WHERE user_id = $1
	`, userID)
	_, _ = r.pool.Exec(ctx, `DELETE FROM live_locations WHERE user_id = $1`, userID)
	_, _ = r.pool.Exec(ctx, `UPDATE users SET last_seen_at = NULL WHERE id = $1`, userID)
}

func (r *Repository) IncrementFailedLogin(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET failed_login_attempts = failed_login_attempts + 1,
			locked_until = CASE WHEN failed_login_attempts + 1 >= 5 THEN now() + interval '15 minutes' ELSE locked_until END
		WHERE id = $1
	`, userID)
	return err
}

func (r *Repository) ResetFailedLogin(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET failed_login_attempts = 0, locked_until = NULL WHERE id = $1`, userID)
	return err
}

func (r *Repository) CreateVerificationToken(ctx context.Context, userID uuid.UUID, tokenHash string, expires time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO email_verification_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)
	`, userID, tokenHash, expires)
	return err
}

func (r *Repository) VerifyEmail(ctx context.Context, tokenHash string) error {
	var userID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		UPDATE email_verification_tokens SET used_at = now()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		RETURNING user_id
	`, tokenHash).Scan(&userID)
	if err != nil {
		return ErrInvalidToken
	}
	_, err = r.pool.Exec(ctx, `UPDATE users SET email_verified_at = now() WHERE id = $1`, userID)
	return err
}

type Service struct {
	repo   *Repository
	cfg    *config.Config
	argon  security.ArgonParams
	log    *logger.Logger
}

func NewService(repo *Repository, cfg *config.Config, argon security.ArgonParams, log *logger.Logger) *Service {
	return &Service{repo: repo, cfg: cfg, argon: argon, log: log}
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (*User, string, error) {
	if len(in.Password) < 8 {
		return nil, "", fmt.Errorf("password must be at least 8 characters")
	}
	hash, err := security.HashPassword(in.Password, s.argon)
	if err != nil {
		return nil, "", err
	}
	u, err := s.repo.CreateUser(ctx, in.Email, in.Username, hash, in.FullName)
	if err != nil {
		return nil, "", ErrEmailTaken
	}
	token, err := security.GenerateToken(32)
	if err != nil {
		return nil, "", err
	}
	_ = s.repo.CreateVerificationToken(ctx, u.ID, security.HashToken(token), time.Now().Add(24*time.Hour))
	return u, token, nil
}

func (s *Service) IssueSession(ctx context.Context, userID uuid.UUID, ip, ua string) (string, error) {
	token, err := security.GenerateToken(32)
	if err != nil {
		return "", err
	}
	// Drop prior sessions so presence (online = active session) cannot stick
	// from an older cookie/login after a fresh login.
	_ = s.repo.RevokeAllUserSessions(ctx, userID)
	_, err = s.repo.CreateSession(ctx, userID, security.HashToken(token), ip, ua, time.Now().Add(s.cfg.SessionTTL))
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) Login(ctx context.Context, in LoginInput, ip, ua string) (*User, string, error) {
	u, hash, err := s.repo.GetByEmail(ctx, in.Email)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}
	if u.LockedUntil != nil && u.LockedUntil.After(time.Now()) {
		return nil, "", ErrAccountLocked
	}
	ok, err := security.VerifyPassword(in.Password, hash)
	if err != nil || !ok {
		_ = s.repo.IncrementFailedLogin(ctx, u.ID)
		return nil, "", ErrInvalidCredentials
	}
	_ = s.repo.ResetFailedLogin(ctx, u.ID)
	token, err := s.IssueSession(ctx, u.ID, ip, ua)
	if err != nil {
		return nil, "", err
	}
	_ = s.repo.pool.QueryRow(ctx, `
		SELECT COALESCE(onboarding_completed, false) FROM profiles WHERE user_id=$1
	`, u.ID).Scan(&u.OnboardingCompleted)
	return u, token, nil
}

func (s *Service) ValidateSession(ctx context.Context, token string) (*User, *Session, error) {
	sess, user, err := s.repo.GetSessionByToken(ctx, security.HashToken(token))
	if err != nil {
		return nil, nil, err
	}
	return user, sess, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	userID, err := s.repo.RevokeSession(ctx, security.HashToken(token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if userID != uuid.Nil {
		// Revoke every session for this user so they go offline immediately
		// (stale sessions from earlier logins would otherwise keep them online).
		_ = s.repo.RevokeAllUserSessions(ctx, userID)
		s.repo.ClearPresenceOnLogout(ctx, userID)
	}
	return nil
}

func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	return s.repo.VerifyEmail(ctx, security.HashToken(token))
}

func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.repo.GetByID(ctx, id)
}
