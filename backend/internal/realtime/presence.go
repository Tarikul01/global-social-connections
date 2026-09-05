package realtime

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OnlineTTL is how long after the last heartbeat a user stays "online".
// Client pings ~25s; closing the browser stops heartbeats → offline after this window
// (Clear on disconnect makes it immediate when the socket closes cleanly).
const OnlineTTL = 90 * time.Second

// DBPresence stores map-online state in Postgres so it works across Vercel instances.
type DBPresence struct {
	pool *pgxpool.Pool
	ttl  time.Duration
	once sync.Once
}

func NewDBPresence(pool *pgxpool.Pool) *DBPresence {
	p := &DBPresence{pool: pool, ttl: OnlineTTL}
	p.ensureColumn()
	return p
}

func (p *DBPresence) ttlSeconds() int {
	s := int(p.ttl.Seconds())
	if s < 30 {
		return 30
	}
	return s
}

func (p *DBPresence) ensureColumn() {
	p.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = p.pool.Exec(ctx, `
			ALTER TABLE users ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ
		`)
	})
}

// Touch marks the user online (call on WS connect + ping).
func (p *DBPresence) Touch(ctx context.Context, userID uuid.UUID) {
	if userID == uuid.Nil || p.pool == nil {
		return
	}
	_, _ = p.pool.Exec(ctx, `
		UPDATE users SET last_seen_at = now() WHERE id = $1 AND deleted_at IS NULL
	`, userID)
}

// Clear marks the user offline immediately (WS disconnect / logout).
func (p *DBPresence) Clear(ctx context.Context, userID uuid.UUID) {
	if userID == uuid.Nil || p.pool == nil {
		return
	}
	_, _ = p.pool.Exec(ctx, `
		UPDATE users SET last_seen_at = NULL WHERE id = $1
	`, userID)
}

func (p *DBPresence) IsOnline(userID uuid.UUID) bool {
	if userID == uuid.Nil || p.pool == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var ok bool
	_ = p.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM users
			WHERE id = $1
			  AND deleted_at IS NULL
			  AND last_seen_at IS NOT NULL
			  AND last_seen_at > now() - make_interval(secs => $2)
		)
	`, userID, p.ttlSeconds()).Scan(&ok)
	return ok
}

func (p *DBPresence) OnlineCount() int {
	if p.pool == nil {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var n int
	_ = p.pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM users
		WHERE deleted_at IS NULL
		  AND last_seen_at IS NOT NULL
		  AND last_seen_at > now() - make_interval(secs => $1)
	`, p.ttlSeconds()).Scan(&n)
	return n
}

func (p *DBPresence) OnlineIDs() []uuid.UUID {
	if p.pool == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rows, err := p.pool.Query(ctx, `
		SELECT id FROM users
		WHERE deleted_at IS NULL
		  AND last_seen_at IS NOT NULL
		  AND last_seen_at > now() - make_interval(secs => $1)
	`, p.ttlSeconds())
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if rows.Scan(&id) == nil {
			out = append(out, id)
		}
	}
	return out
}
