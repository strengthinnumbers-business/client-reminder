package localfs

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
)

var _ ports.UploadSnapshotter = (*UploadSnapshotter)(nil)

type Clock func() time.Time

type UploadSnapshotter struct {
	uploadDir   string
	snapshotDir string
	clock       Clock
	logger      ports.Logger
	mu          sync.Mutex
}

type Option func(*UploadSnapshotter)

func New(uploadDir, snapshotDir string, options ...Option) *UploadSnapshotter {
	s := &UploadSnapshotter{
		uploadDir:   uploadDir,
		snapshotDir: snapshotDir,
		clock:       time.Now,
		logger:      ports.NoopLogger{},
	}
	for _, option := range options {
		option(s)
	}
	s.logger = ports.EnsureLogger(s.logger)
	return s
}

func WithClock(clock Clock) Option {
	return func(s *UploadSnapshotter) {
		if clock != nil {
			s.clock = clock
		}
	}
}

func WithLogger(logger ports.Logger) Option {
	return func(s *UploadSnapshotter) {
		s.logger = ports.EnsureLogger(logger)
	}
}

func (s *UploadSnapshotter) GetPreviousAndCurrentSnapshot() (entities.UploadSnapshot, entities.UploadSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previousPath, ok, err := s.latestSnapshotPath()
	if err != nil {
		return nil, nil, err
	}

	current, err := s.currentSnapshot()
	if err != nil {
		return nil, nil, err
	}

	previous := cloneSnapshot(current)
	if ok {
		previous, err = s.readSnapshot(previousPath)
		if err != nil {
			return nil, nil, err
		}
	}

	if err := s.storeSnapshot(current); err != nil {
		return nil, nil, err
	}

	return previous, current, nil
}

func (s *UploadSnapshotter) latestSnapshotPath() (string, bool, error) {
	entries, err := os.ReadDir(s.snapshotDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read upload snapshot directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		names = append(names, entry.Name())
	}
	if len(names) == 0 {
		return "", false, nil
	}

	sort.Strings(names)
	return filepath.Join(s.snapshotDir, names[len(names)-1]), true, nil
}

func (s *UploadSnapshotter) currentSnapshot() (entities.UploadSnapshot, error) {
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return nil, fmt.Errorf("resolve upload snapshot root: %w", err)
	}

	snapshot := entities.UploadSnapshot{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		checksum, err := sha1File(path)
		if err != nil {
			return err
		}
		snapshot[path] = checksum
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk upload snapshot root: %w", err)
	}

	s.logger.Demo("created upload snapshot", "upload_dir", root, "files", len(snapshot))
	return snapshot, nil
}

func sha1File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for checksum %s: %w", path, err)
	}
	defer file.Close()

	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("checksum file %s: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *UploadSnapshotter) readSnapshot(path string) (entities.UploadSnapshot, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read upload snapshot %s: %w", path, err)
	}

	var snapshot entities.UploadSnapshot
	if err := json.Unmarshal(bytes, &snapshot); err != nil {
		return nil, fmt.Errorf("decode upload snapshot %s: %w", path, err)
	}
	if snapshot == nil {
		snapshot = entities.UploadSnapshot{}
	}

	s.logger.Demo("loaded previous upload snapshot", "path", path, "files", len(snapshot))
	return snapshot, nil
}

func (s *UploadSnapshotter) storeSnapshot(snapshot entities.UploadSnapshot) error {
	if err := os.MkdirAll(s.snapshotDir, 0o755); err != nil {
		return fmt.Errorf("create upload snapshot directory: %w", err)
	}

	bytes, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode upload snapshot: %w", err)
	}

	path := filepath.Join(s.snapshotDir, snapshotFilename(s.clock().UTC()))
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		return fmt.Errorf("write upload snapshot: %w", err)
	}
	s.logger.Demo("stored upload snapshot", "path", path, "files", len(snapshot))
	return nil
}

func snapshotFilename(at time.Time) string {
	return "upload-snapshot-" + at.Format("20060102T150405.000000000Z") + ".json"
}

func cloneSnapshot(snapshot entities.UploadSnapshot) entities.UploadSnapshot {
	clone := make(entities.UploadSnapshot, len(snapshot))
	for path, checksum := range snapshot {
		clone[path] = checksum
	}
	return clone
}
