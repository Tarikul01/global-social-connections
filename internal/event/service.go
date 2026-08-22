package event

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

var ErrNotAuthorized = errors.New("not authorized")
var ErrCapacityFull = errors.New("event is full")
var ErrInvalidState = errors.New("invalid state")

type Event struct {
	ID                  uuid.UUID
	CreatorID           uuid.UUID
	Title               string
	Description         string
	Category            string
	EventDate           time.Time
	StartTime           string
	EndTime             string
	LocationName        string
	Latitude            float64
	Longitude           float64
	MaxParticipants     int
	Visibility          string
	Status              string
	LiveLocationEnabled bool
	MemberCount         int
}

type Repository struct {
	pool *pgxpool.Pool
	db   *database.DB
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) SetDB(db *database.DB) { r.db = db }

func (r *Repository) Create(ctx context.Context, e Event) (*Event, error) {
	var out Event
	err := r.pool.QueryRow(ctx, `
		INSERT INTO events (creator_id, title, description, category, event_date, start_time, end_time,
			location_name, geom, max_participants, visibility, live_location_enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8, ST_SetSRID(ST_MakePoint($10,$9),4326)::geography, $11,$12,$13)
		RETURNING id, creator_id, title, status
	`, e.CreatorID, e.Title, e.Description, e.Category, e.EventDate, e.StartTime, e.EndTime,
		e.LocationName, e.Latitude, e.Longitude, e.MaxParticipants, e.Visibility, e.LiveLocationEnabled,
	).Scan(&out.ID, &out.CreatorID, &out.Title, &out.Status)
	return &out, err
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Event, error) {
	var e Event
	var lat, lng *float64
	err := r.pool.QueryRow(ctx, `
		SELECT e.id, e.creator_id, e.title, COALESCE(e.description,''), COALESCE(e.category,''),
			e.event_date, e.start_time::text, COALESCE(e.end_time::text,''), COALESCE(e.location_name,''),
			ST_Y(e.geom::geometry), ST_X(e.geom::geometry), e.max_participants, e.visibility, e.status,
			e.live_location_enabled,
			(SELECT COUNT(*) FROM event_members em WHERE em.event_id = e.id)
		FROM events e WHERE e.id=$1
	`, id).Scan(&e.ID, &e.CreatorID, &e.Title, &e.Description, &e.Category, &e.EventDate,
		&e.StartTime, &e.EndTime, &e.LocationName, &lat, &lng, &e.MaxParticipants, &e.Visibility,
		&e.Status, &e.LiveLocationEnabled, &e.MemberCount)
	if lat != nil {
		e.Latitude = *lat
	}
	if lng != nil {
		e.Longitude = *lng
	}
	return &e, err
}

func (r *Repository) List(ctx context.Context, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.creator_id, e.title, e.event_date, e.start_time::text, e.location_name, e.visibility, e.status,
			(SELECT COUNT(*) FROM event_members em WHERE em.event_id = e.id)
		FROM events e WHERE e.status='published' ORDER BY e.event_date ASC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Event
	for rows.Next() {
		var e Event
		_ = rows.Scan(&e.ID, &e.CreatorID, &e.Title, &e.EventDate, &e.StartTime, &e.LocationName, &e.Visibility, &e.Status, &e.MemberCount)
		list = append(list, e)
	}
	return list, nil
}

func (r *Repository) CreateJoinRequest(ctx context.Context, eventID, userID uuid.UUID, message string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO event_join_requests (event_id, user_id, message) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING
	`, eventID, userID, message)
	return err
}

func (r *Repository) AcceptJoin(ctx context.Context, eventID, reqID uuid.UUID) error {
	if r.db == nil {
		return errors.New("database not configured")
	}
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		var userID uuid.UUID
		var maxP, count int
		err := tx.QueryRow(ctx, `
			SELECT ejr.user_id, e.max_participants,
				(SELECT COUNT(*) FROM event_members em WHERE em.event_id = e.id)
			FROM event_join_requests ejr
			JOIN events e ON e.id = ejr.event_id
			WHERE ejr.id=$1 AND ejr.event_id=$2 AND ejr.status='pending' AND e.status='published'
		`, reqID, eventID).Scan(&userID, &maxP, &count)
		if err != nil {
			return err
		}
		if count >= maxP {
			return ErrCapacityFull
		}
		tag, err := tx.Exec(ctx, `UPDATE event_join_requests SET status='accepted', responded_at=now() WHERE id=$1`, reqID)
		if err != nil || tag.RowsAffected() == 0 {
			return ErrInvalidState
		}
		_, err = tx.Exec(ctx, `INSERT INTO event_members (event_id, user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, eventID, userID)
		return err
	})
}

func (r *Repository) RejectJoin(ctx context.Context, reqID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE event_join_requests SET status='rejected', responded_at=now() WHERE id=$1 AND status='pending'`, reqID)
	return err
}

func (r *Repository) ToggleLiveConsent(ctx context.Context, eventID, userID uuid.UUID, consented bool) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO event_live_consent (event_id, user_id, consented, consented_at)
		VALUES ($1,$2,$3, CASE WHEN $3 THEN now() ELSE NULL END)
		ON CONFLICT (event_id, user_id) DO UPDATE SET consented=$3, consented_at=CASE WHEN $3 THEN now() ELSE NULL END, revoked_at=CASE WHEN $3 THEN NULL ELSE now() END
	`, eventID, userID, consented)
	return err
}

type Service struct {
	repo *Repository
	pool *pgxpool.Pool
	log  *logger.Logger
}

func NewService(repo *Repository, pool *pgxpool.Pool, log *logger.Logger) *Service {
	return &Service{repo: repo, pool: pool, log: log}
}

func (s *Service) Create(ctx context.Context, e Event) (*Event, error) {
	if e.Title == "" {
		return nil, errors.New("title required")
	}
	if e.MaxParticipants <= 0 {
		e.MaxParticipants = 20
	}
	return s.repo.Create(ctx, e)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Event, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]Event, error) {
	return s.repo.List(ctx, 50)
}

func (s *Service) JoinRequest(ctx context.Context, eventID, userID uuid.UUID, message string) error {
	e, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return err
	}
	if e.Status != "published" {
		return ErrInvalidState
	}
	return s.repo.CreateJoinRequest(ctx, eventID, userID, message)
}

func (s *Service) AcceptJoin(ctx context.Context, eventID, reqID, creatorID uuid.UUID) error {
	e, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return err
	}
	if e.CreatorID != creatorID {
		return ErrNotAuthorized
	}
	return s.repo.AcceptJoin(ctx, eventID, reqID)
}

func (s *Service) RejectJoin(ctx context.Context, eventID, reqID, creatorID uuid.UUID) error {
	e, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return err
	}
	if e.CreatorID != creatorID {
		return ErrNotAuthorized
	}
	return s.repo.RejectJoin(ctx, reqID)
}

func (s *Service) ToggleLiveConsent(ctx context.Context, eventID, userID uuid.UUID, consented bool) error {
	var member bool
	_ = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM event_members WHERE event_id=$1 AND user_id=$2)`, eventID, userID).Scan(&member)
	if !member {
		return ErrNotAuthorized
	}
	return s.repo.ToggleLiveConsent(ctx, eventID, userID, consented)
}
