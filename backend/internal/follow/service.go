package follow

import (
	"context"
	"errors"
	"time"

	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrSelfFollow = errors.New("cannot follow yourself")
var ErrBlocked = errors.New("blocked")

type Follow struct {
	ID          uuid.UUID
	FollowerID  uuid.UUID
	FollowingID uuid.UUID
	CreatedAt   time.Time
}

type FollowUser struct {
	UserID   uuid.UUID
	Username string
	FullName string
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) IsBlocked(ctx context.Context, a, b uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM blocks WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1)
		)
	`, a, b).Scan(&exists)
	return exists, err
}

func (r *Repository) Follow(ctx context.Context, follower, following uuid.UUID) error {
	if follower == following {
		return ErrSelfFollow
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO follows (follower_id, following_id) VALUES ($1, $2) ON CONFLICT DO NOTHING
	`, follower, following)
	return err
}

func (r *Repository) Unfollow(ctx context.Context, follower, following uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM follows WHERE follower_id=$1 AND following_id=$2`, follower, following)
	return err
}

func (r *Repository) Followers(ctx context.Context, userID uuid.UUID) ([]FollowUser, error) {
	return r.list(ctx, userID, true)
}

func (r *Repository) Following(ctx context.Context, userID uuid.UUID) ([]FollowUser, error) {
	return r.list(ctx, userID, false)
}

func (r *Repository) list(ctx context.Context, userID uuid.UUID, followers bool) ([]FollowUser, error) {
	q := `
		SELECT u.id, u.username, p.full_name FROM follows f
		JOIN users u ON u.id = CASE WHEN $2 THEN f.follower_id ELSE f.following_id END
		JOIN profiles p ON p.user_id = u.id
		WHERE CASE WHEN $2 THEN f.following_id ELSE f.follower_id END = $1
		ORDER BY f.created_at DESC LIMIT 100
	`
	rows, err := r.pool.Query(ctx, q, userID, followers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []FollowUser
	for rows.Next() {
		var fu FollowUser
		_ = rows.Scan(&fu.UserID, &fu.Username, &fu.FullName)
		list = append(list, fu)
	}
	return list, nil
}

func (r *Repository) Counts(ctx context.Context, userID uuid.UUID) (followers, following int, err error) {
	err = r.pool.QueryRow(ctx, `SELECT
		(SELECT COUNT(*) FROM follows WHERE following_id=$1),
		(SELECT COUNT(*) FROM follows WHERE follower_id=$1)
	`, userID).Scan(&followers, &following)
	return
}

type Service struct {
	repo *Repository
	pool *pgxpool.Pool
	log  *logger.Logger
}

func NewService(repo *Repository, pool *pgxpool.Pool, log *logger.Logger) *Service {
	return &Service{repo: repo, pool: pool, log: log}
}

func (s *Service) Follow(ctx context.Context, follower, following uuid.UUID) error {
	blocked, err := s.repo.IsBlocked(ctx, follower, following)
	if err != nil {
		return err
	}
	if blocked {
		return ErrBlocked
	}
	return s.repo.Follow(ctx, follower, following)
}

func (s *Service) Unfollow(ctx context.Context, follower, following uuid.UUID) error {
	return s.repo.Unfollow(ctx, follower, following)
}

func (s *Service) Followers(ctx context.Context, userID uuid.UUID) ([]FollowUser, error) {
	return s.repo.Followers(ctx, userID)
}

func (s *Service) Following(ctx context.Context, userID uuid.UUID) ([]FollowUser, error) {
	return s.repo.Following(ctx, userID)
}

func UserExists(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) bool {
	var exists bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND deleted_at IS NULL)`, id).Scan(&exists)
	return exists
}

func ParseTargetID(cParam string) (uuid.UUID, error) {
	return uuid.Parse(cParam)
}

var _ = pgx.ErrNoRows
