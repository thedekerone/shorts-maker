package store

import (
	context "context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const defaultMaxRetries = 3

// JobRecord captures the persisted metadata for a generation job.
type JobRecord struct {
	ID              string
	Script          string
	VoiceID         string
	Mode            string
	VisualStyle     string
	CaptionStyle    string
	CaptionPosition string
	MusicID         string
	Webhook         string
	Status          string
	RetryCount      int
	MaxRetries      int
	GenerateKling   bool
	GenerateImages  bool
	KlingURL        string
	ImagesURL       string
	ThumbURL        string
	CurrentURL      string
	Error           string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Store wraps a SQLite database connection for persisting job state.
type Store struct {
	db *sql.DB
}

// New creates (or opens) the job store at the provided path.
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_busy_timeout=5000&_foreign_keys=on", path))
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		return nil, err
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		script TEXT NOT NULL,
		voice_id TEXT,
		mode TEXT,
		visual_style TEXT,
		caption_style TEXT,
		caption_position TEXT,
		music_id TEXT,
		webhook TEXT,
		status TEXT NOT NULL,
		retry_count INTEGER NOT NULL DEFAULT 0,
		max_retries INTEGER NOT NULL DEFAULT 3,
		generate_kling INTEGER NOT NULL DEFAULT 0,
		generate_images INTEGER NOT NULL DEFAULT 0,
		kling_url TEXT,
		images_url TEXT,
		thumb_url TEXT,
		current_url TEXT,
		error TEXT,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
	`)
	if err != nil {
		return err
	}

	columns := map[string]string{
		"generate_kling":  "INTEGER NOT NULL DEFAULT 0",
		"generate_images": "INTEGER NOT NULL DEFAULT 0",
		"thumb_url":       "TEXT",
		"current_url":     "TEXT",
	}
	for col, def := range columns {
		if err := s.addColumn(col, def); err != nil {
			return err
		}
	}

	return nil
}

// Close releases the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateJob inserts a new job record.
func (s *Store) CreateJob(ctx context.Context, job JobRecord) error {
	if job.MaxRetries == 0 {
		job.MaxRetries = defaultMaxRetries
	}
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
	INSERT INTO jobs (id, script, voice_id, mode, visual_style, caption_style, caption_position, music_id, webhook, status, retry_count, max_retries, generate_kling, generate_images, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?)
	`, job.ID, job.Script, job.VoiceID, job.Mode, job.VisualStyle, job.CaptionStyle, job.CaptionPosition, job.MusicID, job.Webhook, job.Status, job.MaxRetries, boolToInt(job.GenerateKling), boolToInt(job.GenerateImages), now, now)
	return err
}

// UpdateStatus updates the job status, url, and error message.
func (s *Store) UpdateStatus(ctx context.Context, id, status, url, errMsg string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
	UPDATE jobs SET status=?, current_url=?, error=?, updated_at=? WHERE id=?
	`, status, nullIfEmpty(url), errMsg, now, id)
	return err
}

// UpdateVideoURL stores the signed URLs for generated videos.
func (s *Store) UpdateVideoURL(ctx context.Context, id, variant, url string) error {
	if url == "" {
		return nil
	}

	column := ""
	switch variant {
	case "kling_video":
		column = "kling_url"
	case "image_video":
		column = "images_url"
	case "thumb":
		column = "thumb_url"
	default:
		column = "current_url"
	}

	query := fmt.Sprintf("UPDATE jobs SET %s = ?, updated_at = ? WHERE id = ?", column)
	_, err := s.db.ExecContext(ctx, query, url, time.Now().UTC(), id)
	if err != nil {
		return err
	}

	switch variant {
	case "kling_video":
		_, err = s.db.ExecContext(ctx, `UPDATE jobs SET current_url = ? WHERE id = ?`, url, id)
	case "image_video":
		_, err = s.db.ExecContext(ctx, `UPDATE jobs SET current_url = CASE WHEN current_url = '' THEN ? ELSE current_url END WHERE id = ?`, url, id)
	}
	return err
}

// GetJob fetches a job row by ID.
func (s *Store) GetJob(ctx context.Context, id string) (JobRecord, error) {
	row := s.db.QueryRowContext(ctx, `
	SELECT id, script, voice_id, mode, visual_style, caption_style, caption_position, music_id, webhook, status, retry_count, max_retries,
		generate_kling, generate_images,
		COALESCE(kling_url, ''), COALESCE(images_url, ''), COALESCE(thumb_url, ''), COALESCE(current_url, ''), COALESCE(error, ''), created_at, updated_at
	FROM jobs WHERE id=?
	`, id)

	var job JobRecord
	var genK, genI int
	err := row.Scan(&job.ID, &job.Script, &job.VoiceID, &job.Mode, &job.VisualStyle, &job.CaptionStyle, &job.CaptionPosition, &job.MusicID, &job.Webhook, &job.Status, &job.RetryCount, &job.MaxRetries, &genK, &genI, &job.KlingURL, &job.ImagesURL, &job.ThumbURL, &job.CurrentURL, &job.Error, &job.CreatedAt, &job.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return JobRecord{}, fmt.Errorf("job %s not found", id)
	}
	job.GenerateKling = genK == 1
	job.GenerateImages = genI == 1
	return job, err
}

// IncrementRetry increments retry_count and stores the latest error.
func (s *Store) IncrementRetry(ctx context.Context, id string, errMsg string) (JobRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return JobRecord{}, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `UPDATE jobs SET retry_count = retry_count + 1, status = ?, error = ?, updated_at = ? WHERE id = ?`, "retrying", errMsg, now, id)
	if err != nil {
		return JobRecord{}, err
	}

	row := tx.QueryRowContext(ctx, `SELECT retry_count, max_retries FROM jobs WHERE id = ?`, id)
	var retryCount, maxRetries int
	if err := row.Scan(&retryCount, &maxRetries); err != nil {
		return JobRecord{}, err
	}

	if err := tx.Commit(); err != nil {
		return JobRecord{}, err
	}

	job, err := s.GetJob(ctx, id)
	if err != nil {
		return JobRecord{}, err
	}
	job.RetryCount = retryCount
	job.MaxRetries = maxRetries
	return job, nil
}

func (s *Store) MarkForRetry(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE jobs SET status = ?, error = '', retry_count = 0, updated_at = ? WHERE id = ?`, "queued", time.Now().UTC(), id)
	return err
}

// UpdateWebhook sets webhook URL (used when retrying manually).
func (s *Store) UpdateWebhook(ctx context.Context, id, webhook string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE jobs SET webhook=?, updated_at=? WHERE id=?`, webhook, time.Now().UTC(), id)
	return err
}

func nullIfEmpty(val string) any {
	if val == "" {
		return nil
	}
	return val
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) addColumn(name, def string) error {
	stmt := fmt.Sprintf("ALTER TABLE jobs ADD COLUMN %s %s", name, def)
	if _, err := s.db.Exec(stmt); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return nil
		}
		return err
	}
	return nil
}
