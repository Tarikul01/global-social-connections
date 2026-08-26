package profile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	var birthday *time.Time
	var photoPath string
	err := r.pool.QueryRow(ctx, `
		SELECT p.id, p.user_id, p.full_name, COALESCE(p.bio,''), COALESCE(p.about,''),
			COALESCE(p.company,''), COALESCE(p.job_title,''), p.experience_years, p.profession_id,
			pr.name, p.availability_status, p.looking_for, p.is_verified, p.onboarding_completed,
			u.username, u.email,
			COALESCE(p.pitch_url,''), COALESCE(p.personal_quote,''), COALESCE(p.profile_views_count,0),
			COALESCE(p.gender,''), p.birthday,
			COALESCE(f.storage_path, '')
		FROM profiles p
		JOIN users u ON u.id = p.user_id
		LEFT JOIN professions pr ON pr.id = p.profession_id
		LEFT JOIN files f ON f.id = p.profile_photo_file_id
		WHERE p.user_id = $1 AND u.deleted_at IS NULL
	`, userID).Scan(&p.ID, &p.UserID, &p.FullName, &p.Bio, &p.About, &p.Company, &p.JobTitle,
		&p.ExperienceYears, &p.ProfessionID, &profName, &p.AvailabilityStatus, &p.LookingFor,
		&p.IsVerified, &p.OnboardingCompleted, &p.Username, &p.Email,
		&p.PitchURL, &p.PersonalQuote, &p.ProfileViews, &p.Gender, &birthday, &photoPath)
	if err != nil {
		return nil, err
	}
	if profName != nil {
		p.ProfessionName = *profName
	}
	p.Birthday = birthday
	if birthday != nil {
		p.BirthdayDisplay = birthday.Format("January 2, 2006")
	}
	p.ProfilePhotoURL = localMediaURL(photoPath)
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
			experience_years=$7, profession_id=$8, availability_status=$9, looking_for=$10,
			pitch_url=$11, personal_quote=$12, gender=$13, birthday=$14, updated_at=now()
		WHERE user_id=$1
	`, userID, in.FullName, in.Bio, in.About, in.Company, in.JobTitle, in.ExperienceYears,
		in.ProfessionID, in.AvailabilityStatus, in.LookingFor, in.PitchURL, in.PersonalQuote,
		in.Gender, in.Birthday)
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

func (r *PgRepository) ClearTags(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM user_tags WHERE user_id=$1`, userID)
	return err
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

func (r *PgRepository) ClearSkills(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM user_skills WHERE user_id=$1`, userID)
	return err
}

func (r *PgRepository) GetSocialLinks(ctx context.Context, userID uuid.UUID) ([]SocialLink, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT platform, url FROM social_links WHERE user_id=$1 ORDER BY display_order, platform
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []SocialLink
	for rows.Next() {
		var s SocialLink
		_ = rows.Scan(&s.Platform, &s.URL)
		list = append(list, s)
	}
	return list, nil
}

func (r *PgRepository) ReplaceSocialLinks(ctx context.Context, userID uuid.UUID, links []SocialLink) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM social_links WHERE user_id=$1`, userID)
	if err != nil {
		return err
	}
	for i, link := range links {
		if link.Platform == "" || link.URL == "" {
			continue
		}
		_, err = r.pool.Exec(ctx, `
			INSERT INTO social_links (user_id, platform, url, display_order) VALUES ($1,$2,$3,$4)
			ON CONFLICT (user_id, platform) DO UPDATE SET url=EXCLUDED.url, display_order=EXCLUDED.display_order
		`, userID, link.Platform, link.URL, i)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *PgRepository) GetStats(ctx context.Context, userID uuid.UUID) (Stats, error) {
	var s Stats
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM follows WHERE follower_id=$1),
			(SELECT COUNT(*) FROM follows WHERE following_id=$1),
			COALESCE((SELECT profile_views_count FROM profiles WHERE user_id=$1), 0),
			(SELECT COUNT(*) FROM connections WHERE status='accepted' AND (requester_id=$1 OR recipient_id=$1))
	`, userID).Scan(&s.Following, &s.Followers, &s.Views, &s.Interactions)
	return s, err
}

func (r *PgRepository) IncrementViews(ctx context.Context, profileUserID, viewerID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE profiles SET profile_views_count = profile_views_count + 1 WHERE user_id=$1
	`, profileUserID)
	if err != nil {
		return err
	}
	var viewer any
	if viewerID != uuid.Nil {
		viewer = viewerID
	}
	_, _ = r.pool.Exec(ctx, `
		INSERT INTO profile_views (profile_user_id, viewer_user_id) VALUES ($1, $2)
	`, profileUserID, viewer)
	return nil
}

