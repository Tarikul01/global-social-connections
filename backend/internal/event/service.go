package event

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/findneighbour/findneighbour/internal/database"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotAuthorized   = errors.New("not authorized")
	ErrCapacityFull    = errors.New("event is full")
	ErrInvalidState    = errors.New("invalid state")
	ErrPrivateEvent    = errors.New("this is a private event — you need an invitation")
	ErrAlreadyMember   = errors.New("already a member of this event")
	ErrCannotJoinOwn   = errors.New("you cannot join your own event")
	ErrUserNotFound    = errors.New("user not found")
	ErrAlreadyInvited  = errors.New("user already invited")
	ErrEventLimit      = errors.New("you can create at most 2 events at a time")
	ErrNotCreator      = errors.New("only the event creator can do this")
)

// MaxEventsPerCreator is how many events one user may own concurrently.
const MaxEventsPerCreator = 2

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

type JoinRequestView struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	Username string
	FullName string
	Message  string
}

type InvitationView struct {
	ID              uuid.UUID `json:"id"`
	InviteeID       uuid.UUID `json:"invitee_id"`
	Username        string    `json:"username"`
	Email           string    `json:"email,omitempty"`
	FullName        string    `json:"full_name"`
	Status          string    `json:"status"`
	InviterID       uuid.UUID `json:"inviter_id"`
	InviterUsername string    `json:"inviter_username"`
	InviterName     string    `json:"inviter_name"`
}

type HostView struct {
	UserID    uuid.UUID
	Username  string
	FullName  string
	Role      string
	IsCreator bool
}

type UserSearchResult struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	FullName string    `json:"full_name"`
}

type MemberView struct {
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	Role      string    `json:"role"`
	IsCreator bool      `json:"is_creator"`
}

type InvitationListPage struct {
	Items      []InvitationView
	Total      int
	Page       int
	PerPage    int
	TotalPages int
	Accepted   int
	Pending    int
	Declined   int
}

type BrowseUserView struct {
	ID             uuid.UUID
	Username       string
	Email          string
	FullName       string
	AlreadyInvited bool
	IsMember       bool
}

type BrowseUsersPage struct {
	Items      []BrowseUserView
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

// HostedEventSummary is a compact host-facing event card for the discover map.
type HostedEventSummary struct {
	ID           uuid.UUID `json:"id"`
	Title        string    `json:"title"`
	ShortName    string    `json:"short_name"`
	LocationName string    `json:"location_name"`
	Latitude     float64   `json:"lat"`
	Longitude    float64   `json:"lng"`
	EventDate    string    `json:"event_date"`
	MemberCount  int       `json:"member_count"`
}

// EventMapMember is an accepted member with a map position for host routing view.
type EventMapMember struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	FullName string    `json:"full_name"`
	Role     string    `json:"role"`
	Initials string    `json:"initials"`
	Latitude float64   `json:"lat"`
	Longitude float64  `json:"lng"`
	Online   bool      `json:"online"`
	Live     bool      `json:"live"`
}

// EventMapView is event pin + going members with locations (for hosts/members).
type EventMapView struct {
	Event   HostedEventSummary `json:"event"`
	Members []EventMapMember   `json:"members"`
}

type ShowContext struct {
	Event               Event
	IsCreator           bool
	IsHost              bool
	IsMember            bool
	HasPendingJoin      bool
	MyInvitationID      *uuid.UUID
	PendingJoinRequests []JoinRequestView
	Hosts               []HostView
	Invitations         InvitationListPage
	AcceptedInvites     int
	PendingInvites      int
	TotalInvites        int
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
	if r.db == nil {
		return r.createSimple(ctx, e)
	}
	var out Event
	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO events (creator_id, title, description, category, event_date, start_time, end_time,
				location_name, geom, max_participants, visibility, live_location_enabled)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8, ST_SetSRID(ST_MakePoint($10,$9),4326)::geography, $11,$12,$13)
			RETURNING id, creator_id, title, status
		`, e.CreatorID, e.Title, e.Description, e.Category, e.EventDate, e.StartTime, e.EndTime,
			e.LocationName, e.Latitude, e.Longitude, e.MaxParticipants, e.Visibility, e.LiveLocationEnabled,
		).Scan(&out.ID, &out.CreatorID, &out.Title, &out.Status)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO event_members (event_id, user_id, role) VALUES ($1, $2, 'host')
			ON CONFLICT (event_id, user_id) DO NOTHING
		`, out.ID, e.CreatorID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, out.ID)
}

func (r *Repository) createSimple(ctx context.Context, e Event) (*Event, error) {
	var out Event
	err := r.pool.QueryRow(ctx, `
		INSERT INTO events (creator_id, title, description, category, event_date, start_time, end_time,
			location_name, geom, max_participants, visibility, live_location_enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8, ST_SetSRID(ST_MakePoint($10,$9),4326)::geography, $11,$12,$13)
		RETURNING id, creator_id, title, status
	`, e.CreatorID, e.Title, e.Description, e.Category, e.EventDate, e.StartTime, e.EndTime,
		e.LocationName, e.Latitude, e.Longitude, e.MaxParticipants, e.Visibility, e.LiveLocationEnabled,
	).Scan(&out.ID, &out.CreatorID, &out.Title, &out.Status)
	if err != nil {
		return nil, err
	}
	_, _ = r.pool.Exec(ctx, `
		INSERT INTO event_members (event_id, user_id, role) VALUES ($1, $2, 'host')
		ON CONFLICT (event_id, user_id) DO NOTHING
	`, out.ID, e.CreatorID)
	return r.GetByID(ctx, out.ID)
}

