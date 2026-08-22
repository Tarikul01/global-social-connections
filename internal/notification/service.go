package notification

import (
	"context"
	"encoding/json"
	"time"

	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Notification struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Type      string
	Title     string
	Body      string
	Data      map[string]string
	ReadAt    *time.Time
	CreatedAt time.Time
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, userID uuid.UUID, nType, title, body string, data map[string]string) error {
	var raw []byte
	if data != nil {
		raw, _ = json.Marshal(data)
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, body, data) VALUES ($1,$2,$3,$4,$5)
	`, userID, nType, title, body, raw)
	return err
}

func (r *Repository) List(ctx context.Context, userID uuid.UUID, limit int) ([]Notification, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, type, title, COALESCE(body,''), data, read_at, created_at
		FROM notifications WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Notification
	for rows.Next() {
		var n Notification
		var raw []byte
		_ = rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &raw, &n.ReadAt, &n.CreatedAt)
		if raw != nil {
			_ = json.Unmarshal(raw, &n.Data)
		}
		list = append(list, n)
	}
	return list, nil
}

func (r *Repository) MarkRead(ctx context.Context, id, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE notifications SET read_at=now() WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}

type Service struct {
	repo *Repository
	log  *logger.Logger
}

func NewService(repo *Repository, log *logger.Logger) *Service {
	return &Service{repo: repo, log: log}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, nType, title, body string, data map[string]string) error {
	return s.repo.Create(ctx, userID, nType, title, body, data)
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Notification, error) {
	return s.repo.List(ctx, userID, 50)
}

func (s *Service) MarkRead(ctx context.Context, id, userID uuid.UUID) error {
	return s.repo.MarkRead(ctx, id, userID)
}
