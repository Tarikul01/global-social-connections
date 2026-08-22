package services

import (
	"context"
	"errors"

	"github.com/findneighbour/findneighbour/internal/discovery"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/profile"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// UserService is the shared domain layer used by both HTML and JSON controllers.
// It reuses discovery (PostGIS nearby) and profile repositories — no duplicated queries.
type UserService struct {
	discovery *discovery.Service
	profiles  *profile.Service
	log       *logger.Logger
}

func NewUserService(disc *discovery.Service, profiles *profile.Service, log *logger.Logger) *UserService {
	return &UserService{discovery: disc, profiles: profiles, log: log}
}

// NearbyFilters mirrors discovery filters for API/web handlers.
type NearbyFilters = discovery.Filters

// NearbyUser is the shared nearby-user DTO (JSON + HTML).
type NearbyUser = discovery.NearbyUser

// PublicUser is a privacy-safe profile payload for mobile/API clients.
type PublicUser struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	FullName     string    `json:"full_name"`
	Bio          string    `json:"bio,omitempty"`
	Profession   string    `json:"profession,omitempty"`
	Company      string    `json:"company,omitempty"`
	JobTitle     string    `json:"job_title,omitempty"`
	Availability string    `json:"availability"`
	Online       bool      `json:"online"`
	Verified     bool      `json:"verified"`
}

func (s *UserService) Nearby(ctx context.Context, viewerID uuid.UUID, f NearbyFilters) ([]NearbyUser, error) {
	return s.discovery.Nearby(ctx, viewerID, f)
}

func (s *UserService) CountDiscoverable(ctx context.Context) (total, online int, err error) {
	return s.discovery.CountDiscoverable(ctx)
}

func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*PublicUser, error) {
	p, err := s.profiles.GetOwn(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return toPublicUser(p), nil
}

var ErrUserNotFound = errors.New("user not found")

func toPublicUser(p *profile.Profile) *PublicUser {
	return &PublicUser{
		ID:           p.UserID,
		Username:     p.Username,
		FullName:     p.FullName,
		Bio:          p.Bio,
		Profession:   p.ProfessionName,
		Company:      p.Company,
		JobTitle:     p.JobTitle,
		Availability: p.AvailabilityStatus,
		Online:       p.AvailabilityStatus != "" && p.AvailabilityStatus != "offline",
		Verified:     p.IsVerified,
	}
}