func (r *Repository) CountByCreator(ctx context.Context, creatorID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM events WHERE creator_id = $1
	`, creatorID).Scan(&n)
	return n, err
}

// DeleteByCreator removes the event and all dependent rows. Only the creator may delete.
// Cascades cover members, invitations, join requests, and live consent; we also clear
// live_locations, reports, and notifications that reference the event.
func (r *Repository) DeleteByCreator(ctx context.Context, eventID, creatorID uuid.UUID) error {
	var owner uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT creator_id FROM events WHERE id = $1`, eventID).Scan(&owner)
	if err != nil {
		return err
	}
	if owner != creatorID {
		return ErrNotCreator
	}

	run := func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM live_locations WHERE event_id = $1`, eventID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE reports SET reported_event_id = NULL WHERE reported_event_id = $1`, eventID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM notifications
			WHERE data->>'event_id' = $1
		`, eventID.String()); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM events WHERE id = $1 AND creator_id = $2`, eventID, creatorID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotCreator
		}
		return nil
	}

	if r.db != nil {
		return r.db.WithTx(ctx, run)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := run(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) UpdateByCreator(ctx context.Context, eventID, creatorID uuid.UUID, e Event) (*Event, error) {
	var owner uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT creator_id FROM events WHERE id = $1`, eventID).Scan(&owner)
	if err != nil {
		return nil, err
	}
	if owner != creatorID {
		return nil, ErrNotCreator
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE events SET
			title = $3,
			description = $4,
			category = $5,
			event_date = $6,
			start_time = $7,
			end_time = NULLIF($8, '')::time,
			location_name = $9,
			geom = ST_SetSRID(ST_MakePoint($11, $10), 4326)::geography,
			max_participants = $12,
			visibility = $13,
			live_location_enabled = $14,
			updated_at = now()
		WHERE id = $1 AND creator_id = $2
	`, eventID, creatorID, e.Title, e.Description, e.Category, e.EventDate, e.StartTime, e.EndTime,
		e.LocationName, e.Latitude, e.Longitude, e.MaxParticipants, e.Visibility, e.LiveLocationEnabled)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotCreator
	}
	return r.GetByID(ctx, eventID)
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
	return r.ListForViewer(ctx, uuid.Nil, limit)
}

