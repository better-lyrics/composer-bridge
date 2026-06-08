package activity

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func openTempLog(t *testing.T) (*Log, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "activity.db")
	log, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { log.Close() })
	return log, path
}

func TestOpen_CreatesSchemaAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "activity.db")

	log, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	id, err := log.Start(KindAudioDownload, "dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if id == 0 {
		t.Fatal("Start: returned id is zero")
	}
	if err := log.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	log2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer log2.Close()

	entries, err := log2.Recent(10)
	if err != nil {
		t.Fatalf("Recent after reopen: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Recent after reopen: got %d entries, want 1", len(entries))
	}
	if entries[0].ID != id {
		t.Errorf("ID after reopen: got %d, want %d", entries[0].ID, id)
	}
}

func TestStart_PersistsRunningRow(t *testing.T) {
	log, _ := openTempLog(t)

	before := time.Now().UnixMilli()
	id, err := log.Start(KindImport, "dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	after := time.Now().UnixMilli()

	entries, err := log.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Recent: got %d, want 1", len(entries))
	}
	e := entries[0]
	if e.ID != id {
		t.Errorf("ID: got %d, want %d", e.ID, id)
	}
	if e.Kind != KindImport {
		t.Errorf("Kind: got %q, want %q", e.Kind, KindImport)
	}
	if e.VideoID != "dQw4w9WgXcQ" {
		t.Errorf("VideoID: got %q", e.VideoID)
	}
	if e.Status != StatusRunning {
		t.Errorf("Status: got %q, want %q", e.Status, StatusRunning)
	}
	if e.EndedAt != 0 {
		t.Errorf("EndedAt: got %d, want 0", e.EndedAt)
	}
	if e.Message != "" {
		t.Errorf("Message: got %q, want empty", e.Message)
	}
	if e.StartedAt < before || e.StartedAt > after {
		t.Errorf("StartedAt: got %d, want in [%d, %d]", e.StartedAt, before, after)
	}
}

func TestStartedAtIsMillisecondGranularity(t *testing.T) {
	log, _ := openTempLog(t)

	if _, err := log.Start(KindAudioDownload, "vid1"); err != nil {
		t.Fatalf("Start 1: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, err := log.Start(KindAudioDownload, "vid2"); err != nil {
		t.Fatalf("Start 2: %v", err)
	}

	entries, err := log.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Recent: got %d, want 2", len(entries))
	}
	if entries[0].StartedAt == entries[1].StartedAt {
		t.Errorf("StartedAt: Start calls 2ms apart produced equal timestamps (%d == %d); millisecond granularity expected (seconds-granular Unix() would equate these)", entries[0].StartedAt, entries[1].StartedAt)
	}
}

func TestEnd_FlipsToOkAndSetsEndedAt(t *testing.T) {
	log, _ := openTempLog(t)

	id, err := log.Start(KindAudioDownload, "dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	before := time.Now().UnixMilli()
	if err := log.End(id, StatusOK, "downloaded 4.5MB"); err != nil {
		t.Fatalf("End: %v", err)
	}
	after := time.Now().UnixMilli()

	entries, err := log.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Recent: got %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Status != StatusOK {
		t.Errorf("Status: got %q, want %q", e.Status, StatusOK)
	}
	if e.Message != "downloaded 4.5MB" {
		t.Errorf("Message: got %q", e.Message)
	}
	if e.EndedAt < before || e.EndedAt > after {
		t.Errorf("EndedAt: got %d, want in [%d, %d]", e.EndedAt, before, after)
	}
}

func TestEnd_FlipsToError(t *testing.T) {
	log, _ := openTempLog(t)

	id, err := log.Start(KindYtdlpUpdate, "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := log.End(id, StatusError, "network unreachable"); err != nil {
		t.Fatalf("End: %v", err)
	}

	entries, err := log.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Recent: got %d, want 1", len(entries))
	}
	if entries[0].Status != StatusError {
		t.Errorf("Status: got %q, want %q", entries[0].Status, StatusError)
	}
	if entries[0].Message != "network unreachable" {
		t.Errorf("Message: got %q", entries[0].Message)
	}
	if entries[0].EndedAt <= 0 {
		t.Errorf("EndedAt: got %d, want > 0", entries[0].EndedAt)
	}
}

