package chat

import (
	"context"
	"errors"
	"time"

	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotAuthorized = errors.New("not authorized")
var ErrNotConnected = errors.New("users not connected")

type Conversation struct {
	ID            uuid.UUID
	ConnectionID  uuid.UUID
	OtherUserID   uuid.UUID
	OtherName     string
	LastMessage   string
	UpdatedAt     time.Time
	UnreadCount   int
}

type Message struct {
	ID             uuid.UUID `json:"id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	SenderID       uuid.UUID `json:"sender_id"`
	SenderName     string    `json:"sender_name"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) IsMember(ctx context.Context, convID, userID uuid.UUID) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2)`, convID, userID).Scan(&ok)
	return ok, err
}

func (r *Repository) ListConversations(ctx context.Context, userID uuid.UUID) ([]Conversation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.connection_id, c.updated_at,
			other.user_id, other.full_name,
			COALESCE((SELECT content FROM messages m WHERE m.conversation_id=c.id AND m.deleted_at IS NULL ORDER BY created_at DESC LIMIT 1), '')
		FROM conversations c
		JOIN conversation_members cm ON cm.conversation_id = c.id AND cm.user_id = $1
		JOIN LATERAL (
			SELECT p.user_id, p.full_name
			FROM conversation_members om
			JOIN profiles p ON p.user_id = om.user_id
			WHERE om.conversation_id = c.id AND om.user_id != $1
			LIMIT 1
		) other ON true
		ORDER BY c.updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Conversation
	for rows.Next() {
		var conv Conversation
		_ = rows.Scan(&conv.ID, &conv.ConnectionID, &conv.UpdatedAt, &conv.OtherUserID, &conv.OtherName, &conv.LastMessage)
		list = append(list, conv)
	}
	return list, nil
}

func (r *Repository) GetMessages(ctx context.Context, convID uuid.UUID, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.conversation_id, m.sender_id, p.full_name, m.content, m.created_at
		FROM messages m JOIN profiles p ON p.user_id = m.sender_id
		WHERE m.conversation_id=$1 AND m.deleted_at IS NULL
		ORDER BY m.created_at ASC LIMIT $2
	`, convID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Message
	for rows.Next() {
		var m Message
		_ = rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.SenderName, &m.Content, &m.CreatedAt)
		list = append(list, m)
	}
	return list, nil
}

func (r *Repository) SendMessage(ctx context.Context, convID, senderID uuid.UUID, content string) (*Message, error) {
	var m Message
	err := r.pool.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, sender_id, content) VALUES ($1,$2,$3)
		RETURNING id, conversation_id, sender_id, content, created_at
	`, convID, senderID, content).Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Content, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	_, _ = r.pool.Exec(ctx, `UPDATE conversations SET updated_at=now() WHERE id=$1`, convID)
	return &m, nil
}

func (r *Repository) IsBlocked(ctx context.Context, a, b uuid.UUID) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM blocks WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1))
	`, a, b).Scan(&ok)
	return ok, err
}

type Service struct {
	repo *Repository
	pool *pgxpool.Pool
	log  *logger.Logger
}

func NewService(repo *Repository, pool *pgxpool.Pool, log *logger.Logger) *Service {
	return &Service{repo: repo, pool: pool, log: log}
}

func (s *Service) ListConversations(ctx context.Context, userID uuid.UUID) ([]Conversation, error) {
	return s.repo.ListConversations(ctx, userID)
}

func (s *Service) GetMessages(ctx context.Context, convID, userID uuid.UUID) ([]Message, error) {
	ok, err := s.repo.IsMember(ctx, convID, userID)
	if err != nil || !ok {
		return nil, ErrNotAuthorized
	}
	return s.repo.GetMessages(ctx, convID, 50)
}

func (s *Service) SendMessage(ctx context.Context, convID, senderID uuid.UUID, content string) (*Message, error) {
	if content == "" {
		return nil, errors.New("empty message")
	}
	ok, err := s.repo.IsMember(ctx, convID, senderID)
	if err != nil || !ok {
		return nil, ErrNotAuthorized
	}
	return s.repo.SendMessage(ctx, convID, senderID, content)
}

var _ = pgx.ErrNoRows
