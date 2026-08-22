package profile

import (
	"time"

	"github.com/google/uuid"
)

type Profile struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	FullName            string
	Bio                 string
	About               string
	Company             string
	JobTitle            string
	ExperienceYears     *int
	ProfessionID        *uuid.UUID
	ProfessionName      string
	ProfilePhotoURL     string
	CoverPhotoURL       string
	AvailabilityStatus  string
	LookingFor          []string
	IsVerified          bool
	OnboardingCompleted bool
	Username            string
	CreatedAt           time.Time
}

type UpdateInput struct {
	FullName           string
	Bio                string
	About              string
	Company            string
	JobTitle           string
	ExperienceYears    *int
	ProfessionID       *uuid.UUID
	AvailabilityStatus string
	LookingFor         []string
}

type Tag struct {
	ID   uuid.UUID
	Name string
	Slug string
}

type Skill struct {
	ID   uuid.UUID
	Name string
}

type SocialLink struct {
	Platform string
	URL      string
}
