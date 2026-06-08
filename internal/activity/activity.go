package activity

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when no activity row matches the given id.
var ErrNotFound = errors.New("activity not found")

// Kind identifies what kind of background work an activity row describes.
type Kind string

const (
	// KindAudioDownload covers yt-dlp audio fetches initiated by Composer.
	KindAudioDownload Kind = "audio_download"
	// KindImport covers metadata-only imports of a YouTube video into the library.
	KindImport Kind = "import"
	// KindYtdlpUpdate covers self-update runs of the bundled yt-dlp binary.
	KindYtdlpUpdate Kind = "ytdlp_update"
)

// Status is the lifecycle state of an activity row.
type Status string

const (
	// StatusRunning means the activity is in progress; EndedAt is 0.
	StatusRunning Status = "running"
	// StatusOK means the activity finished successfully.
	StatusOK Status = "ok"
	// StatusError means the activity finished with an error; Message holds the reason.
	StatusError Status = "error"
)

// Entry is one persisted activity row. StartedAt and EndedAt are Unix seconds; EndedAt is 0 while running.
type Entry struct {
	ID        int64
	Kind      Kind
	VideoID   string
	StartedAt int64
	EndedAt   int64
	Status    Status
	Message   string
}

// Log is a SQLite-backed append-only activity log with mutable end state. Safe for concurrent use.
type Log struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS activity (
  id         INTEGER PRIMARY KEY,
  kind       TEXT NOT NULL,
  video_id   TEXT NOT NULL DEFAULT '',
  started_at INTEGER NOT NULL,
  ended_at   INTEGER NOT NULL DEFAULT 0,
  status     TEXT NOT NULL,
  message    TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_activity_started_at ON activity(started_at DESC);
`

const entryColumns = `id, kind, video_id, started_at, ended_at, status, message`

// Open opens (and creates if missing) the SQLite-backed activity log at path. Schema application is idempotent.
func Open(path string) (*Log, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Log{db: db}, nil
}

// Close releases the underlying database handle.
func (l *Log) Close() error { return l.db.Close() }

// Start inserts a new row with status=running, ended_at=0, and the current Unix time as started_at. Returns the new row id.
func (l *Log) Start(kind Kind, videoID string) (int64, error) {
	const stmt = `INSERT INTO activity (kind, video_id, started_at, ended_at, status, message) VALUES (?, ?, ?, 0, ?, '') RETURNING id`
	var id int64
	if err := l.db.QueryRow(stmt, string(kind), videoID, time.Now().Unix(), string(StatusRunning)).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert activity: %w", err)
	}
	return id, nil
}

// End sets ended_at to the current Unix time and writes status and message for the row matching id. Returns ErrNotFound if no row matched.
func (l *Log) End(id int64, status Status, message string) error {
	const stmt = `UPDATE activity SET ended_at = ?, status = ?, message = ? WHERE id = ?`
	res, err := l.db.Exec(stmt, time.Now().Unix(), string(status), message, id)
	if err != nil {
		return fmt.Errorf("update activity: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Recent returns rows ordered by started_at DESC, limited to limit. A non-positive limit returns an empty slice without error.
func (l *Log) Recent(limit int) ([]Entry, error) {
	if limit <= 0 {
		return nil, nil
	}
	q := `SELECT ` + entryColumns + ` FROM activity ORDER BY started_at DESC LIMIT ?`
	rows, err := l.db.Query(q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		var kind, status string
		if err := rows.Scan(&e.ID, &kind, &e.VideoID, &e.StartedAt, &e.EndedAt, &status, &e.Message); err != nil {
			return nil, err
		}
		e.Kind = Kind(kind)
		e.Status = Status(status)
		out = append(out, e)
	}
	return out, rows.Err()
}
