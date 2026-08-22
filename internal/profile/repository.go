package profile

import (
	"context"
	"errors"
	"fmt"

	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PgRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PgRepository {
	return &PgRepository{pool: pool}
}

func (r *PgRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*Profile, error) {
	var p Profile
	var profName *string
	err := r.pool.QueryRow(ctx, `
		SELECT p.id, p.user_id, p.full_name, COALESCE(p.bio,''), COALESCE(p.about,''),
			COALESCE(p.company,''), COALESCE(p.job_title,''), p.experience_years, p.profession_id,
			pr.name, p.availability_status, p.looking_for, p.is_verified, p.onboarding_completed, u.username
		FROM profiles p
		JOIN users u ON u.id = p.user_id
		LEFT JOIN professions pr ON pr.id = p.profession_id
		WHERE p.user_id = $1 AND u.deleted_at IS NULL
	`, userID).Scan(&p.ID, &p.UserID, &p.FullName, &p.Bio, &p.About, &p.Company, &p.JobTitle,
		&p.ExperienceYears, &p.ProfessionID, &profName, &p.AvailabilityStatus, &p.LookingFor,
		&p.IsVerified, &p.OnboardingCompleted, &p.Username)
	if err != nil {
		return nil, err
	}
	if profName != nil {
		p.ProfessionName = *profName
	}
	return &p, nil
}

func (r *PgRepository) GetByUsername(ctx context.Context, username string) (*Profile, error) {
	var userID uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT id FROM users WHERE username = $1 AND deleted_at IS NULL`, username).Scan(&userID)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *PgRepository) Update(ctx context.Context, userID uuid.UUID, in UpdateInput) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE profiles SET full_name=$2, bio=$3, about=$4, company=$5, job_title=$6,
			experience_years=$7, profession_id=$8, availability_status=$9, looking_for=$10, updated_at=now()
		WHERE user_id=$1
	`, userID, in.FullName, in.Bio, in.About, in.Company, in.JobTitle, in.ExperienceYears,
		in.ProfessionID, in.AvailabilityStatus, in.LookingFor)
	return err
}

func (r *PgRepository) CompleteOnboarding(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE profiles SET onboarding_completed=true, updated_at=now() WHERE user_id=$1`, userID)
	return err
}

func (r *PgRepository) SetProfilePhoto(ctx context.Context, userID, fileID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE profiles SET profile_photo_file_id=$2, updated_at=now() WHERE user_id=$1`, userID, fileID)
	return err
}

func (r *PgRepository) ListProfessions(ctx context.Context) ([]Tag, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, slug FROM professions ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, nil
}

func (r *PgRepository) GetTags(ctx context.Context, userID uuid.UUID) ([]Tag, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.name, t.slug FROM tags t
		JOIN user_tags ut ON ut.tag_id = t.id WHERE ut.user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Tag
	for rows.Next() {
		var t Tag
		_ = rows.Scan(&t.ID, &t.Name, &t.Slug)
		list = append(list, t)
	}
	return list, nil
}

func (r *PgRepository) AddTag(ctx context.Context, userID uuid.UUID, name string) error {
	slug := slugify(name)
	var tagID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO tags (name, slug, created_by) VALUES ($1, $2, $3)
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, name, slug, userID).Scan(&tagID)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO user_tags (user_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, tagID)
	return err
}

func (r *PgRepository) GetSkills(ctx context.Context, userID uuid.UUID) ([]Skill, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.name FROM skills s JOIN user_skills us ON us.skill_id = s.id WHERE us.user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Skill
	for rows.Next() {
		var s Skill
		_ = rows.Scan(&s.ID, &s.Name)
		list = append(list, s)
	}
	return list, nil
}

func (r *PgRepository) AddSkill(ctx context.Context, userID uuid.UUID, name string) error {
	var skillID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO skills (name, created_by) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id
	`, name, userID).Scan(&skillID)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO user_skills (user_id, skill_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, skillID)
	return err
}

func slugify(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			out = append(out, c)
		} else if c >= 'A' && c <= 'Z' {
			out = append(out, c+32)
		} else if c == ' ' || c == '_' {
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return "tag"
	}
	return string(out)
}

type Service struct {
	repo *PgRepository
	log  *logger.Logger
}

func NewService(repo *PgRepository, log *logger.Logger) *Service {
	return &Service{repo: repo, log: log}
}

func (s *Service) GetOwn(ctx context.Context, userID uuid.UUID) (*Profile, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *Service) GetPublic(ctx context.Context, username string) (*Profile, error) {
	return s.repo.GetByUsername(ctx, username)
}

func (s *Service) Update(ctx context.Context, userID uuid.UUID, in UpdateInput) error {
	if in.FullName == "" {
		return fmt.Errorf("full name is required")
	}
	return s.repo.Update(ctx, userID, in)
}

func (s *Service) CompleteOnboarding(ctx context.Context, userID uuid.UUID) error {
	return s.repo.CompleteOnboarding(ctx, userID)
}

func (s *Service) ListProfessions(ctx context.Context) ([]Tag, error) {
	return s.repo.ListProfessions(ctx)
}

func (s *Service) GetTags(ctx context.Context, userID uuid.UUID) ([]Tag, error) {
	return s.repo.GetTags(ctx, userID)
}

func (s *Service) AddTag(ctx context.Context, userID uuid.UUID, name string) error {
	if name == "" {
		return errors.New("tag name required")
	}
	return s.repo.AddTag(ctx, userID, name)
}

func (s *Service) GetSkills(ctx context.Context, userID uuid.UUID) ([]Skill, error) {
	return s.repo.GetSkills(ctx, userID)
}

func (s *Service) AddSkill(ctx context.Context, userID uuid.UUID, name string) error {
	if name == "" {
		return errors.New("skill name required")
	}
	return s.repo.AddSkill(ctx, userID, name)
}

func (s *Service) SetProfilePhoto(ctx context.Context, userID, fileID uuid.UUID) error {
	return s.repo.SetProfilePhoto(ctx, userID, fileID)
}

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
