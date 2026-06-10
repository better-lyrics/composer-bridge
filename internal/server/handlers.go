package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/better-lyrics/composer-bridge/internal/activity"
	"github.com/better-lyrics/composer-bridge/internal/bridgestate"
	"github.com/better-lyrics/composer-bridge/internal/events"
	"github.com/better-lyrics/composer-bridge/internal/library"
	"github.com/better-lyrics/composer-bridge/internal/ytdlp"
)

const (
	thumbnailArtSize  = 1024
	thumbnailMaxAge   = "public, max-age=86400"
	thumbnailFetchTTL = 30 * time.Second
	audioStreamTTL    = 10 * time.Minute
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
//
// State is optional: when set, audio downloads flip the holder so the frontend
// and tray can render a live status ("downloading X"). A nil State leaves the
// handler behavior unchanged, which is what existing handler tests rely on.
//
// YtdlpVersion is a callback rather than a stored string so /health always reads
// the latest cached value even when yt-dlp is refreshed in the background.
type Handlers struct {
	Library      *library.Library
	Activity     *activity.Log
	YtdlpPath    string
	YtdlpVersion func() string
	// CookiesPath returns the absolute path of the user-uploaded cookies file
	// when the user has enabled the feature AND the file exists; otherwise "".
	// Read on every request so toggles take effect immediately. A nil callback
	// is treated as "" (no cookies, anonymous downloads).
	CookiesPath func() string
	// PreferPremiumAudio reports whether yt-dlp should try YouTube Music's
	// higher quality tier first. Read on every request so toggling takes
	// effect immediately. A nil callback is treated as false. Gated by the
	// App so the flag only takes effect when cookies are also live.
	PreferPremiumAudio func() bool
	// DownloadDir returns the absolute path of the user-configured audio
	// download root. Read on every request so a config change takes effect
	// immediately. A nil callback (or empty return) disables the cache-first
	// audio path, leaving every /audio/{id} request to stream via yt-dlp.
	// The Audio handler only serves cached files whose track.AudioPath
	// resolves to a location under this root, mirroring the ThumbDir guard
	// used by Thumb.
	DownloadDir func() string
	ThumbDir    string
	Bridge      string
	AudioFormat string
	Emitter     events.Emitter
	EmitterCtx  context.Context
	State       *bridgestate.Holder
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

// cookiesPath returns the live cookies file path via the callback, or "" when
// no callback is wired. Centralized so every yt-dlp call site stays consistent.
func (h *Handlers) cookiesPath() string {
	if h.CookiesPath == nil {
		return ""
	}
	return h.CookiesPath()
}

// preferPremium returns the live premium-audio gate via the callback, or
// false when no callback is wired. Centralized so every yt-dlp call site
// stays consistent.
func (h *Handlers) preferPremium() bool {
	if h.PreferPremiumAudio == nil {
		return false
	}
	return h.PreferPremiumAudio()
}

// downloadDir returns the live audio download root via the callback, or ""
// when no callback is wired. Centralized so the cache-first guard reads the
// same value on every request.
func (h *Handlers) downloadDir() string {
	if h.DownloadDir == nil {
		return ""
	}
	return h.DownloadDir()
}

// Health returns bridge version, yt-dlp version, and a literal "ok" status. Field names are locked to Composer's BridgeHealth interface.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	var ver string
	if h.YtdlpVersion != nil {
		ver = h.YtdlpVersion()
	} else {
		ver = ytdlp.Version(h.YtdlpPath)
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"bridge": h.Bridge,
		"ytdlp":  ver,
		"status": "ok",
	})
}

// Audio serves the bestaudio track for videoID. When a previous /download has
// landed the file under the configured DownloadDir, the handler serves the
// cached copy via http.ServeFile (which handles Range requests natively, so
// seeking works); otherwise it falls through to a live yt-dlp stream wrapped
// in an activity row. Returns 502 JSON if the stream fails before any bytes
// flow, otherwise the connection is closed mid-stream. Track metadata is
// fetched up front so the X-Track-* headers are populated identically on both
// branches, and so a brand-new video gets persisted to the library either way.
func (h *Handlers) Audio(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")
	if !ytdlp.VideoIDRe.MatchString(videoID) {
		writeError(w, http.StatusBadRequest, "invalid video id")
		return
	}
	track := h.resolveTrackForAudio(r.Context(), videoID)
	if h.serveCachedAudio(w, r, track) {
		return
	}
	format := h.AudioFormat
	if format == "" {
		format = "opus"
	}
	actID := h.startActivity(activity.KindAudioDownload, videoID)
	if h.State != nil {
		h.State.StartDownload(videoID)
	}
	h.writeAudioHeaders(w, audioContentType(format), track)
	tw := &trackingWriter{rw: w}
	streamCtx, cancel := context.WithTimeout(r.Context(), audioStreamTTL)
	defer cancel()
	err := ytdlp.StreamAudio(streamCtx, h.YtdlpPath, videoID, format, h.cookiesPath(), h.preferPremium(), tw)
	if err == nil {
		h.endActivity(actID, activity.StatusOK, "")
		if h.State != nil {
			h.State.EndDownload("")
		}
		return
	}
	h.endActivity(actID, activity.StatusError, fmt.Sprintf("%s: %v", videoID, err))
	if h.State != nil {
		h.State.EndDownload(fmt.Sprintf("%s: %v", videoID, err))
	}
	if tw.wrote {
		slog.Warn("audio stream failed mid-flight", "videoID", videoID, "err", err)
		return
	}
	writeError(w, http.StatusBadGateway, fmt.Sprintf("yt-dlp failed for %s", videoID))
}

