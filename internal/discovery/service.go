package discovery

import (
	"context"
	"math"
	"strconv"

	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NearbyUser struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	FullName     string    `json:"full_name"`
	Profession   string    `json:"profession"`
	Latitude     float64   `json:"lat"`
	Longitude    float64   `json:"lng"`
	DistanceKm   float64   `json:"distance_km"`
	Availability string    `json:"availability"`
	Online       bool      `json:"online"`
	Initials     string    `json:"initials"`
	MarkerType   string    `json:"marker_type"`
	IsMe         bool      `json:"is_me"`
	Live         bool      `json:"live"`
}

type Filters struct {
	Lat          float64
	Lng          float64
	RadiusKm     float64
	Profession   string
	Availability string
	OnlineOnly   bool
	OfflineOnly  bool
	Query        string
	Limit        int
	Offset       int
}

type Service struct {
	pool *pgxpool.Pool
	log  *logger.Logger
}

func NewService(pool *pgxpool.Pool, log *logger.Logger) *Service {
	return &Service{pool: pool, log: log}
}

func (s *Service) Nearby(ctx context.Context, viewerID uuid.UUID, f Filters) ([]NearbyUser, error) {
	if f.RadiusKm <= 0 {
		f.RadiusKm = 50
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 100
	}
	radiusM := f.RadiusKm * 1000

	q := `
		SELECT u.id, u.username, p.full_name, COALESCE(pr.name,''),
			COALESCE(
				(SELECT ll.latitude FROM live_locations ll
				 WHERE ll.user_id = u.id AND ll.event_id IS NULL
				   AND ls.live_sharing_enabled = true
				   AND ll.updated_at > now() - interval '5 minutes'
				 ORDER BY ll.updated_at DESC LIMIT 1),
				ST_Y(ul.fuzzed_geom::geometry)
			),
			COALESCE(
				(SELECT ll.longitude FROM live_locations ll
				 WHERE ll.user_id = u.id AND ll.event_id IS NULL
				   AND ls.live_sharing_enabled = true
				   AND ll.updated_at > now() - interval '5 minutes'
				 ORDER BY ll.updated_at DESC LIMIT 1),
				ST_X(ul.fuzzed_geom::geometry)
			),
			ST_Distance(ul.fuzzed_geom, ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography) / 1000.0,
			p.availability_status,
			EXISTS (
				SELECT 1 FROM live_locations ll
				WHERE ll.user_id = u.id AND ll.event_id IS NULL
				  AND ls.live_sharing_enabled = true
				  AND ll.updated_at > now() - interval '5 minutes'
			) AS is_live
		FROM users u
		JOIN profiles p ON p.user_id = u.id
		JOIN user_locations ul ON ul.user_id = u.id
		JOIN location_settings ls ON ls.user_id = u.id
		LEFT JOIN professions pr ON pr.id = p.profession_id
		WHERE u.deleted_at IS NULL
		  AND ($3::uuid IS NULL OR u.id != $3)
		  AND ls.discovery_enabled = true
		  AND ls.sharing_mode IN ('approximate', 'discoverable')
		  AND ul.fuzzed_geom IS NOT NULL
		  AND ST_DWithin(ul.fuzzed_geom, ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography, $4)
		  AND (
			$3::uuid IS NULL OR NOT EXISTS (
				SELECT 1 FROM blocks b
				WHERE (b.blocker_id = u.id AND b.blocked_id = $3) OR (b.blocker_id = $3 AND b.blocked_id = u.id)
			)
		  )
	`
	var viewer any
	if viewerID != uuid.Nil {
		viewer = viewerID
	}
	args := []any{f.Lat, f.Lng, viewer, radiusM}
	argN := 5

	if f.Profession != "" {
		q += ` AND pr.slug = $` + strconv.Itoa(argN)
		args = append(args, f.Profession)
		argN++
	}
	if f.Availability != "" {
		q += ` AND p.availability_status = $` + strconv.Itoa(argN)
		args = append(args, f.Availability)
		argN++
	}
	if f.OnlineOnly {
		q += ` AND p.availability_status IN ('available','busy','away')`
	}
	if f.OfflineOnly {
		q += ` AND p.availability_status = 'offline'`
	}
	if f.Query != "" {
		q += ` AND (p.full_name ILIKE $` + strconv.Itoa(argN) + ` OR u.username ILIKE $` + strconv.Itoa(argN) + ` OR COALESCE(pr.name,'') ILIKE $` + strconv.Itoa(argN) + ` OR COALESCE(ul.city,'') ILIKE $` + strconv.Itoa(argN) + `)`
		args = append(args, "%"+f.Query+"%")
		argN++
	}

	q += ` ORDER BY 7 LIMIT $` + strconv.Itoa(argN) + ` OFFSET $` + strconv.Itoa(argN+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []NearbyUser
	for rows.Next() {
		var u NearbyUser
		var isLive bool
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Profession, &u.Latitude, &u.Longitude, &u.DistanceKm, &u.Availability, &isLive); err != nil {
			return nil, err
		}
		u.Live = isLive
		u.MarkerType = "approximate"
		if isLive {
			u.MarkerType = "live"
		}
		u.Online = u.Availability != "" && u.Availability != "offline"
		u.Initials = initials(u.FullName)
		u.DistanceKm = math.Round(u.DistanceKm*10) / 10
		list = append(list, u)
	}
	return list, nil
}

