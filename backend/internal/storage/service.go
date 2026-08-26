package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/gin-gonic/gin"
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

func (s *Service) UploadRoot() string {
	if v := os.Getenv("UPLOAD_DIR"); v != "" {
		return v
	}
	return "data/uploads"
}

func (s *Service) PublicURL(storagePath string) string {
	if storagePath == "" {
		return ""
	}
	path := strings.TrimPrefix(storagePath, "/")
	local := filepath.Join(s.UploadRoot(), path)
	if _, err := os.Stat(local); err == nil {
		return "/api/v1/media/" + path
	}
	return ""
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

	dest := filepath.Join(s.UploadRoot(), storagePath)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return uuid.Nil, err
	}
	out, err := os.Create(dest)
	if err != nil {
		return uuid.Nil, err
	}
	written, err := io.Copy(out, file)
	_ = out.Close()
	if err != nil {
		_ = os.Remove(dest)
		return uuid.Nil, err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO files (id, user_id, bucket, storage_path, original_filename, mime_type, size_bytes, purpose)
		VALUES ($1,$2,'avatars',$3,$4,$5,$6,'profile_photo')
	`, fileID, userID, storagePath, filepath.Base(fh.Filename), mime, written)
	if err != nil {
		_ = os.Remove(dest)
		return uuid.Nil, err
	}

	if s.cfg.SupabaseURL != "" && s.cfg.SupabaseKey != "" && !strings.Contains(s.cfg.SupabaseKey, "paste-your") {
		f2, err := os.Open(dest)
		if err == nil {
			defer f2.Close()
			if err := s.uploadToSupabase(ctx, "avatars", storagePath, f2, mime); err != nil {
				s.log.Error("supabase upload failed", "error", err)
			}
		}
	}

	return fileID, nil
}

// ServeMedia serves GET /api/v1/media/* from local upload storage.
func (s *Service) ServeMedia(c *gin.Context) {
	rel := strings.TrimPrefix(c.Param("filepath"), "/")
	if rel == "" || strings.Contains(rel, "..") {
		c.Status(http.StatusNotFound)
		return
	}
	full := filepath.Join(s.UploadRoot(), filepath.Clean(rel))
	root, err := filepath.Abs(s.UploadRoot())
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	abs, err := filepath.Abs(full)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	sep := string(os.PathSeparator)
	if abs != root && !strings.HasPrefix(abs, root+sep) {
		c.Status(http.StatusNotFound)
		return
	}
	if st, err := os.Stat(abs); err != nil || st.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}
	c.File(abs)
}

func (s *Service) uploadToSupabase(ctx context.Context, bucket, path string, r io.Reader, mime string) error {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", strings.TrimRight(s.cfg.SupabaseURL, "/"), bucket, path)
	req, err := http.NewRequestWithContext(ctx, "POST", url, r)
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