// ListForViewer returns published events the viewer may see on discover:
// all public events, plus private events where they are creator or accepted member.
func (r *Repository) ListForViewer(ctx context.Context, viewerID uuid.UUID, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 20
	}
	const cols = `
		SELECT e.id, e.creator_id, e.title, COALESCE(e.description,''), COALESCE(e.category,''),
			e.event_date, e.start_time::text, COALESCE(e.end_time::text,''), e.location_name,
			COALESCE(ST_Y(e.geom::geometry), 0), COALESCE(ST_X(e.geom::geometry), 0),
			e.max_participants, e.visibility, e.status, e.live_location_enabled,
			(SELECT COUNT(*) FROM event_members em WHERE em.event_id = e.id)
		FROM events e`
	var (
		rows pgx.Rows
		err  error
	)
	if viewerID == uuid.Nil {
		rows, err = r.pool.Query(ctx, cols+`
			WHERE e.status = 'published' AND e.visibility = 'public'
			ORDER BY e.event_date ASC
			LIMIT $1
		`, limit)
	} else {
		rows, err = r.pool.Query(ctx, cols+`
			WHERE e.status = 'published'
			  AND (
				e.visibility = 'public'
				OR (
					e.visibility = 'private'
					AND (
						e.creator_id = $2
						OR EXISTS (
							SELECT 1 FROM event_members em
							WHERE em.event_id = e.id AND em.user_id = $2
						)
					)
				)
			  )
			ORDER BY e.event_date ASC
			LIMIT $1
		`, limit, viewerID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Event
	for rows.Next() {
		var e Event
		_ = rows.Scan(
			&e.ID, &e.CreatorID, &e.Title, &e.Description, &e.Category,
			&e.EventDate, &e.StartTime, &e.EndTime, &e.LocationName,
			&e.Latitude, &e.Longitude,
			&e.MaxParticipants, &e.Visibility, &e.Status, &e.LiveLocationEnabled,
			&e.MemberCount,
		)
		list = append(list, e)
	}
	return list, nil
}

func shortEventName(title string) string {
	t := strings.TrimSpace(title)
	runes := []rune(t)
	if len(runes) <= 22 {
		return t
	}
	return string(runes[:20]) + "…"
}

func (r *Repository) ListHostedByUser(ctx context.Context, userID uuid.UUID) ([]HostedEventSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.title, COALESCE(e.location_name,''),
			COALESCE(ST_Y(e.geom::geometry), 0), COALESCE(ST_X(e.geom::geometry), 0),
			to_char(e.event_date, 'Mon DD'),
			(SELECT COUNT(*) FROM event_members em WHERE em.event_id = e.id)
		FROM events e
		JOIN event_members hm ON hm.event_id = e.id AND hm.user_id = $1 AND hm.role IN ('host','co_host')
		WHERE e.status = 'published' AND e.geom IS NOT NULL
		ORDER BY e.event_date ASC, e.created_at DESC
		LIMIT 30
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []HostedEventSummary
	for rows.Next() {
		var e HostedEventSummary
		if err := rows.Scan(&e.ID, &e.Title, &e.LocationName, &e.Latitude, &e.Longitude, &e.EventDate, &e.MemberCount); err != nil {
			return nil, err
		}
		e.ShortName = shortEventName(e.Title)
		list = append(list, e)
	}
	return list, nil
}

func (r *Repository) ListAcceptedMembersWithLocations(ctx context.Context, eventID uuid.UUID) ([]EventMapMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.username, COALESCE(p.full_name, u.username), em.role,
			COALESCE(
				(SELECT ll.latitude FROM live_locations ll
				 JOIN location_settings ls ON ls.user_id = u.id
				 WHERE ll.user_id = u.id
				   AND ls.live_sharing_enabled = true
				   AND ll.updated_at > now() - interval '10 minutes'
				   AND (ll.event_id IS NULL OR ll.event_id = $1)
				 ORDER BY ll.updated_at DESC LIMIT 1),
				ST_Y(ul.geom::geometry)
			),
			COALESCE(
				(SELECT ll.longitude FROM live_locations ll
				 JOIN location_settings ls ON ls.user_id = u.id
				 WHERE ll.user_id = u.id
				   AND ls.live_sharing_enabled = true
				   AND ll.updated_at > now() - interval '10 minutes'
				   AND (ll.event_id IS NULL OR ll.event_id = $1)
				 ORDER BY ll.updated_at DESC LIMIT 1),
				ST_X(ul.geom::geometry)
			),
			EXISTS (
				SELECT 1 FROM live_locations ll
				JOIN location_settings ls ON ls.user_id = u.id
				WHERE ll.user_id = u.id
				  AND ls.live_sharing_enabled = true
				  AND ll.updated_at > now() - interval '10 minutes'
				  AND (ll.event_id IS NULL OR ll.event_id = $1)
			) AS is_live,
			(u.last_seen_at IS NOT NULL AND u.last_seen_at > now() - interval '90 seconds') AS is_online
		FROM event_members em
		JOIN users u ON u.id = em.user_id
		LEFT JOIN profiles p ON p.user_id = u.id
		JOIN user_locations ul ON ul.user_id = u.id AND ul.geom IS NOT NULL
		WHERE em.event_id = $1
		  AND u.deleted_at IS NULL
		ORDER BY
			CASE em.role WHEN 'host' THEN 0 WHEN 'co_host' THEN 1 WHEN 'member' THEN 2 ELSE 3 END,
			COALESCE(p.full_name, u.username)
	`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []EventMapMember
	for rows.Next() {
		var m EventMapMember
		if err := rows.Scan(&m.ID, &m.Username, &m.FullName, &m.Role, &m.Latitude, &m.Longitude, &m.Live, &m.Online); err != nil {
			return nil, err
		}
		m.Initials = memberInitials(m.FullName)
		list = append(list, m)
	}
	return list, nil
}

func memberInitials(name string) string {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) == 0 {
		return "?"
	}
	out := string(runes[0])
	for i := 1; i < len(runes)-1; i++ {
		if runes[i] == ' ' && i+1 < len(runes) {
			out += string(runes[i+1])
			break
		}
	}
	if len([]rune(out)) > 2 {
		return string([]rune(out)[:2])
	}
	return out
}

func (r *Repository) IsHost(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM event_members
			WHERE event_id=$1 AND user_id=$2 AND role IN ('host','co_host')
		)
	`, eventID, userID).Scan(&ok)
	return ok, err
}

func (r *Repository) IsMember(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM event_members WHERE event_id=$1 AND user_id=$2)
	`, eventID, userID).Scan(&ok)
	return ok, err
}

func (r *Repository) HasPendingJoinRequest(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM event_join_requests WHERE event_id=$1 AND user_id=$2 AND status='pending')
	`, eventID, userID).Scan(&ok)
	return ok, err
}

func (r *Repository) CreateJoinRequest(ctx context.Context, eventID, userID uuid.UUID, message string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO event_join_requests (event_id, user_id, message) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING
	`, eventID, userID, message)
	return err
}

func (r *Repository) ListPendingJoinRequests(ctx context.Context, eventID uuid.UUID) ([]JoinRequestView, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ejr.id, ejr.user_id, u.username, COALESCE(p.full_name, u.username), COALESCE(ejr.message,'')
		FROM event_join_requests ejr
		JOIN users u ON u.id = ejr.user_id
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE ejr.event_id=$1 AND ejr.status='pending'
		ORDER BY ejr.created_at ASC
	`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []JoinRequestView
	for rows.Next() {
		var j JoinRequestView
		if err := rows.Scan(&j.ID, &j.UserID, &j.Username, &j.FullName, &j.Message); err != nil {
			return nil, err
		}
		list = append(list, j)
	}
	return list, nil
}