func (r *PgRepository) ListFriends(ctx context.Context, userID uuid.UUID) ([]Friend, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.username, p.full_name
		FROM connections c
		JOIN users u ON u.id = CASE WHEN c.requester_id=$1 THEN c.recipient_id ELSE c.requester_id END
		JOIN profiles p ON p.user_id = u.id
		WHERE c.status='accepted' AND (c.requester_id=$1 OR c.recipient_id=$1)
		ORDER BY p.full_name
		LIMIT 50
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Friend
	for rows.Next() {
		var f Friend
		_ = rows.Scan(&f.UserID, &f.Username, &f.FullName)
		list = append(list, f)
	}
	return list, nil
}

func (r *PgRepository) GetAddress(ctx context.Context, userID uuid.UUID) (Address, error) {
	var a Address
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(city,''), COALESCE(region,''), COALESCE(country,''),
			COALESCE(ST_Y(geom::geometry),0), COALESCE(ST_X(geom::geometry),0)
		FROM user_locations WHERE user_id=$1 AND geom IS NOT NULL
	`, userID).Scan(&a.City, &a.Region, &a.Country, &a.Lat, &a.Lng)
	if err != nil {
		return a, nil
	}
	a.OK = true
	return a, nil
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

func (s *Service) ReplaceTags(ctx context.Context, userID uuid.UUID, names []string) error {
	if err := s.repo.ClearTags(ctx, userID); err != nil {
		return err
	}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if err := s.repo.AddTag(ctx, userID, name); err != nil {
			return err
		}
	}
	return nil
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

func (s *Service) ReplaceSkills(ctx context.Context, userID uuid.UUID, names []string) error {
	if err := s.repo.ClearSkills(ctx, userID); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		if err := s.repo.AddSkill(ctx, userID, name); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) GetSocialLinks(ctx context.Context, userID uuid.UUID) ([]SocialLink, error) {
	return s.repo.GetSocialLinks(ctx, userID)
}

func (s *Service) ReplaceSocialLinks(ctx context.Context, userID uuid.UUID, links []SocialLink) error {
	return s.repo.ReplaceSocialLinks(ctx, userID, links)
}

func (s *Service) GetStats(ctx context.Context, userID uuid.UUID) (Stats, error) {
	return s.repo.GetStats(ctx, userID)
}

func (s *Service) RecordView(ctx context.Context, profileUserID, viewerID uuid.UUID) error {
	if profileUserID == viewerID {
		return nil
	}
	return s.repo.IncrementViews(ctx, profileUserID, viewerID)
}

func (s *Service) ListFriends(ctx context.Context, userID uuid.UUID) ([]Friend, error) {
	return s.repo.ListFriends(ctx, userID)
}

func (s *Service) GetAddress(ctx context.Context, userID uuid.UUID) (Address, error) {
	return s.repo.GetAddress(ctx, userID)
}

func (s *Service) SetProfilePhoto(ctx context.Context, userID, fileID uuid.UUID) error {
	return s.repo.SetProfilePhoto(ctx, userID, fileID)
}

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// MaskEmail hides the local part for profile display (t***@gmail.com).
func MaskEmail(email string) string {
	at := strings.Index(email, "@")
	if at <= 0 {
		return email
	}
	local, domain := email[:at], email[at:]
	if len(local) == 1 {
		return local + "***" + domain
	}
	return string(local[0]) + "***" + domain
}

func localMediaURL(storagePath string) string {
	if storagePath == "" {
		return ""
	}
	path := strings.TrimPrefix(storagePath, "/")
	root := "data/uploads"
	if v := os.Getenv("UPLOAD_DIR"); v != "" {
		root = v
	}
	if _, err := os.Stat(filepath.Join(root, path)); err == nil {
		return "/api/v1/media/" + path
	}
	return ""
}
