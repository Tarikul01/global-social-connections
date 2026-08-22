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
	Email               string
	PitchURL            string
	PersonalQuote       string
	ProfileViews        int
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
	PitchURL           string
	PersonalQuote      string
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

type Stats struct {
	Following    int
	Followers    int
	Views        int
	Interactions int
}

type Friend struct {
	UserID   uuid.UUID
	Username string
	FullName string
}

type Address struct {
	City    string
	Region  string
	Country string
	Lat     float64
	Lng     float64
	OK      bool
}