func (r *Repository) AcceptJoin(ctx context.Context, eventID, reqID uuid.UUID) (uuid.UUID, error) {
	if r.db == nil {
		return uuid.Nil, errors.New("database not configured")
	}
	var joinedUserID uuid.UUID
	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
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
		joinedUserID = userID
		return err
	})
	return joinedUserID, err
}

func (r *Repository) RejectJoin(ctx context.Context, reqID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE event_join_requests SET status='rejected', responded_at=now() WHERE id=$1 AND status='pending'`, reqID)
	return err
}

func (r *Repository) LookupUserByUsername(ctx context.Context, username string) (uuid.UUID, string, error) {
	return r.lookupUser(ctx, username, false)
}

func (r *Repository) LookupUserByQuery(ctx context.Context, query string) (uuid.UUID, string, error) {
	return r.lookupUser(ctx, query, true)
}

func (r *Repository) lookupUser(ctx context.Context, raw string, allowEmail bool) (uuid.UUID, string, error) {
	q := strings.TrimSpace(strings.TrimPrefix(raw, "@"))
	if q == "" {
		return uuid.Nil, "", pgx.ErrNoRows
	}
	var id uuid.UUID
	var fullName string
	if allowEmail && strings.Contains(q, "@") {
		err := r.pool.QueryRow(ctx, `
			SELECT u.id, COALESCE(p.full_name, u.username)
			FROM users u LEFT JOIN profiles p ON p.user_id = u.id
			WHERE lower(u.email) = lower($1) AND u.deleted_at IS NULL
		`, q).Scan(&id, &fullName)
		if err == nil {
			return id, fullName, nil
		}
	}
	err := r.pool.QueryRow(ctx, `
		SELECT u.id, COALESCE(p.full_name, u.username)
		FROM users u LEFT JOIN profiles p ON p.user_id = u.id
		WHERE u.username = $1 AND u.deleted_at IS NULL
	`, q).Scan(&id, &fullName)
	if err == nil {
		return id, fullName, nil
	}
	if allowEmail {
		err = r.pool.QueryRow(ctx, `
			SELECT u.id, COALESCE(p.full_name, u.username)
			FROM users u LEFT JOIN profiles p ON p.user_id = u.id
			WHERE u.username ILIKE $1 AND u.deleted_at IS NULL
			ORDER BY u.username LIMIT 1
		`, q).Scan(&id, &fullName)
	}
	return id, fullName, err
}

func (r *Repository) SearchUsers(ctx context.Context, eventID, excludeUserID uuid.UUID, query string, limit int) ([]UserSearchResult, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	pattern := "%" + q + "%"
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.username, u.email, COALESCE(p.full_name, u.username)
		FROM users u
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE u.deleted_at IS NULL
		  AND u.id <> $4
		  AND NOT EXISTS (
			SELECT 1 FROM event_members em
			WHERE em.event_id = $3 AND em.user_id = u.id
		  )
		  AND (u.username ILIKE $1 OR u.email ILIKE $1 OR COALESCE(p.full_name,'') ILIKE $1)
		ORDER BY
			CASE WHEN u.username ILIKE $2 THEN 0 WHEN u.email ILIKE $2 THEN 1 ELSE 2 END,
			u.username
		LIMIT $5
	`, pattern, q+"%", eventID, excludeUserID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []UserSearchResult
	for rows.Next() {
		var u UserSearchResult
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.FullName); err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

// SearchUsersGlobal matches username, email, or full name (for create-event invites).
func (r *Repository) SearchUsersGlobal(ctx context.Context, excludeUserID uuid.UUID, query string, limit int) ([]UserSearchResult, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	pattern := "%" + q + "%"
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.username, u.email, COALESCE(p.full_name, u.username)
		FROM users u
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE u.deleted_at IS NULL
		  AND u.id <> $3
		  AND (u.username ILIKE $1 OR u.email ILIKE $1 OR COALESCE(p.full_name,'') ILIKE $1)
		ORDER BY
			CASE WHEN u.username ILIKE $2 THEN 0 WHEN u.email ILIKE $2 THEN 1 ELSE 2 END,
			u.username
		LIMIT $4
	`, pattern, q+"%", excludeUserID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []UserSearchResult
	for rows.Next() {
		var u UserSearchResult
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.FullName); err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

func (r *Repository) ListMembers(ctx context.Context, eventID uuid.UUID) ([]MemberView, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT em.user_id, u.username, COALESCE(p.full_name, u.username), em.role, (e.creator_id = em.user_id)
		FROM event_members em
		JOIN users u ON u.id = em.user_id
		LEFT JOIN profiles p ON p.user_id = em.user_id
		JOIN events e ON e.id = em.event_id
		WHERE em.event_id = $1 AND u.deleted_at IS NULL
		ORDER BY
			CASE em.role WHEN 'host' THEN 0 WHEN 'co_host' THEN 1 ELSE 2 END,
			COALESCE(p.full_name, u.username), u.username
	`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []MemberView
	for rows.Next() {
		var m MemberView
		if err := rows.Scan(&m.UserID, &m.Username, &m.FullName, &m.Role, &m.IsCreator); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *Repository) InvitationStats(ctx context.Context, eventID uuid.UUID) (accepted, pending, declined, total int, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status='accepted'),
			COUNT(*) FILTER (WHERE status='pending'),
			COUNT(*) FILTER (WHERE status='declined'),
			COUNT(*)
		FROM event_invitations WHERE event_id=$1
	`, eventID).Scan(&accepted, &pending, &declined, &total)
	return
}

func (r *Repository) ListInvitationsPage(ctx context.Context, eventID uuid.UUID, page, perPage int) (InvitationListPage, error) {
	if perPage <= 0 {
		perPage = 10
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage
	var out InvitationListPage
	out.Page = page
	out.PerPage = perPage
	out.Accepted, out.Pending, out.Declined, out.Total, _ = r.InvitationStats(ctx, eventID)
	if out.Total == 0 {
		return out, nil
	}
	out.TotalPages = int(math.Ceil(float64(out.Total) / float64(perPage)))
	rows, err := r.pool.Query(ctx, `
		SELECT ei.id, ei.invitee_id, u.username, u.email, COALESCE(p.full_name, u.username), ei.status,
			ei.inviter_id, inv.username, COALESCE(ip.full_name, inv.username)
		FROM event_invitations ei
		JOIN users u ON u.id = ei.invitee_id
		LEFT JOIN profiles p ON p.user_id = ei.invitee_id
		JOIN users inv ON inv.id = ei.inviter_id
		LEFT JOIN profiles ip ON ip.user_id = ei.inviter_id
		WHERE ei.event_id=$1
		ORDER BY ei.created_at DESC
		LIMIT $2 OFFSET $3
	`, eventID, perPage, offset)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var v InvitationView
		if err := rows.Scan(&v.ID, &v.InviteeID, &v.Username, &v.Email, &v.FullName, &v.Status,
			&v.InviterID, &v.InviterUsername, &v.InviterName); err != nil {
			return out, err
		}
		out.Items = append(out.Items, v)
	}
	return out, nil
}

func (r *Repository) ListInvitations(ctx context.Context, eventID uuid.UUID) ([]InvitationView, error) {
	page, err := r.ListInvitationsPage(ctx, eventID, 1, 1000)
	return page.Items, err
}

func (r *Repository) ListHosts(ctx context.Context, eventID uuid.UUID) ([]HostView, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT em.user_id, u.username, COALESCE(p.full_name, u.username), em.role, (e.creator_id = em.user_id)
		FROM event_members em
		JOIN users u ON u.id = em.user_id
		LEFT JOIN profiles p ON p.user_id = em.user_id
		JOIN events e ON e.id = em.event_id
		WHERE em.event_id=$1 AND em.role IN ('host','co_host')
		ORDER BY CASE em.role WHEN 'host' THEN 0 ELSE 1 END, u.username
	`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []HostView
	for rows.Next() {
		var h HostView
		if err := rows.Scan(&h.UserID, &h.Username, &h.FullName, &h.Role, &h.IsCreator); err != nil {
			return nil, err
		}
		list = append(list, h)
	}
	return list, nil
}

func (r *Repository) ListBrowseUsersPage(ctx context.Context, eventID uuid.UUID, page, perPage int) (BrowseUsersPage, error) {
	if perPage <= 0 {
		perPage = 10
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage
	var out BrowseUsersPage
	out.Page = page
	out.PerPage = perPage
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users u WHERE u.deleted_at IS NULL`).Scan(&out.Total)
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.username, u.email, COALESCE(p.full_name, u.username),
			EXISTS(SELECT 1 FROM event_invitations ei WHERE ei.event_id=$1 AND ei.invitee_id=u.id),
			EXISTS(SELECT 1 FROM event_members em WHERE em.event_id=$1 AND em.user_id=u.id)
		FROM users u
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE u.deleted_at IS NULL
		ORDER BY COALESCE(p.full_name, u.username), u.username
		LIMIT $2 OFFSET $3
	`, eventID, perPage, offset)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var b BrowseUserView
		if err := rows.Scan(&b.ID, &b.Username, &b.Email, &b.FullName, &b.AlreadyInvited, &b.IsMember); err != nil {
			return out, err
		}
		out.Items = append(out.Items, b)
	}
	if out.Total > 0 {
		out.TotalPages = int(math.Ceil(float64(out.Total) / float64(perPage)))
	}
	return out, nil
}

