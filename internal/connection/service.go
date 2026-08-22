package connection

import (
	"context"
	"errors"
	"time"

	"github.com/findneighbour/findneighbour/internal/database"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSelfConnect    = errors.New("cannot connect with yourself")
	ErrNotAuthorized  = errors.New("not authorized")
	ErrInvalidState   = errors.New("invalid connection state")
	ErrBlocked        = errors.New("blocked")
	ErrDuplicate      = errors.New("connection already exists")
)

type Connection struct {
	ID          uuid.UUID
	RequesterID uuid.UUID
	RecipientID uuid.UUID
	Status      string
	Message     string
	CreatedAt   time.Time
	RequesterName string
	RecipientName string
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Connection, error) {
	var conn Connection
	err := r.pool.QueryRow(ctx, `
		SELECT id, requester_id, recipient_id, status, COALESCE(message,''), created_at
		FROM connections WHERE id=$1
	`, id).Scan(&conn.ID, &conn.RequesterID, &conn.RecipientID, &conn.Status, &conn.Message, &conn.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &conn, nil
}

func (r *Repository) Create(ctx context.Context, requester, recipient uuid.UUID, message string) (*Connection, error) {
	var conn Connection
	err := r.pool.QueryRow(ctx, `
		INSERT INTO connections (requester_id, recipient_id, message)
		VALUES ($1, $2, $3)
		ON CONFLICT (requester_id, recipient_id) DO NOTHING
		RETURNING id, requester_id, recipient_id, status, created_at
	`, requester, recipient, message).Scan(&conn.ID, &conn.RequesterID, &conn.RecipientID, &conn.Status, &conn.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return &conn, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE connections SET status=$2, responded_at=now(), updated_at=now() WHERE id=$1 AND status='pending'
	`, id, status)
	return err
}

func (r *Repository) ListAccepted(ctx context.Context, userID uuid.UUID) ([]Connection, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.requester_id, c.recipient_id, c.status, c.created_at,
			pr.full_name, pe.full_name
		FROM connections c
		JOIN profiles pr ON pr.user_id = c.requester_id
		JOIN profiles pe ON pe.user_id = c.recipient_id
		WHERE c.status='accepted' AND (c.requester_id=$1 OR c.recipient_id=$1)
		ORDER BY c.updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Connection
	for rows.Next() {
		var conn Connection
		_ = rows.Scan(&conn.ID, &conn.RequesterID, &conn.RecipientID, &conn.Status, &conn.CreatedAt, &conn.RequesterName, &conn.RecipientName)
		list = append(list, conn)
	}
	return list, nil
}

func (r *Repository) ListPending(ctx context.Context, userID uuid.UUID) ([]Connection, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.requester_id, c.recipient_id, c.status, COALESCE(c.message,''), c.created_at, p.full_name
		FROM connections c
		JOIN profiles p ON p.user_id = c.requester_id
		WHERE c.recipient_id=$1 AND c.status='pending'
		ORDER BY c.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Connection
	for rows.Next() {
		var conn Connection
		_ = rows.Scan(&conn.ID, &conn.RequesterID, &conn.RecipientID, &conn.Status, &conn.Message, &conn.CreatedAt, &conn.RequesterName)
		list = append(list, conn)
	}
	return list, nil
}

type Service struct {
	repo *Repository
	pool *pgxpool.Pool
	db   *database.DB
	log  *logger.Logger
}

func NewService(repo *Repository, pool *pgxpool.Pool, log *logger.Logger) *Service {
	return &Service{repo: repo, pool: pool, log: log}
}

func (s *Service) SetDB(db *database.DB) { s.db = db }

func (s *Service) isBlocked(ctx context.Context, a, b uuid.UUID) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM blocks WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1))
	`, a, b).Scan(&exists)
	return exists, err
}

func (s *Service) SendRequest(ctx context.Context, requester, recipient uuid.UUID, message string) (*Connection, error) {
	if requester == recipient {
		return nil, ErrSelfConnect
	}
	blocked, err := s.isBlocked(ctx, requester, recipient)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, ErrBlocked
	}
	return s.repo.Create(ctx, requester, recipient, message)
}

func (s *Service) Accept(ctx context.Context, connID, userID uuid.UUID) error {
	conn, err := s.repo.GetByID(ctx, connID)
	if err != nil {
		return err
	}
	if conn.RecipientID != userID {
		return ErrNotAuthorized
	}
	if conn.Status != "pending" {
		return ErrInvalidState
	}
	return s.db.WithTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE connections SET status='accepted', responded_at=now(), updated_at=now() WHERE id=$1 AND status='pending'`, connID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrInvalidState
		}
		var convID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO conversations (connection_id) VALUES ($1) RETURNING id
		`, connID).Scan(&convID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO conversation_members (conversation_id, user_id) VALUES ($1, $2), ($1, $3)
		`, convID, conn.RequesterID, conn.RecipientID)
		return err
	})
}

func (s *Service) Reject(ctx context.Context, connID, userID uuid.UUID) error {
	conn, err := s.repo.GetByID(ctx, connID)
	if err != nil {
		return err
	}
	if conn.RecipientID != userID {
		return ErrNotAuthorized
	}
	return s.repo.UpdateStatus(ctx, connID, "rejected")
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Connection, error) {
	return s.repo.ListAccepted(ctx, userID)
}

func (s *Service) Pending(ctx context.Context, userID uuid.UUID) ([]Connection, error) {
	return s.repo.ListPending(ctx, userID)
}

func (s *Service) AreConnected(ctx context.Context, a, b uuid.UUID) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM connections
			WHERE status='accepted' AND (
				(requester_id=$1 AND recipient_id=$2) OR (requester_id=$2 AND recipient_id=$1)
			)
		)
	`, a, b).Scan(&exists)
	return exists, err
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Connection, error) {
	return s.repo.GetByID(ctx, id)
}