// writeAudioHeaders writes the headers shared between the cache-hit and
// streaming branches of Audio. contentType is the negotiated MIME type: for
// cache hits it is derived from the on-disk extension, for streams from the
// configured AudioFormat. track is optional; when set the X-Track-* metadata
// headers (and the matching CORS expose list) are emitted so Composer can
// populate the project title without a second round trip.
func (h *Handlers) writeAudioHeaders(w http.ResponseWriter, contentType string, track *library.Track) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Bridge-Version", h.Bridge)
	if track == nil {
		return
	}
	// HTTP headers are Latin-1 by spec; raw UTF-8 gets mojibake'd in the browser.
	// Percent-encode so the client can decodeURIComponent it back to the original string.
	w.Header().Set("Access-Control-Expose-Headers", "X-Track-Title, X-Track-Artist, X-Track-Album, X-Bridge-Version")
	if track.Title != "" {
		w.Header().Set("X-Track-Title", url.PathEscape(track.Title))
	}
	if track.Artist != "" {
		w.Header().Set("X-Track-Artist", url.PathEscape(track.Artist))
	}
	if track.Album != "" {
		w.Header().Set("X-Track-Album", url.PathEscape(track.Album))
	}
}

// serveCachedAudio attempts to serve the audio file recorded on track from
// disk. Returns true when the response has been written (cache hit); false to
// signal that the caller should fall through to the streaming path. Mirrors
// the Thumb cache guard: requires the file to be (a) recorded on the library
// row, (b) located under the configured DownloadDir so a stale DB row can't
// point the handler at an arbitrary filesystem path, (c) present on disk.
// Content-Type is inferred from the on-disk extension rather than h.AudioFormat
// because the user may have downloaded in one format then switched the setting.
// Cache hits are intentionally silent: no activity row, no State flip, so the
// tray's "downloading X" indicator stays accurate and the Activity feed only
// shows actual yt-dlp work.
func (h *Handlers) serveCachedAudio(w http.ResponseWriter, r *http.Request, track *library.Track) bool {
	if track == nil || track.AudioPath == "" {
		return false
	}
	root := h.downloadDir()
	if root == "" {
		return false
	}
	if !pathIsUnder(track.AudioPath, root) {
		return false
	}
	if _, err := os.Stat(track.AudioPath); err != nil {
		return false
	}
	ext := strings.TrimPrefix(filepath.Ext(track.AudioPath), ".")
	h.writeAudioHeaders(w, audioContentType(ext), track)
	http.ServeFile(w, r, track.AudioPath)
	return true
}

// resolveTrackForAudio returns the library entry for videoID, fetching and
// inserting metadata via yt-dlp on a cache miss. Best-effort: returns nil on
// any failure so the audio request can still proceed with no title headers.
func (h *Handlers) resolveTrackForAudio(ctx context.Context, videoID string) *library.Track {
	if existing, err := h.Library.GetTrack(videoID); err == nil && existing != nil {
		return existing
	}
	infoCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	info, err := ytdlp.FetchInfo(infoCtx, h.YtdlpPath, videoID, h.cookiesPath(), h.preferPremium())
	if err != nil {
		slog.Warn("audio: info fetch failed", "videoID", videoID, "err", err)
		return nil
	}
	track := trackFromInfo(info)
	if err := h.Library.InsertTrack(&track); err != nil {
		slog.Warn("audio: library insert failed", "videoID", videoID, "err", err)
		return &track
	}
	h.emitLibraryUpdate(track.VideoID)
	return &track
}

// emitLibraryUpdate pushes a library:update event to the frontend so any open
// library view can refresh without polling. Silent no-op when no Emitter is
// wired (e.g., in unit tests).
func (h *Handlers) emitLibraryUpdate(videoID string) {
	if h.Emitter == nil || h.EmitterCtx == nil {
		return
	}
	h.Emitter.Emit(h.EmitterCtx, "library:update", map[string]string{"video_id": videoID})
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
	info, err := ytdlp.FetchInfo(r.Context(), h.YtdlpPath, body.VideoID, h.cookiesPath(), h.preferPremium())
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
	h.emitLibraryUpdate(track.VideoID)
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
	if track.ThumbPath != "" && pathIsUnder(track.ThumbPath, h.ThumbDir) {
		if _, err := os.Stat(track.ThumbPath); err == nil {
			w.Header().Set("Cache-Control", thumbnailMaxAge)
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
	w.Header().Set("Cache-Control", thumbnailMaxAge)
	http.ServeFile(w, r, path)
}

// pathIsUnder reports whether path resolves to a location inside root. Both are
// cleaned via filepath.Abs before comparison; empty root rejects everything.
func pathIsUnder(path, root string) bool {
	if root == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