func (r *Repository) PromoteCoHost(ctx context.Context, eventID, targetID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE event_members SET role='co_host'
		WHERE event_id=$1 AND user_id=$2 AND role='member'
	`, eventID, targetID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidState
	}
	return nil
}

func (r *Repository) CreateInvitation(ctx context.Context, eventID, inviterID, inviteeID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO event_invitations (event_id, inviter_id, invitee_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (event_id, invitee_id) DO UPDATE SET
			status = CASE WHEN event_invitations.status IN ('declined','pending') THEN 'pending' ELSE event_invitations.status END,
			inviter_id = EXCLUDED.inviter_id,
			responded_at = NULL
		RETURNING id
	`, eventID, inviterID, inviteeID).Scan(&id)
	return id, err
}

func (r *Repository) GetPendingInvitationID(ctx context.Context, eventID, userID uuid.UUID) (*uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM event_invitations
		WHERE event_id=$1 AND invitee_id=$2 AND status='pending'
	`, eventID, userID).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (r *Repository) InvitationStatus(ctx context.Context, invID, inviteeID uuid.UUID) (string, error) {
	var status string
	err := r.pool.QueryRow(ctx, `
		SELECT status FROM event_invitations WHERE id=$1 AND invitee_id=$2
	`, invID, inviteeID).Scan(&status)
	return status, err
}

func (r *Repository) AcceptInvitation(ctx context.Context, eventID, invID, userID uuid.UUID) error {
	if r.db == nil {
		return errors.New("database not configured")
	}
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		var inviteeID uuid.UUID
		var maxP, count int
		err := tx.QueryRow(ctx, `
			SELECT ei.invitee_id, e.max_participants,
				(SELECT COUNT(*) FROM event_members em WHERE em.event_id = e.id)
			FROM event_invitations ei
			JOIN events e ON e.id = ei.event_id
			WHERE ei.id=$1 AND ei.event_id=$2 AND ei.status='pending' AND e.status='published'
		`, invID, eventID).Scan(&inviteeID, &maxP, &count)
		if err != nil {
			return err
		}
		if inviteeID != userID {
			return ErrNotAuthorized
		}
		if count >= maxP {
			return ErrCapacityFull
		}
		tag, err := tx.Exec(ctx, `UPDATE event_invitations SET status='accepted', responded_at=now() WHERE id=$1`, invID)
		if err != nil || tag.RowsAffected() == 0 {
			return ErrInvalidState
		}
		_, err = tx.Exec(ctx, `INSERT INTO event_members (event_id, user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, eventID, userID)
		return err
	})
}