func TestEnd_UnknownIDReturnsErrNotFound(t *testing.T) {
	log, _ := openTempLog(t)

	err := log.End(99999, StatusOK, "")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("End(missing): got %v, want ErrNotFound", err)
	}
}

func TestEnd_CalledTwiceSecondWins(t *testing.T) {
	log, _ := openTempLog(t)

	id, err := log.Start(KindAudioDownload, "dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := log.End(id, StatusOK, "first call"); err != nil {
		t.Fatalf("first End: %v", err)
	}
	if err := log.End(id, StatusError, "second call"); err != nil {
		t.Fatalf("second End: %v", err)
	}

	entries, err := log.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Recent: got %d, want 1", len(entries))
	}
	if entries[0].Status != StatusError {
		t.Errorf("Status: got %q, want %q (second End must win)", entries[0].Status, StatusError)
	}
	if entries[0].Message != "second call" {
		t.Errorf("Message: got %q, want %q (second End must win)", entries[0].Message, "second call")
	}
}

func TestRecent_OrderedByStartedAtDesc(t *testing.T) {
	log, _ := openTempLog(t)

	id1, err := log.Start(KindAudioDownload, "aaaaaaaaaaa")
	if err != nil {
		t.Fatalf("Start 1: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	id2, err := log.Start(KindImport, "bbbbbbbbbbb")
	if err != nil {
		t.Fatalf("Start 2: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	id3, err := log.Start(KindYtdlpUpdate, "")
	if err != nil {
		t.Fatalf("Start 3: %v", err)
	}

	entries, err := log.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("Recent: got %d, want 3", len(entries))
	}
	wantOrder := []int64{id3, id2, id1}
	for i, want := range wantOrder {
		if entries[i].ID != want {
			t.Errorf("Recent[%d].ID: got %d, want %d", i, entries[i].ID, want)
		}
	}
	for i := 1; i < len(entries); i++ {
		if entries[i-1].StartedAt < entries[i].StartedAt {
			t.Errorf("Recent: entries[%d].StartedAt (%d) < entries[%d].StartedAt (%d)", i-1, entries[i-1].StartedAt, i, entries[i].StartedAt)
		}
	}
}

func TestRecent_LimitZeroReturnsEmpty(t *testing.T) {
	log, _ := openTempLog(t)

	if _, err := log.Start(KindAudioDownload, "dQw4w9WgXcQ"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	entries, err := log.Recent(0)
	if err != nil {
		t.Fatalf("Recent(0): %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Recent(0): got %d entries, want 0", len(entries))
	}
}

func TestRecent_LimitExceedsRowCountReturnsAll(t *testing.T) {
	log, _ := openTempLog(t)

	for i := 0; i < 3; i++ {
		if _, err := log.Start(KindAudioDownload, "id"); err != nil {
			t.Fatalf("Start: %v", err)
		}
	}

	entries, err := log.Recent(50)
	if err != nil {
		t.Fatalf("Recent(50): %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("Recent(50): got %d entries, want 3", len(entries))
	}
}

func TestRecent_EmptyTableReturnsEmpty(t *testing.T) {
	log, _ := openTempLog(t)

	entries, err := log.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Recent on empty table: got %d, want 0", len(entries))
	}
}

func TestStart_EmptyVideoIDSucceeds(t *testing.T) {
	log, _ := openTempLog(t)

	id, err := log.Start(KindYtdlpUpdate, "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if id == 0 {
		t.Fatal("Start: id is zero")
	}

	entries, err := log.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Recent: got %d, want 1", len(entries))
	}
	if entries[0].VideoID != "" {
		t.Errorf("VideoID: got %q, want empty", entries[0].VideoID)
	}
}

func TestEnd_LongMessageRoundTrips(t *testing.T) {
	log, _ := openTempLog(t)

	id, err := log.Start(KindImport, "dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	msg := strings.Repeat("x", 4096)
	if err := log.End(id, StatusError, msg); err != nil {
		t.Fatalf("End: %v", err)
	}

	entries, err := log.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Recent: got %d, want 1", len(entries))
	}
	if entries[0].Message != msg {
		t.Errorf("Message: got len=%d, want len=%d (mismatch)", len(entries[0].Message), len(msg))
	}
}

func TestStatusValuesRoundTrip(t *testing.T) {
	log, _ := openTempLog(t)

	cases := []struct {
		status Status
		want   string
	}{
		{StatusRunning, "running"},
		{StatusOK, "ok"},
		{StatusError, "error"},
	}

	for _, c := range cases {
		id, err := log.Start(KindAudioDownload, "vid")
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if c.status != StatusRunning {
			if err := log.End(id, c.status, "msg"); err != nil {
				t.Fatalf("End: %v", err)
			}
		}
	}

	entries, err := log.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("Recent: got %d, want 3", len(entries))
	}

	seen := map[Status]bool{}
	for _, e := range entries {
		seen[e.Status] = true
		switch string(e.Status) {
		case "running", "ok", "error":
		default:
			t.Errorf("Status: got %q, want one of running/ok/error", e.Status)
		}
	}
	for _, want := range []Status{StatusRunning, StatusOK, StatusError} {
		if !seen[want] {
			t.Errorf("Status %q not observed in Recent results", want)
		}
	}
}

func TestStart_UnknownKindStoredAsIs(t *testing.T) {
	log, _ := openTempLog(t)

	const garbage Kind = "garbage_kind"
	id, err := log.Start(garbage, "vid")
	if err != nil {
		t.Fatalf("Start with unknown kind: %v", err)
	}
	if id == 0 {
		t.Fatal("Start: id is zero")
	}

	entries, err := log.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Recent: got %d, want 1", len(entries))
	}
	if entries[0].Kind != garbage {
		t.Errorf("Kind: got %q, want %q (db layer must not enforce enum)", entries[0].Kind, garbage)
	}
}

func TestPersistence_RowSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "activity.db")

	log, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	id, err := log.Start(KindImport, "dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := log.End(id, StatusOK, "done"); err != nil {
		t.Fatalf("End: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	log2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer log2.Close()

	entries, err := log2.Recent(10)
	if err != nil {
		t.Fatalf("Recent after reopen: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Recent after reopen: got %d, want 1", len(entries))
	}
	if entries[0].ID != id {
		t.Errorf("ID after reopen: got %d, want %d", entries[0].ID, id)
	}
	if entries[0].Status != StatusOK {
		t.Errorf("Status after reopen: got %q, want %q", entries[0].Status, StatusOK)
	}
	if entries[0].Message != "done" {
		t.Errorf("Message after reopen: got %q, want %q", entries[0].Message, "done")
	}
}

func TestPersistence_StartInRun1EndInRun2(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "activity.db")

	log, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	id, err := log.Start(KindAudioDownload, "dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopening the log flips the still-running row to error via
	// RecoverStranded. A subsequent End on the same id must still succeed
	// (the row exists, End overwrites status/message/ended_at) and the End
	// call wins over the boot-time recovery state.
	log2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer log2.Close()

	if err := log2.End(id, StatusOK, "completed across restarts"); err != nil {
		t.Fatalf("End after reopen: %v", err)
	}

	entries, err := log2.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Recent: got %d, want 1", len(entries))
	}
	if entries[0].Status != StatusOK {
		t.Errorf("Status: got %q, want %q (run-2 End must overwrite the recovery-flipped error state)", entries[0].Status, StatusOK)
	}
	if entries[0].Message != "completed across restarts" {
		t.Errorf("Message: got %q (recovery message must not survive a subsequent End)", entries[0].Message)
	}
	if entries[0].EndedAt == 0 {
		t.Errorf("EndedAt: got 0, want > 0")
	}
}

