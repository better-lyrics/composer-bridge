package library

import (
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("track not found")

type Track struct {
	ID           int64
	VideoID      string
	Title        string
	Artist       string
	Album        string
	ReleaseYear  int
	DurationSec  int
	ThumbnailURL string
	ThumbPath    string
	IsMusic      bool
	MusicType    string
	SourceURL    string
	ImportedAt   int64
	AudioPath    string
	AudioSize    int64
}

type Library struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS tracks (
  id            INTEGER PRIMARY KEY,
  video_id      TEXT UNIQUE NOT NULL,
  title         TEXT NOT NULL,
  artist        TEXT NOT NULL DEFAULT '',
  album         TEXT NOT NULL DEFAULT '',
  release_year  INTEGER NOT NULL DEFAULT 0,
  duration_sec  INTEGER NOT NULL,
  thumbnail_url TEXT NOT NULL,
  thumb_path    TEXT NOT NULL DEFAULT '',
  is_music      INTEGER NOT NULL DEFAULT 0,
  music_type    TEXT NOT NULL DEFAULT '',
  source_url    TEXT NOT NULL,
  imported_at   INTEGER NOT NULL,
  audio_path    TEXT NOT NULL DEFAULT '',
  audio_size    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_tracks_imported_at ON tracks(imported_at DESC);
`

const trackColumns = `id, video_id, title, artist, album, release_year, duration_sec, thumbnail_url, thumb_path, is_music, music_type, source_url, imported_at, audio_path, audio_size`

func Open(path string) (*Library, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Library{db: db}, nil
}

func (l *Library) Close() error { return l.db.Close() }

func (l *Library) InsertTrack(t *Track) error {
	const stmt = `
INSERT INTO tracks (video_id, title, artist, album, release_year, duration_sec, thumbnail_url, thumb_path, is_music, music_type, source_url, imported_at, audio_path, audio_size)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(video_id) DO UPDATE SET
  title=excluded.title, artist=excluded.artist, album=excluded.album,
  release_year=excluded.release_year, duration_sec=excluded.duration_sec,
  thumbnail_url=excluded.thumbnail_url, thumb_path=excluded.thumb_path,
  is_music=excluded.is_music, music_type=excluded.music_type,
  source_url=excluded.source_url, imported_at=excluded.imported_at,
  audio_path=excluded.audio_path, audio_size=excluded.audio_size
RETURNING id;
`
	return l.db.QueryRow(stmt,
		t.VideoID, t.Title, t.Artist, t.Album, t.ReleaseYear, t.DurationSec,
		t.ThumbnailURL, t.ThumbPath, t.IsMusic, t.MusicType, t.SourceURL,
		t.ImportedAt, t.AudioPath, t.AudioSize,
	).Scan(&t.ID)
}

func scanTrack(s interface {
	Scan(dest ...any) error
}, t *Track) error {
	return s.Scan(
		&t.ID, &t.VideoID, &t.Title, &t.Artist, &t.Album, &t.ReleaseYear, &t.DurationSec,
		&t.ThumbnailURL, &t.ThumbPath, &t.IsMusic, &t.MusicType, &t.SourceURL,
		&t.ImportedAt, &t.AudioPath, &t.AudioSize,
	)
}

func (l *Library) GetTrack(videoID string) (*Track, error) {
	q := `SELECT ` + trackColumns + ` FROM tracks WHERE video_id = ?`
	var t Track
	err := scanTrack(l.db.QueryRow(q, videoID), &t)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (l *Library) ListTracks() ([]Track, error) {
	q := `SELECT ` + trackColumns + ` FROM tracks ORDER BY imported_at DESC`
	rows, err := l.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Track
	for rows.Next() {
		var t Track
		if err := scanTrack(rows, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (l *Library) MarkAudioDownloaded(videoID, path string, size int64) error {
	res, err := l.db.Exec(`UPDATE tracks SET audio_path = ?, audio_size = ? WHERE video_id = ?`, path, size, videoID)
	if err != nil {
		return err
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

func (l *Library) RemoveTrack(videoID string) error {
	res, err := l.db.Exec(`DELETE FROM tracks WHERE video_id = ?`, videoID)
	if err != nil {
		return err
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