func (r *Repository) DeclineInvitation(ctx context.Context, invID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE event_invitations SET status='declined', responded_at=now()
		WHERE id=$1 AND invitee_id=$2 AND status='pending'
	`, invID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidState
	}
	return nil
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
	if e.MaxParticipants <= 0 {
		e.MaxParticipants = 20
	}
	cat, err := normalizeEventCategory(e.Category)
	if err != nil {
		return nil, err
	}
	e.Category = cat
	e.Title = strings.TrimSpace(e.Title)
	e.LocationName = strings.TrimSpace(e.LocationName)
	if err := validateEventFields(e); err != nil {
		return nil, err
	}
	n, err := s.repo.CountByCreator(ctx, e.CreatorID)
	if err != nil {
		return nil, err
	}
	if n >= MaxEventsPerCreator {
		return nil, ErrEventLimit
	}
	return s.repo.Create(ctx, e)
}

func (s *Service) Delete(ctx context.Context, eventID, creatorID uuid.UUID) error {
	return s.repo.DeleteByCreator(ctx, eventID, creatorID)
}

func (s *Service) Update(ctx context.Context, eventID, creatorID uuid.UUID, e Event) (*Event, error) {
	if e.MaxParticipants <= 0 {
		e.MaxParticipants = 20
	}
	if e.Visibility != "public" && e.Visibility != "private" {
		return nil, errors.New("visibility must be public or private")
	}
	cat, err := normalizeEventCategory(e.Category)
	if err != nil {
		return nil, err
	}
	e.Category = cat
	e.Title = strings.TrimSpace(e.Title)
	e.LocationName = strings.TrimSpace(e.LocationName)
	if err := validateEventFields(e); err != nil {
		return nil, err
	}
	return s.repo.UpdateByCreator(ctx, eventID, creatorID, e)
}

// Allowed event categories (event types).
var allowedEventCategories = map[string]string{
	"coffee":     "coffee",
	"meetup":     "meetup",
	"ride_share": "ride_share",
	"business":   "business",
}

func normalizeEventCategory(raw string) (string, error) {
	c := strings.TrimSpace(strings.ToLower(raw))
	if c == "" {
		return "", errors.New("event type is required")
	}
	if v, ok := allowedEventCategories[c]; ok {
		return v, nil
	}
	return "", errors.New("event type must be coffee, meetup, ride_share, or business")
}

func validateEventFields(e Event) error {
	if strings.TrimSpace(e.Title) == "" {
		return errors.New("event name is required")
	}
	if e.EventDate.IsZero() {
		return errors.New("date is required")
	}
	if strings.TrimSpace(e.LocationName) == "" {
		return errors.New("location is required")
	}
	if e.Visibility != "public" && e.Visibility != "private" {
		return errors.New("visibility must be public or private")
	}
	if strings.TrimSpace(e.StartTime) == "" {
		return errors.New("start time is required")
	}
	return nil
}

func (s *Service) CountByCreator(ctx context.Context, creatorID uuid.UUID) (int, error) {
	return s.repo.CountByCreator(ctx, creatorID)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Event, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context, viewerID uuid.UUID) ([]Event, error) {
	return s.repo.ListForViewer(ctx, viewerID, 50)
}

func (s *Service) ListHosted(ctx context.Context, userID uuid.UUID) ([]HostedEventSummary, error) {
	return s.repo.ListHostedByUser(ctx, userID)
}

func (s *Service) EventMapForHost(ctx context.Context, eventID, viewerID uuid.UUID) (*EventMapView, error) {
	sc, err := s.ShowContext(ctx, eventID, viewerID, 1, 1)
	if err != nil {
		return nil, err
	}
	if !sc.IsHost && !sc.IsMember {
		return nil, ErrNotAuthorized
	}
	e := &sc.Event
	if e.Latitude == 0 && e.Longitude == 0 {
		return nil, ErrInvalidState
	}
	members, err := s.repo.ListAcceptedMembersWithLocations(ctx, eventID)
	if err != nil {
		return nil, err
	}
	return &EventMapView{
		Event: HostedEventSummary{
			ID:           e.ID,
			Title:        e.Title,
			ShortName:    shortEventName(e.Title),
			LocationName: e.LocationName,
			Latitude:     e.Latitude,
			Longitude:    e.Longitude,
			EventDate:    e.EventDate.Format("Jan 02"),
			MemberCount:  e.MemberCount,
		},
		Members: members,
	}, nil
}

func (s *Service) ShowContext(ctx context.Context, eventID, viewerID uuid.UUID, invPage, _ int) (*ShowContext, error) {
	e, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	sc := &ShowContext{Event: *e}
	if viewerID == uuid.Nil {
		return sc, nil
	}
	sc.IsCreator = e.CreatorID == viewerID
	sc.IsHost, _ = s.repo.IsHost(ctx, eventID, viewerID)
	sc.IsMember, _ = s.repo.IsMember(ctx, eventID, viewerID)
	sc.HasPendingJoin, _ = s.repo.HasPendingJoinRequest(ctx, eventID, viewerID)
	sc.MyInvitationID, _ = s.repo.GetPendingInvitationID(ctx, eventID, viewerID)

	if sc.IsHost {
		sc.PendingJoinRequests, _ = s.repo.ListPendingJoinRequests(ctx, eventID)
		sc.Hosts, _ = s.repo.ListHosts(ctx, eventID)
		sc.Invitations, _ = s.repo.ListInvitationsPage(ctx, eventID, invPage, 10)
		sc.AcceptedInvites = sc.Invitations.Accepted
		sc.PendingInvites = sc.Invitations.Pending
		sc.TotalInvites = sc.Invitations.Total
	}
	return sc, nil
}

func (s *Service) assertHost(ctx context.Context, eventID, userID uuid.UUID) error {
	ok, err := s.repo.IsHost(ctx, eventID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotAuthorized
	}
	return nil
}

func (s *Service) JoinRequest(ctx context.Context, eventID, userID uuid.UUID, message string) error {
	e, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return err
	}
	if e.Status != "published" {
		return ErrInvalidState
	}
	if e.CreatorID == userID {
		return ErrCannotJoinOwn
	}
	host, _ := s.repo.IsHost(ctx, eventID, userID)
	if host {
		return ErrCannotJoinOwn
	}
	if e.Visibility == "private" {
		return ErrPrivateEvent
	}
	member, _ := s.repo.IsMember(ctx, eventID, userID)
	if member {
		return ErrAlreadyMember
	}
	pending, _ := s.repo.HasPendingJoinRequest(ctx, eventID, userID)
	if pending {
		return ErrInvalidState
	}
	return s.repo.CreateJoinRequest(ctx, eventID, userID, message)
}

func (s *Service) AcceptJoin(ctx context.Context, eventID, reqID, hostID uuid.UUID) (uuid.UUID, error) {
	if err := s.assertHost(ctx, eventID, hostID); err != nil {
		return uuid.Nil, err
	}
	return s.repo.AcceptJoin(ctx, eventID, reqID)
}

func (s *Service) RejectJoin(ctx context.Context, eventID, reqID, hostID uuid.UUID) error {
	if err := s.assertHost(ctx, eventID, hostID); err != nil {
		return err
	}
	return s.repo.RejectJoin(ctx, reqID)
}

func (s *Service) SearchUsers(ctx context.Context, eventID, excludeUserID uuid.UUID, query string) ([]UserSearchResult, error) {
	return s.repo.SearchUsers(ctx, eventID, excludeUserID, query, 10)
}

func (s *Service) SearchUsersGlobal(ctx context.Context, excludeUserID uuid.UUID, query string) ([]UserSearchResult, error) {
	return s.repo.SearchUsersGlobal(ctx, excludeUserID, query, 10)
}

func (s *Service) ListMembers(ctx context.Context, eventID, viewerID uuid.UUID) ([]MemberView, error) {
	sc, err := s.ShowContext(ctx, eventID, viewerID, 1, 1)
	if err != nil {
		return nil, err
	}
	// Public events: anyone can see the going list. Private: members/hosts/invitees only.
	if sc.Event.Visibility == "private" && !sc.IsHost && !sc.IsMember && sc.MyInvitationID == nil {
		return nil, ErrNotAuthorized
	}
	return s.repo.ListMembers(ctx, eventID)
}

// PeopleRoster is the going list plus pending invites for the event detail panel.
type PeopleRoster struct {
	Going   []MemberView     `json:"going"`
	Invited []InvitationView `json:"invited"`
	IsHost  bool             `json:"is_host"`
}

func (s *Service) ListPeople(ctx context.Context, eventID, viewerID uuid.UUID) (*PeopleRoster, error) {
	sc, err := s.ShowContext(ctx, eventID, viewerID, 1, 1)
	if err != nil {
		return nil, err
	}
	if sc.Event.Visibility == "private" && !sc.IsHost && !sc.IsMember && sc.MyInvitationID == nil {
		return nil, ErrNotAuthorized
	}
	going, err := s.repo.ListMembers(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if going == nil {
		going = []MemberView{}
	}
	out := &PeopleRoster{Going: going, Invited: []InvitationView{}, IsHost: sc.IsHost}
	// Invites: hosts always see them; members of public/private events see pending invitees (no emails).
	invites, err := s.repo.ListInvitations(ctx, eventID)
	if err != nil {
		return out, nil
	}
	for _, inv := range invites {
		if inv.Status != "pending" {
			continue
		}
		if !sc.IsHost {
			inv.Email = ""
		}
		out.Invited = append(out.Invited, inv)
	}
	return out, nil
}

func (s *Service) InviteUser(ctx context.Context, eventID, hostID, inviteeID uuid.UUID) (uuid.UUID, string, error) {
	if err := s.assertHost(ctx, eventID, hostID); err != nil {
		return uuid.Nil, "", err
	}
	if inviteeID == hostID {
		return uuid.Nil, "", errors.New("cannot invite yourself")
	}
	member, _ := s.repo.IsMember(ctx, eventID, inviteeID)
	if member {
		return uuid.Nil, "", ErrAlreadyMember
	}
	var name string
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(p.full_name, u.username) FROM users u
		LEFT JOIN profiles p ON p.user_id=u.id WHERE u.id=$1
	`, inviteeID).Scan(&name); err != nil {
		return uuid.Nil, "", ErrUserNotFound
	}
	invID, err := s.repo.CreateInvitation(ctx, eventID, hostID, inviteeID)
	return invID, name, err
}