func TestRecent_UsesStartedAtIndex(t *testing.T) {
	log, _ := openTempLog(t)

	for i := 0; i < 50; i++ {
		if _, err := log.Start(KindAudioDownload, "vid"); err != nil {
			t.Fatalf("Start: %v", err)
		}
	}

	rows, err := log.db.Query(`EXPLAIN QUERY PLAN SELECT id, kind, video_id, started_at, ended_at, status, message FROM activity ORDER BY started_at DESC LIMIT 1`)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v", err)
	}
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		cols, err := rows.Columns()
		if err != nil {
			t.Fatalf("Columns: %v", err)
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatalf("Scan: %v", err)
		}
		for _, v := range vals {
			plan.WriteString(" ")
			switch s := v.(type) {
			case string:
				plan.WriteString(s)
			case []byte:
				plan.Write(s)
			}
		}
		plan.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}

	planStr := plan.String()
	if !strings.Contains(planStr, "idx_activity_started_at") {
		t.Errorf("query plan does not use idx_activity_started_at:\n%s", planStr)
	}
}

func TestConcurrentStart_ProducesDistinctIDs(t *testing.T) {
	log, _ := openTempLog(t)

	const n = 16
	var wg sync.WaitGroup
	var ids sync.Map

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := log.Start(KindAudioDownload, uniqueVideoID(i))
			if err != nil {
				t.Errorf("Start[%d]: %v", i, err)
				return
			}
			ids.Store(id, true)
		}(i)
	}
	wg.Wait()

	count := 0
	ids.Range(func(_, _ any) bool {
		count++
		return true
	})
	if count != n {
		t.Errorf("distinct ids: got %d, want %d", count, n)
	}

	entries, err := log.Recent(100)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != n {
		t.Errorf("Recent: got %d rows, want %d", len(entries), n)
	}
}

