package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/boidushya/composer-bridge/internal/activity"
	"github.com/boidushya/composer-bridge/internal/events"
	"github.com/boidushya/composer-bridge/internal/library"
	"github.com/boidushya/composer-bridge/internal/ytdlp"
)

const (
	thumbnailArtSize  = 1024
	thumbnailMaxAge   = "public, max-age=86400"
	thumbnailFetchTTL = 30 * time.Second
)

func audioContentType(format string) string {
	switch format {
	case "m4a":
		return "audio/mp4"
	case "mp3":
		return "audio/mpeg"
	default:
		// opus, webm, anything unknown.
		return "audio/webm"
	}
}

// Handlers wires the bridge HTTP API to the library, activity log, and yt-dlp.
// Library, Activity, YtdlpPath, ThumbDir, and Bridge are required. Emitter and
// EmitterCtx are optional: when both are set, every successful activity transition
// (start, ok, error) publishes an activity:update event so the Wails frontend can
// keep its live feed in sync without polling. A nil Emitter leaves handlers
// emitting nothing, which is what tests want by default.
type Handlers struct {
	Library     *library.Library
	Activity    *activity.Log
	YtdlpPath   string
	ThumbDir    string
	Bridge      string
	AudioFormat string
	Emitter     events.Emitter
	EmitterCtx  context.Context
}

// Router returns the bridge's HTTP mux. Wrap with WithCORS at the call site for browser access.
func (h *Handlers) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /audio/{id}", h.Audio)
	mux.HandleFunc("GET /thumb/{id}", h.Thumb)
	mux.HandleFunc("POST /import", h.Import)
	for _, p := range []string{"OPTIONS /audio/{id}", "OPTIONS /thumb/{id}", "OPTIONS /import", "OPTIONS /health"} {
		mux.HandleFunc(p, h.preflight)
	}
	return mux
}

// Health returns bridge version, yt-dlp version, and a literal "ok" status. Field names are locked to Composer's BridgeHealth interface.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"bridge": h.Bridge,
		"ytdlp":  ytdlp.Version(h.YtdlpPath),
		"status": "ok",
	})
}

// Audio streams the bestaudio track for videoID. Wraps the call in an activity row so the Activity feed shows "downloading X" in real time. Returns 502 JSON if yt-dlp fails before any bytes flow, otherwise the connection is closed mid-stream. As a side effect, the videoID is upserted into the library in the background so the bridge UI can display the track even though Composer never explicitly imports it.
func (h *Handlers) Audio(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")
	if !ytdlp.VideoIDRe.MatchString(videoID) {
		writeError(w, http.StatusBadRequest, "invalid video id")
		return
	}
	format := h.AudioFormat
	if format == "" {
		format = "opus"
	}
	go h.ensureLibraryEntry(videoID)
	actID := h.startActivity(activity.KindAudioDownload, videoID)
	w.Header().Set("Content-Type", audioContentType(format))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Bridge-Version", h.Bridge)
	tw := &trackingWriter{rw: w}
	err := ytdlp.StreamAudio(r.Context(), h.YtdlpPath, videoID, format, tw)
	if err == nil {
		h.endActivity(actID, activity.StatusOK, "")
		return
	}
	h.endActivity(actID, activity.StatusError, fmt.Sprintf("%s: %v", videoID, err))
	if tw.wrote {
		slog.Warn("audio stream failed mid-flight", "videoID", videoID, "err", err)
		return
	}
	writeError(w, http.StatusBadGateway, fmt.Sprintf("yt-dlp failed for %s", videoID))
}

// ensureLibraryEntry fetches metadata for videoID and inserts/updates the
// library row in the background. Skipped when a row already exists. Failures
// are logged and never break the caller's request. Uses a detached context so
// the request closing doesn't kill the metadata fetch mid-flight.
func (h *Handlers) ensureLibraryEntry(videoID string) {
	if _, err := h.Library.GetTrack(videoID); err == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	info, err := ytdlp.FetchInfo(ctx, h.YtdlpPath, videoID)
	if err != nil {
		slog.Warn("audio: background info fetch failed", "videoID", videoID, "err", err)
		return
	}
	track := trackFromInfo(info)
	if err := h.Library.InsertTrack(&track); err != nil {
		slog.Warn("audio: background library insert failed", "videoID", videoID, "err", err)
	}
}

