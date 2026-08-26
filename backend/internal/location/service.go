package location

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Settings struct {
	UserID             uuid.UUID
	SharingMode        string
	DiscoveryEnabled   bool
	LiveSharingEnabled bool
	LiveSharingScope   string
	ShowOnMap          bool
}

type LiveLocation struct {
	UserID    uuid.UUID
	EventID   *uuid.UUID
	Latitude  float64
	Longitude float64
	UpdatedAt time.Time
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetSettings(ctx context.Context, userID uuid.UUID) (*Settings, error) {
	var s Settings
	err := r.pool.QueryRow(ctx, `
		SELECT user_id, sharing_mode, discovery_enabled, live_sharing_enabled, live_sharing_scope, show_on_map
		FROM location_settings WHERE user_id=$1
	`, userID).Scan(&s.UserID, &s.SharingMode, &s.DiscoveryEnabled, &s.LiveSharingEnabled, &s.LiveSharingScope, &s.ShowOnMap)
	return &s, err
}

func (r *Repository) UpdateSettings(ctx context.Context, userID uuid.UUID, s Settings) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE location_settings SET sharing_mode=$2, discovery_enabled=$3, live_sharing_enabled=$4,
			live_sharing_scope=$5, show_on_map=$6, updated_at=now() WHERE user_id=$1
	`, userID, s.SharingMode, s.DiscoveryEnabled, s.LiveSharingEnabled, s.LiveSharingScope, s.ShowOnMap)
	return err
}

func (r *Repository) UpsertLocation(ctx context.Context, userID uuid.UUID, lat, lng float64, city, region, country string) error {
	fuzzLat, fuzzLng := fuzzCoordinates(lat, lng)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_locations (user_id, city, region, country, geom, fuzzed_geom, updated_at)
		VALUES ($1, $2, $3, $4,
			ST_SetSRID(ST_MakePoint($6, $5), 4326)::geography,
			ST_SetSRID(ST_MakePoint($8, $7), 4326)::geography,
			now())
		ON CONFLICT (user_id) DO UPDATE SET
			city=$2, region=$3, country=$4,
			geom=ST_SetSRID(ST_MakePoint($6, $5), 4326)::geography,
			fuzzed_geom=ST_SetSRID(ST_MakePoint($8, $7), 4326)::geography,
			updated_at=now()
	`, userID, city, region, country, lat, lng, fuzzLat, fuzzLng)
	return err
}

func (r *Repository) UpsertLiveLocation(ctx context.Context, userID uuid.UUID, eventID *uuid.UUID, lat, lng float64) error {
	if eventID == nil {
		_, _ = r.pool.Exec(ctx, `DELETE FROM live_locations WHERE user_id=$1 AND event_id IS NULL`, userID)
		_, err := r.pool.Exec(ctx, `
			INSERT INTO live_locations (user_id, event_id, latitude, longitude, updated_at)
			VALUES ($1, NULL, $2, $3, now())
		`, userID, lat, lng)
		return err
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO live_locations (user_id, event_id, latitude, longitude, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (user_id, event_id) DO UPDATE SET latitude=$3, longitude=$4, updated_at=now()
	`, userID, *eventID, lat, lng)
	return err
}

func (r *Repository) GetPermanentLocation(ctx context.Context, userID uuid.UUID) (lat, lng float64, city string, ok bool, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT ST_Y(geom::geometry), ST_X(geom::geometry), COALESCE(city,'')
		FROM user_locations WHERE user_id=$1 AND geom IS NOT NULL
	`, userID).Scan(&lat, &lng, &city)
	if err != nil {
		return 0, 0, "", false, nil
	}
	return lat, lng, city, true, nil
}

func (r *Repository) GetActiveLiveLocation(ctx context.Context, userID uuid.UUID) (lat, lng float64, ok bool, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT latitude, longitude FROM live_locations
		WHERE user_id=$1 AND event_id IS NULL AND updated_at > now() - interval '5 minutes'
		ORDER BY updated_at DESC LIMIT 1
	`, userID).Scan(&lat, &lng)
	if err != nil {
		return 0, 0, false, nil
	}
	return lat, lng, true, nil
}

func (r *Repository) DeleteLiveLocation(ctx context.Context, userID uuid.UUID, eventID *uuid.UUID) error {
	if eventID != nil {
		_, err := r.pool.Exec(ctx, `DELETE FROM live_locations WHERE user_id=$1 AND event_id=$2`, userID, *eventID)
		return err
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM live_locations WHERE user_id=$1`, userID)
	return err
}