func (s *Service) InviteByUsername(ctx context.Context, eventID, hostID uuid.UUID, query string) (invID, inviteeID uuid.UUID, fullName string, err error) {
	if err := s.assertHost(ctx, eventID, hostID); err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}
	inviteeID, fullName, err = s.repo.LookupUserByQuery(ctx, query)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", ErrUserNotFound
	}
	if inviteeID == hostID {
		return uuid.Nil, uuid.Nil, "", errors.New("cannot invite yourself")
	}
	member, _ := s.repo.IsMember(ctx, eventID, inviteeID)
	if member {
		return uuid.Nil, uuid.Nil, "", ErrAlreadyMember
	}
	invID, err = s.repo.CreateInvitation(ctx, eventID, hostID, inviteeID)
	return invID, inviteeID, fullName, err
}

func (s *Service) InviteUsersBatch(ctx context.Context, eventID, hostID uuid.UUID, userIDs []uuid.UUID) (int, []string) {
	var sent int
	var errs []string
	seen := map[uuid.UUID]bool{}
	for _, id := range userIDs {
		if id == uuid.Nil || seen[id] {
			continue
		}
		seen[id] = true
		invID, _, err := s.InviteUser(ctx, eventID, hostID, id)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if invID != uuid.Nil {
			sent++
		}
	}
	return sent, errs
}

