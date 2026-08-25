package social

import (
	"context"
	"errors"

	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrSelf = errors.New("cannot block or report yourself")

type Service struct {
	pool *pgxpool.Pool
	log  *logger.Logger
}

func NewService(pool *pgxpool.Pool, log *logger.Logger) *Service {
	return &Service{pool: pool, log: log}
}

func (s *Service) Block(ctx context.Context, blocker, blocked uuid.UUID) error {
	if blocker == blocked {
		return ErrSelf
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, blocker, blocked)
	return err
}

func (s *Service) Unblock(ctx context.Context, blocker, blocked uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM blocks WHERE blocker_id=$1 AND blocked_id=$2`, blocker, blocked)
	return err
}

func (s *Service) Report(ctx context.Context, reporter uuid.UUID, reason, description string, reportedUser *uuid.UUID, reportedEvent *uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO reports (reporter_id, reported_user_id, reported_event_id, reason, description)
		VALUES ($1,$2,$3,$4,$5)
	`, reporter, reportedUser, reportedEvent, reason, description)
	return err
}
