package localfs

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

func TestUploadSnapshotter_InitialRunReturnsCurrentAsPreviousAndStoresSnapshot(t *testing.T) {
	uploadDir := t.TempDir()
	snapshotDir := t.TempDir()
	writeFile(t, filepath.Join(uploadDir, "root.txt"), "root content")
	writeFile(t, filepath.Join(uploadDir, "nested", "child.txt"), "child content")

	snapshotter := New(uploadDir, snapshotDir, WithClock(fixedClock(time.Date(2026, time.May, 5, 9, 1, 2, 3, time.UTC))))
	previous, current, err := snapshotter.GetPreviousAndCurrentSnapshot()
	if err != nil {
		t.Fatalf("GetPreviousAndCurrentSnapshot returned error: %v", err)
	}

	if !reflect.DeepEqual(previous, current) {
		t.Fatalf("expected initial previous snapshot to duplicate current\nprevious=%+v\ncurrent=%+v", previous, current)
	}

	expected := entities.UploadSnapshot{
		absPath(t, filepath.Join(uploadDir, "root.txt")):            sha1String("root content"),
		absPath(t, filepath.Join(uploadDir, "nested", "child.txt")): sha1String("child content"),
	}
	if !reflect.DeepEqual(current, expected) {
		t.Fatalf("unexpected current snapshot\nwant=%+v\ngot=%+v", expected, current)
	}

	stored := readOnlyStoredSnapshot(t, snapshotDir)
	if !reflect.DeepEqual(stored, current) {
		t.Fatalf("stored snapshot mismatch\nwant=%+v\ngot=%+v", current, stored)
	}
}

func TestUploadSnapshotter_LoadsLatestPreviousSnapshotBySortableFilename(t *testing.T) {
	uploadDir := t.TempDir()
	snapshotDir := t.TempDir()
	writeFile(t, filepath.Join(uploadDir, "upload.txt"), "current")

	oldSnapshot := entities.UploadSnapshot{"/old.txt": "old"}
	latestSnapshot := entities.UploadSnapshot{"/latest.txt": "latest"}
	writeSnapshot(t, filepath.Join(snapshotDir, "upload-snapshot-20260505T090000.000000000Z.json"), oldSnapshot)
	writeSnapshot(t, filepath.Join(snapshotDir, "upload-snapshot-20260505T100000.000000000Z.json"), latestSnapshot)
	writeFile(t, filepath.Join(snapshotDir, "ignore.txt"), "not json")

	snapshotter := New(uploadDir, snapshotDir, WithClock(fixedClock(time.Date(2026, time.May, 5, 11, 0, 0, 0, time.UTC))))
	previous, current, err := snapshotter.GetPreviousAndCurrentSnapshot()
	if err != nil {
		t.Fatalf("GetPreviousAndCurrentSnapshot returned error: %v", err)
	}

	if !reflect.DeepEqual(previous, latestSnapshot) {
		t.Fatalf("expected latest previous snapshot\nwant=%+v\ngot=%+v", latestSnapshot, previous)
	}

	expectedCurrent := entities.UploadSnapshot{
		absPath(t, filepath.Join(uploadDir, "upload.txt")): sha1String("current"),
	}
	if !reflect.DeepEqual(current, expectedCurrent) {
		t.Fatalf("unexpected current snapshot\nwant=%+v\ngot=%+v", expectedCurrent, current)
	}

	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		t.Fatalf("read snapshot dir: %v", err)
	}
	var jsonFiles int
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			jsonFiles++
		}
	}
	if jsonFiles != 3 {
		t.Fatalf("expected current run to store a third JSON snapshot, got %d", jsonFiles)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeSnapshot(t *testing.T, path string, snapshot entities.UploadSnapshot) {
	t.Helper()
	bytes, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatalf("encode snapshot: %v", err)
	}
	writeFile(t, path, string(bytes))
}

func readOnlyStoredSnapshot(t *testing.T, snapshotDir string) entities.UploadSnapshot {
	t.Helper()
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		t.Fatalf("read snapshot dir: %v", err)
	}
	var files []os.DirEntry
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			files = append(files, entry)
		}
	}
	if len(files) != 1 {
		t.Fatalf("expected exactly one stored snapshot, got %d", len(files))
	}

	bytes, err := os.ReadFile(filepath.Join(snapshotDir, files[0].Name()))
	if err != nil {
		t.Fatalf("read stored snapshot: %v", err)
	}

	var snapshot entities.UploadSnapshot
	if err := json.Unmarshal(bytes, &snapshot); err != nil {
		t.Fatalf("decode stored snapshot: %v", err)
	}
	return snapshot
}

func absPath(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("resolve absolute path: %v", err)
	}
	return abs
}

func sha1String(content string) string {
	sum := sha1.Sum([]byte(content))
	return hex.EncodeToString(sum[:])
}

func fixedClock(at time.Time) Clock {
	return func() time.Time {
		return at
	}
}