func (s *Service) PromoteCoHost(ctx context.Context, eventID, actorID, targetID uuid.UUID) error {
	if err := s.assertHost(ctx, eventID, actorID); err != nil {
		return err
	}
	e, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return err
	}
	if targetID == e.CreatorID {
		return ErrInvalidState
	}
	member, _ := s.repo.IsMember(ctx, eventID, targetID)
	if !member {
		return ErrInvalidState
	}
	return s.repo.PromoteCoHost(ctx, eventID, targetID)
}

func (s *Service) AcceptInvitation(ctx context.Context, eventID, invID, userID uuid.UUID) error {
	return s.repo.AcceptInvitation(ctx, eventID, invID, userID)
}

func (s *Service) DeclineInvitation(ctx context.Context, invID, userID uuid.UUID) error {
	return s.repo.DeclineInvitation(ctx, invID, userID)
}

func (s *Service) InvitationStatus(ctx context.Context, invID, inviteeID uuid.UUID) (string, error) {
	return s.repo.InvitationStatus(ctx, invID, inviteeID)
}

func (s *Service) ToggleLiveConsent(ctx context.Context, eventID, userID uuid.UUID, consented bool) error {
	var member bool
	_ = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM event_members WHERE event_id=$1 AND user_id=$2)`, eventID, userID).Scan(&member)
	if !member {
		return ErrNotAuthorized
	}
	return s.repo.ToggleLiveConsent(ctx, eventID, userID, consented)
}