func (r *Repository) GetEventLiveLocations(ctx context.Context, eventID uuid.UUID) ([]LiveLocation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ll.user_id, ll.latitude, ll.longitude, ll.updated_at
		FROM live_locations ll
		JOIN event_live_consent elc ON elc.user_id = ll.user_id AND elc.event_id = $1
		JOIN event_members em ON em.user_id = ll.user_id AND em.event_id = $1
		WHERE ll.event_id = $1 AND elc.consented = true
		  AND ll.updated_at > now() - interval '60 seconds'
	`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []LiveLocation
	for rows.Next() {
		var ll LiveLocation
		eid := eventID
		ll.EventID = &eid
		_ = rows.Scan(&ll.UserID, &ll.Latitude, &ll.Longitude, &ll.UpdatedAt)
		list = append(list, ll)
	}
	return list, nil
}

func fuzzCoordinates(lat, lng float64) (float64, float64) {
	offsetM := 200 + rand.Float64()*300
	angle := rand.Float64() * 2 * math.Pi
	dLat := (offsetM / 111000) * math.Cos(angle)
	dLng := (offsetM / (111000 * math.Cos(lat*math.Pi/180))) * math.Sin(angle)
	return lat + dLat, lng + dLng
}

type Service struct {
	repo *Repository
	log  *logger.Logger
}

func NewService(repo *Repository, log *logger.Logger) *Service {
	return &Service{repo: repo, log: log}
}

func (s *Service) GetSettings(ctx context.Context, userID uuid.UUID) (*Settings, error) {
	return s.repo.GetSettings(ctx, userID)
}

func (s *Service) UpdateSettings(ctx context.Context, userID uuid.UUID, settings Settings) error {
	allowed := map[string]bool{"hidden": true, "city_only": true, "approximate": true, "discoverable": true}
	if !allowed[settings.SharingMode] {
		settings.SharingMode = "hidden"
	}
	return s.repo.UpdateSettings(ctx, userID, settings)
}

func (s *Service) UpdateLocation(ctx context.Context, userID uuid.UUID, lat, lng float64, city, region, country string) error {
	return s.repo.UpsertLocation(ctx, userID, lat, lng, city, region, country)
}

func (s *Service) GetPermanentLocation(ctx context.Context, userID uuid.UUID) (lat, lng float64, city string, ok bool, err error) {
	return s.repo.GetPermanentLocation(ctx, userID)
}

func (s *Service) EnableLiveSharing(ctx context.Context, userID uuid.UUID) error {
	sSettings, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		return err
	}
	sSettings.LiveSharingEnabled = true
	sSettings.LiveSharingScope = "public"
	sSettings.DiscoveryEnabled = true
	sSettings.ShowOnMap = true
	if sSettings.SharingMode == "hidden" {
		sSettings.SharingMode = "discoverable"
	}
	return s.repo.UpdateSettings(ctx, userID, *sSettings)
}

func (s *Service) DisableLiveSharing(ctx context.Context, userID uuid.UUID) error {
	sSettings, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		return err
	}
	sSettings.LiveSharingEnabled = false
	sSettings.LiveSharingScope = "none"
	_ = s.repo.DeleteLiveLocation(ctx, userID, nil)
	return s.repo.UpdateSettings(ctx, userID, *sSettings)
}

func (s *Service) UpdateLiveLocation(ctx context.Context, userID uuid.UUID, eventID *uuid.UUID, lat, lng float64) error {
	settings, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		return err
	}
	if !settings.LiveSharingEnabled {
		if err := s.EnableLiveSharing(ctx, userID); err != nil {
			return err
		}
	}
	return s.repo.UpsertLiveLocation(ctx, userID, eventID, lat, lng)
}

func (s *Service) StopLiveLocation(ctx context.Context, userID uuid.UUID, eventID *uuid.UUID) error {
	if eventID == nil {
		return s.DisableLiveSharing(ctx, userID)
	}
	return s.repo.DeleteLiveLocation(ctx, userID, eventID)
}

func (s *Service) IsLiveSharing(ctx context.Context, userID uuid.UUID) bool {
	sSettings, err := s.repo.GetSettings(ctx, userID)
	return err == nil && sSettings.LiveSharingEnabled
}

// StartLive / UpdateLive / StopLive implement realtime.LiveLocationHandler for WebSocket.
func (s *Service) StartLive(ctx context.Context, userID uuid.UUID, lat, lng float64) error {
	if err := s.EnableLiveSharing(ctx, userID); err != nil {
		return err
	}
	if lat != 0 || lng != 0 {
		return s.UpdateLiveLocation(ctx, userID, nil, lat, lng)
	}
	return nil
}

func (s *Service) UpdateLive(ctx context.Context, userID uuid.UUID, lat, lng float64) error {
	return s.UpdateLiveLocation(ctx, userID, nil, lat, lng)
}

func (s *Service) StopLive(ctx context.Context, userID uuid.UUID) error {
	return s.StopLiveLocation(ctx, userID, nil)
}

func (s *Service) ActiveLiveOrPermanent(ctx context.Context, userID uuid.UUID) (lat, lng float64, live bool, ok bool) {
	if flat, flng, lok, _ := s.repo.GetActiveLiveLocation(ctx, userID); lok {
		return flat, flng, true, true
	}
	plat, plng, _, pok, _ := s.repo.GetPermanentLocation(ctx, userID)
	return plat, plng, false, pok
}

func (s *Service) EventLiveLocations(ctx context.Context, eventID uuid.UUID) ([]LiveLocation, error) {
	return s.repo.GetEventLiveLocations(ctx, eventID)
}