// Import fetches metadata for the body's video_id, inserts a track row, and returns the inserted record. Wrapped in an activity row.
func (h *Handlers) Import(w http.ResponseWriter, r *http.Request) {
	var body struct {
		VideoID string `json:"video_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if !ytdlp.VideoIDRe.MatchString(body.VideoID) {
		writeError(w, http.StatusBadRequest, "invalid video id")
		return
	}
	actID := h.startActivity(activity.KindImport, body.VideoID)
	info, err := ytdlp.FetchInfo(r.Context(), h.YtdlpPath, body.VideoID)
	if err != nil {
		h.endActivity(actID, activity.StatusError, fmt.Sprintf("%s: %v", body.VideoID, err))
		writeError(w, http.StatusBadGateway, fmt.Sprintf("yt-dlp info failed for %s", body.VideoID))
		return
	}
	track := trackFromInfo(info)
	if err := h.Library.InsertTrack(&track); err != nil {
		h.endActivity(actID, activity.StatusError, fmt.Sprintf("insert %s: %v", body.VideoID, err))
		writeError(w, http.StatusInternalServerError, "library insert failed")
		return
	}
	h.endActivity(actID, activity.StatusOK, "")
	writeJSON(w, http.StatusOK, track)
}

// Thumb serves the cached album art for videoID, lazily fetching and caching from the track's ThumbnailURL on a cache miss.
func (h *Handlers) Thumb(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")
	if !ytdlp.VideoIDRe.MatchString(videoID) {
		writeError(w, http.StatusBadRequest, "invalid video id")
		return
	}
	track, err := h.Library.GetTrack(videoID)
	if errors.Is(err, library.ErrNotFound) {
		writeError(w, http.StatusNotFound, "track not found")
		return
	}
	if err != nil {
		slog.Error("thumb lookup failed", "videoID", videoID, "err", err)
		writeError(w, http.StatusInternalServerError, "library read failed")
		return
	}
	w.Header().Set("Cache-Control", thumbnailMaxAge)
	if track.ThumbPath != "" {
		if _, err := os.Stat(track.ThumbPath); err == nil {
			http.ServeFile(w, r, track.ThumbPath)
			return
		}
	}
	path, err := h.fetchAndCacheThumb(r.Context(), track)
	if err != nil {
		slog.Warn("thumb fetch failed", "videoID", videoID, "err", err)
		writeError(w, http.StatusBadGateway, "thumb fetch failed")
		return
	}
	http.ServeFile(w, r, path)
}

func (h *Handlers) preflight(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) fetchAndCacheThumb(ctx context.Context, track *library.Track) (string, error) {
	if track.ThumbnailURL == "" {
		return "", errors.New("track has no thumbnail url")
	}
	if err := os.MkdirAll(h.ThumbDir, 0o755); err != nil {
		return "", fmt.Errorf("create thumb dir: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, track.ThumbnailURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := (&http.Client{Timeout: thumbnailFetchTTL}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("thumb http %d", resp.StatusCode)
	}
	dest := filepath.Join(h.ThumbDir, track.VideoID+".jpg")
	tmp, err := os.CreateTemp(h.ThumbDir, track.VideoID+".*.tmp")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, copyErr := io.Copy(tmp, resp.Body); copyErr != nil {
		tmp.Close()
		return "", copyErr
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return "", err
	}
	if err := h.Library.SetThumbPath(track.VideoID, dest); err != nil {
		slog.Warn("set thumb path failed", "videoID", track.VideoID, "err", err)
	}
	return dest, nil
}

func (h *Handlers) startActivity(kind activity.Kind, videoID string) int64 {
	id, err := h.Activity.Start(kind, videoID)
	if err != nil {
		slog.Warn("activity start failed", "kind", kind, "videoID", videoID, "err", err)
		return 0
	}
	h.emitActivity(id)
	return id
}

func (h *Handlers) endActivity(id int64, st activity.Status, msg string) {
	if id == 0 {
		return
	}
	if err := h.Activity.End(id, st, msg); err != nil {
		slog.Warn("activity end failed", "id", id, "status", st, "err", err)
		return
	}
	h.emitActivity(id)
}

// emitActivity publishes the latest snapshot of the activity row to the
// frontend. Fails silently when no Emitter is wired (tests, headless boot).
// Looks the row up by id rather than constructing it inline so a single source
// of truth (the SQLite row) drives both the Activity view's initial fetch and
// its live update stream: any drift would show up as a UI inconsistency.
func (h *Handlers) emitActivity(id int64) {
	if h.Emitter == nil || id == 0 {
		return
	}
	entries, err := h.Activity.Recent(50)
	if err != nil {
		slog.Warn("emit activity recent failed", "id", id, "err", err)
		return
	}
	for _, e := range entries {
		if e.ID == id {
			h.Emitter.Emit(h.EmitterCtx, "activity:update", e)
			return
		}
	}
}

func trackFromInfo(info *ytdlp.Info) library.Track {
	thumb, isMusic, _ := ytdlp.NormalizeArt(info, thumbnailArtSize)
	musicType, title, source := "", info.Title, info.WebpageURL
	if isMusic {
		musicType = "song"
	}
	if info.Track != "" {
		title = info.Track
	}
	if source == "" {
		source = "https://www.youtube.com/watch?v=" + info.ID
	}
	return library.Track{
		VideoID: info.ID, Title: title, Artist: info.Artist, Album: info.Album,
		ReleaseYear: info.ReleaseYear, DurationSec: info.Duration,
		ThumbnailURL: thumb, IsMusic: isMusic, MusicType: musicType,
		SourceURL: source, ImportedAt: time.Now().UnixMilli(),
	}
}

type trackingWriter struct {
	rw    http.ResponseWriter
	wrote bool
}

func (t *trackingWriter) Write(p []byte) (int, error) {
	if len(p) > 0 {
		t.wrote = true
	}
	return t.rw.Write(p)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Warn("write json", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
