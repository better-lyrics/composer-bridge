package library

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

func openTempLibrary(t *testing.T) (*Library, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "library.db")
	lib, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { lib.Close() })
	return lib, path
}

func sampleTrack(videoID string, importedAt int64) Track {
	return Track{
		VideoID:      videoID,
		Title:        "Fly Me To The Moon",
		Artist:       "Frank Sinatra",
		Album:        "Nothing But The Best",
		ReleaseYear:  2008,
		DurationSec:  147,
		ThumbnailURL: "https://yt3.googleusercontent.com/x.jpg",
		ThumbPath:    "thumbs/" + videoID + ".jpg",
		IsMusic:      true,
		MusicType:    "song",
		SourceURL:    "https://www.youtube.com/watch?v=" + videoID,
		ImportedAt:   importedAt,
		AudioPath:    "",
		AudioSize:    0,
	}
}

func TestOpen_CreatesSchemaAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.db")

	lib, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	track := sampleTrack("dQw4w9WgXcQ", 1000)
	if err := lib.InsertTrack(&track); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	lib2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer lib2.Close()
	got, err := lib2.GetTrack("dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("GetTrack after reopen: %v", err)
	}
	if got.Title != track.Title {
		t.Errorf("Title after reopen: got %q, want %q", got.Title, track.Title)
	}
}

func TestInsertTrack_RoundTripsAllFields(t *testing.T) {
	lib, _ := openTempLibrary(t)

	in := Track{
		VideoID:      "ZEcqHA7dbwM",
		Title:        "Fly Me To The Moon (2008 Remastered)",
		Artist:       "Frank Sinatra, Count Basie And His Orchestra",
		Album:        "Nothing But The Best",
		ReleaseYear:  2008,
		DurationSec:  147,
		ThumbnailURL: "https://yt3.googleusercontent.com/cover.jpg",
		ThumbPath:    "thumbs/ZEcqHA7dbwM.jpg",
		IsMusic:      true,
		MusicType:    "song",
		SourceURL:    "https://music.youtube.com/watch?v=ZEcqHA7dbwM",
		ImportedAt:   1717891234,
		AudioPath:    "audio/ZEcqHA7dbwM.opus",
		AudioSize:    4567890,
	}
	if err := lib.InsertTrack(&in); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}
	if in.ID == 0 {
		t.Fatal("InsertTrack: ID was not populated via RETURNING id")
	}

	got, err := lib.GetTrack(in.VideoID)
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if *got != in {
		t.Errorf("round trip mismatch:\n got  %+v\n want %+v", *got, in)
	}
}

func TestInsertTrack_IsMusicFalseRoundTrip(t *testing.T) {
	lib, _ := openTempLibrary(t)

	tr := sampleTrack("jNQXAC9IVRw", 2000)
	tr.IsMusic = false
	tr.MusicType = ""
	if err := lib.InsertTrack(&tr); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	got, err := lib.GetTrack("jNQXAC9IVRw")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if got.IsMusic != false {
		t.Errorf("IsMusic: got %v, want false", got.IsMusic)
	}
}

func TestListTracks_OrderedByImportedAtDesc(t *testing.T) {
	lib, _ := openTempLibrary(t)

	oldest := sampleTrack("aaaaaaaaaaa", 1000)
	middle := sampleTrack("bbbbbbbbbbb", 2000)
	newest := sampleTrack("ccccccccccc", 3000)

	for _, tr := range []Track{oldest, newest, middle} {
		track := tr
		if err := lib.InsertTrack(&track); err != nil {
			t.Fatalf("InsertTrack %s: %v", tr.VideoID, err)
		}
	}

	tracks, err := lib.ListTracks()
	if err != nil {
		t.Fatalf("ListTracks: %v", err)
	}
	if len(tracks) != 3 {
		t.Fatalf("ListTracks: got %d rows, want 3", len(tracks))
	}
	wantOrder := []string{"ccccccccccc", "bbbbbbbbbbb", "aaaaaaaaaaa"}
	for i, want := range wantOrder {
		if tracks[i].VideoID != want {
			t.Errorf("ListTracks[%d]: got %q, want %q", i, tracks[i].VideoID, want)
		}
	}
}

func TestListTracks_EmptyReturnsNoError(t *testing.T) {
	lib, _ := openTempLibrary(t)

	tracks, err := lib.ListTracks()
	if err != nil {
		t.Fatalf("ListTracks: %v", err)
	}
	if len(tracks) != 0 {
		t.Errorf("ListTracks on empty db: got %d rows, want 0", len(tracks))
	}
}

