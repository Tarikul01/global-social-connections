package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var allowedMIME = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/webp": true,
}

type Service struct {
	cfg  *config.Config
	pool *pgxpool.Pool
	log  *logger.Logger
}

func NewService(cfg *config.Config, pool *pgxpool.Pool, log *logger.Logger) *Service {
	return &Service{cfg: cfg, pool: pool, log: log}
}

func (s *Service) UploadProfilePhoto(ctx context.Context, userID uuid.UUID, fh *multipart.FileHeader) (uuid.UUID, error) {
	if fh.Size > 5*1024*1024 {
		return uuid.Nil, fmt.Errorf("file too large (max 5MB)")
	}
	file, err := fh.Open()
	if err != nil {
		return uuid.Nil, err
	}
	defer file.Close()

	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	mime := http.DetectContentType(buf[:n])
	if !allowedMIME[mime] {
		return uuid.Nil, fmt.Errorf("invalid file type")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return uuid.Nil, err
	}

	ext := extFromMIME(mime)
	fileID := uuid.New()
	storagePath := fmt.Sprintf("avatars/%s/%s%s", userID, fileID, ext)

	// Store metadata in DB; actual Supabase upload when credentials configured
	_, err = s.pool.Exec(ctx, `
		INSERT INTO files (id, user_id, bucket, storage_path, original_filename, mime_type, size_bytes, purpose)
		VALUES ($1,$2,'avatars',$3,$4,$5,$6,'profile_photo')
	`, fileID, userID, storagePath, filepath.Base(fh.Filename), mime, fh.Size)
	if err != nil {
		return uuid.Nil, err
	}

	if s.cfg.SupabaseURL != "" && s.cfg.SupabaseKey != "" {
		if err := s.uploadToSupabase(ctx, "avatars", storagePath, file, mime); err != nil {
			s.log.Error("supabase upload failed", "error", err)
		}
	}

	return fileID, nil
}

func (s *Service) uploadToSupabase(ctx context.Context, bucket, path string, r io.Reader, mime string) error {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", strings.TrimRight(s.cfg.SupabaseURL, "/"), bucket, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, r)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.SupabaseKey)
	req.Header.Set("Content-Type", mime)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("upload failed: %d", resp.StatusCode)
	}
	return nil
}

func extFromMIME(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ".bin"
	}
}