// MyMapMarker always returns the logged-in user's marker (exact permanent, or live if sharing).
func (s *Service) MyMapMarker(ctx context.Context, userID uuid.UUID, viewLat, viewLng float64) (*NearbyUser, error) {
	var u NearbyUser
	var isLive bool
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, p.full_name, COALESCE(pr.name,''),
			COALESCE(
				(SELECT ll.latitude FROM live_locations ll
				 WHERE ll.user_id = u.id AND ll.event_id IS NULL
				   AND ls.live_sharing_enabled = true
				   AND ll.updated_at > now() - interval '5 minutes'
				 ORDER BY ll.updated_at DESC LIMIT 1),
				ST_Y(ul.geom::geometry)
			),
			COALESCE(
				(SELECT ll.longitude FROM live_locations ll
				 WHERE ll.user_id = u.id AND ll.event_id IS NULL
				   AND ls.live_sharing_enabled = true
				   AND ll.updated_at > now() - interval '5 minutes'
				 ORDER BY ll.updated_at DESC LIMIT 1),
				ST_X(ul.geom::geometry)
			),
			ST_Distance(
				COALESCE(ul.geom, ul.fuzzed_geom),
				ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography
			) / 1000.0,
			p.availability_status,
			EXISTS (
				SELECT 1 FROM live_locations ll
				WHERE ll.user_id = u.id AND ll.event_id IS NULL
				  AND ls.live_sharing_enabled = true
				  AND ll.updated_at > now() - interval '5 minutes'
			)
		FROM users u
		JOIN profiles p ON p.user_id = u.id
		JOIN user_locations ul ON ul.user_id = u.id
		JOIN location_settings ls ON ls.user_id = u.id
		LEFT JOIN professions pr ON pr.id = p.profession_id
		WHERE u.id = $3 AND u.deleted_at IS NULL AND ul.geom IS NOT NULL
	`, viewLat, viewLng, userID).Scan(
		&u.ID, &u.Username, &u.FullName, &u.Profession,
		&u.Latitude, &u.Longitude, &u.DistanceKm, &u.Availability, &isLive,
	)
	if err != nil {
		return nil, err
	}
	u.IsMe = true
	u.Live = isLive
	u.MarkerType = "me"
	if isLive {
		u.MarkerType = "live"
	}
	u.Online = true
	if u.Availability == "offline" {
		u.Online = false
	}
	u.Initials = initials(u.FullName)
	u.DistanceKm = math.Round(u.DistanceKm*10) / 10
	return &u, nil
}

func (s *Service) CountDiscoverable(ctx context.Context) (total, online int, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE true),
			COUNT(*) FILTER (WHERE p.availability_status IN ('available','busy','away'))
		FROM users u
		JOIN profiles p ON p.user_id = u.id
		JOIN user_locations ul ON ul.user_id = u.id
		JOIN location_settings ls ON ls.user_id = u.id
		WHERE u.deleted_at IS NULL
		  AND ls.discovery_enabled = true
		  AND ls.sharing_mode IN ('approximate', 'discoverable')
		  AND ul.fuzzed_geom IS NOT NULL
	`).Scan(&total, &online)
	return
}

func initials(name string) string {
	runes := []rune(name)
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
	if len(out) > 2 {
		return out[:2]
	}
	return out
}