func TestInsertTrack_DuplicateVideoIDUpserts(t *testing.T) {
	lib, _ := openTempLibrary(t)

	first := sampleTrack("dQw4w9WgXcQ", 1000)
	if err := lib.InsertTrack(&first); err != nil {
		t.Fatalf("first InsertTrack: %v", err)
	}
	firstID := first.ID

	second := sampleTrack("dQw4w9WgXcQ", 2000)
	second.Title = "Updated Title"
	second.Artist = "Updated Artist"
	second.DurationSec = 999
	if err := lib.InsertTrack(&second); err != nil {
		t.Fatalf("upsert InsertTrack: %v", err)
	}

	if second.ID != firstID {
		t.Errorf("upsert ID: got %d, want %d (row id must stay stable)", second.ID, firstID)
	}

	var count int
	if err := lib.db.QueryRow(`SELECT COUNT(*) FROM tracks`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("row count after upsert: got %d, want 1", count)
	}

	got, err := lib.GetTrack("dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if got.Title != "Updated Title" {
		t.Errorf("Title: got %q, want Updated Title", got.Title)
	}
	if got.Artist != "Updated Artist" {
		t.Errorf("Artist: got %q, want Updated Artist", got.Artist)
	}
	if got.DurationSec != 999 {
		t.Errorf("DurationSec: got %d, want 999", got.DurationSec)
	}
}

func TestGetTrack_NotFound(t *testing.T) {
	lib, _ := openTempLibrary(t)

	_, err := lib.GetTrack("does_not_exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTrack(missing): got %v, want ErrNotFound", err)
	}
}

func TestMarkAudioDownloaded_HappyPath(t *testing.T) {
	lib, _ := openTempLibrary(t)

	tr := sampleTrack("dQw4w9WgXcQ", 1000)
	if err := lib.InsertTrack(&tr); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	if err := lib.MarkAudioDownloaded("dQw4w9WgXcQ", "audio/dQw4w9WgXcQ.opus", 1234567); err != nil {
		t.Fatalf("MarkAudioDownloaded: %v", err)
	}

	got, err := lib.GetTrack("dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if got.AudioPath != "audio/dQw4w9WgXcQ.opus" {
		t.Errorf("AudioPath: got %q", got.AudioPath)
	}
	if got.AudioSize != 1234567 {
		t.Errorf("AudioSize: got %d, want 1234567", got.AudioSize)
	}
}

func TestMarkAudioDownloaded_NotFound(t *testing.T) {
	lib, _ := openTempLibrary(t)

	err := lib.MarkAudioDownloaded("missing", "audio/missing.opus", 100)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("MarkAudioDownloaded(missing): got %v, want ErrNotFound", err)
	}
}

func TestMarkAudioDownloaded_NoOpUpdateReturnsNil(t *testing.T) {
	lib, _ := openTempLibrary(t)

	tr := sampleTrack("dQw4w9WgXcQ", 1000)
	if err := lib.InsertTrack(&tr); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	const path = "audio/dQw4w9WgXcQ.opus"
	const size int64 = 1234567

	if err := lib.MarkAudioDownloaded("dQw4w9WgXcQ", path, size); err != nil {
		t.Fatalf("first MarkAudioDownloaded: %v", err)
	}
	if err := lib.MarkAudioDownloaded("dQw4w9WgXcQ", path, size); err != nil {
		t.Fatalf("second MarkAudioDownloaded (no-op): got %v, want nil", err)
	}
}

func TestSetThumbPath_RoundTrip(t *testing.T) {
	lib, _ := openTempLibrary(t)

	tr := sampleTrack("dQw4w9WgXcQ", 1000)
	tr.ThumbPath = ""
	if err := lib.InsertTrack(&tr); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	if err := lib.SetThumbPath("dQw4w9WgXcQ", "thumbs/dQw4w9WgXcQ.jpg"); err != nil {
		t.Fatalf("SetThumbPath: %v", err)
	}

	got, err := lib.GetTrack("dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if got.ThumbPath != "thumbs/dQw4w9WgXcQ.jpg" {
		t.Errorf("ThumbPath: got %q, want thumbs/dQw4w9WgXcQ.jpg", got.ThumbPath)
	}
}

func TestSetThumbPath_NotFound(t *testing.T) {
	lib, _ := openTempLibrary(t)

	err := lib.SetThumbPath("missing", "thumbs/missing.jpg")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("SetThumbPath(missing): got %v, want ErrNotFound", err)
	}
}

func TestRemoveTrack_HappyPath(t *testing.T) {
	lib, _ := openTempLibrary(t)

	tr := sampleTrack("dQw4w9WgXcQ", 1000)
	if err := lib.InsertTrack(&tr); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	if err := lib.RemoveTrack("dQw4w9WgXcQ"); err != nil {
		t.Fatalf("RemoveTrack: %v", err)
	}

	_, err := lib.GetTrack("dQw4w9WgXcQ")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTrack after remove: got %v, want ErrNotFound", err)
	}
}