func uniqueVideoID(i int) string {
	const base = "vid00000000"
	b := []byte(base)
	b[len(b)-2] = byte('A' + (i / 26))
	b[len(b)-1] = byte('A' + (i % 26))
	return string(b)
}

func TestRecoverStranded_FlipsRunningToError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "activity.db")

	log, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	id, err := log.Start(KindAudioDownload, "dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	log2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer log2.Close()

	entries, err := log2.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Recent: got %d, want 1", len(entries))
	}
	e := entries[0]
	if e.ID != id {
		t.Errorf("ID: got %d, want %d", e.ID, id)
	}
	if e.Status != StatusError {
		t.Errorf("Status: got %q, want %q", e.Status, StatusError)
	}
	if e.EndedAt == 0 {
		t.Errorf("EndedAt: got 0, want != 0 (RecoverStranded sets ended_at = started_at)")
	}
	if e.EndedAt != e.StartedAt {
		t.Errorf("EndedAt: got %d, want %d (== StartedAt)", e.EndedAt, e.StartedAt)
	}
	if !strings.Contains(e.Message, "stranded") {
		t.Errorf("Message: got %q, want contains \"stranded\"", e.Message)
	}
}

func TestRecoverStranded_DoesNotTouchEndedRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "activity.db")

	log, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	id, err := log.Start(KindImport, "dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := log.End(id, StatusOK, "done"); err != nil {
		t.Fatalf("End: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	log2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer log2.Close()

	entries, err := log2.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Recent: got %d, want 1", len(entries))
	}
	if entries[0].Status != StatusOK {
		t.Errorf("Status: got %q, want %q (already-ended rows must not be touched)", entries[0].Status, StatusOK)
	}
	if entries[0].Message != "done" {
		t.Errorf("Message: got %q, want %q", entries[0].Message, "done")
	}
}

func TestRecoverStranded_NoRowsToRecoverIsZero(t *testing.T) {
	log, _ := openTempLog(t)

	n, err := log.RecoverStranded()
	if err != nil {
		t.Fatalf("RecoverStranded: %v", err)
	}
	if n != 0 {
		t.Errorf("recovered count: got %d, want 0", n)
	}
}