func TestRemoveTrack_NotFound(t *testing.T) {
	lib, _ := openTempLibrary(t)

	err := lib.RemoveTrack("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("RemoveTrack(missing): got %v, want ErrNotFound", err)
	}
}

func TestInsertTrack_EmptyOptionalFields(t *testing.T) {
	lib, _ := openTempLibrary(t)

	tr := Track{
		VideoID:      "jNQXAC9IVRw",
		Title:        "Me at the zoo",
		Artist:       "",
		Album:        "",
		ReleaseYear:  0,
		DurationSec:  19,
		ThumbnailURL: "https://i.ytimg.com/vi/jNQXAC9IVRw/hqdefault.jpg",
		ThumbPath:    "",
		IsMusic:      false,
		MusicType:    "",
		SourceURL:    "https://www.youtube.com/watch?v=jNQXAC9IVRw",
		ImportedAt:   1500,
		AudioPath:    "",
		AudioSize:    0,
	}
	if err := lib.InsertTrack(&tr); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	got, err := lib.GetTrack("jNQXAC9IVRw")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if got.Artist != "" || got.Album != "" || got.ThumbPath != "" || got.MusicType != "" || got.AudioPath != "" {
		t.Errorf("empty fields not preserved: %+v", got)
	}
	if got.ReleaseYear != 0 || got.AudioSize != 0 {
		t.Errorf("zero numeric fields not preserved: ReleaseYear=%d AudioSize=%d", got.ReleaseYear, got.AudioSize)
	}
}

func TestInsertTrack_UnicodeTitleRoundTrip(t *testing.T) {
	lib, _ := openTempLibrary(t)

	const exotic = "\U0001D4DC\U0001D4FE\U0001D4FC\U0001D4F2\U0001D4EC éè 漢字 \U0001F3B5"
	tr := sampleTrack("uni00000001", 1000)
	tr.Title = exotic
	if err := lib.InsertTrack(&tr); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	got, err := lib.GetTrack("uni00000001")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if got.Title != exotic {
		t.Errorf("unicode title:\n got  %q\n want %q", got.Title, exotic)
	}
}

func TestInsertTrack_RealLengthVideoID(t *testing.T) {
	lib, _ := openTempLibrary(t)

	const id = "dQw4w9WgXcQ"
	if len(id) != 11 {
		t.Fatalf("test setup: id length %d, want 11", len(id))
	}
	tr := sampleTrack(id, 1000)
	if err := lib.InsertTrack(&tr); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	got, err := lib.GetTrack(id)
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if got.VideoID != id {
		t.Errorf("VideoID: got %q, want %q", got.VideoID, id)
	}
}

func TestInsertTrack_LargeAudioSize(t *testing.T) {
	lib, _ := openTempLibrary(t)

	const huge int64 = 9_000_000_000
	tr := sampleTrack("dQw4w9WgXcQ", 1000)
	tr.AudioSize = huge
	tr.AudioPath = "audio/huge.opus"
	if err := lib.InsertTrack(&tr); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	got, err := lib.GetTrack("dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if got.AudioSize != huge {
		t.Errorf("AudioSize: got %d, want %d", got.AudioSize, huge)
	}
}

func TestPersistence_RowSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.db")

	lib, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	tr := sampleTrack("dQw4w9WgXcQ", 1000)
	if err := lib.InsertTrack(&tr); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	lib2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer lib2.Close()

	got, err := lib2.GetTrack("dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("GetTrack after reopen: %v", err)
	}
	if got.ID != tr.ID {
		t.Errorf("ID after reopen: got %d, want %d", got.ID, tr.ID)
	}
}

func TestPersistence_IsMusicTrueSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.db")

	lib, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	tr := sampleTrack("dQw4w9WgXcQ", 1000)
	tr.IsMusic = true
	if err := lib.InsertTrack(&tr); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	lib2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer lib2.Close()

	got, err := lib2.GetTrack("dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("GetTrack after reopen: %v", err)
	}
	if got.IsMusic != true {
		t.Errorf("IsMusic after reopen: got %v, want true", got.IsMusic)
	}
}

func TestConcurrentInsertTracks_NoDeadlock(t *testing.T) {
	lib, _ := openTempLibrary(t)

	const n = 16
	var wg sync.WaitGroup
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tr := sampleTrack(uniqueVideoID(i), int64(1000+i))
			if err := lib.InsertTrack(&tr); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent InsertTrack: %v", err)
	}

	tracks, err := lib.ListTracks()
	if err != nil {
		t.Fatalf("ListTracks: %v", err)
	}
	if len(tracks) != n {
		t.Errorf("ListTracks: got %d rows, want %d", len(tracks), n)
	}
}

func uniqueVideoID(i int) string {
	const base = "vid00000000"
	b := []byte(base)
	b[len(b)-2] = byte('A' + (i / 26))
	b[len(b)-1] = byte('A' + (i % 26))
	return string(b)
}
