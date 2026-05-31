package demolog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const KeyLength = 12

type Source struct {
	Path     string
	Line     int
	Function string
	Message  string
}

func SourceMapKey(source Source) string {
	raw := fmt.Sprintf("%s\n%d\n%s\n%s", source.Path, source.Line, source.Function, source.Message)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])[:KeyLength]
}

func CallerSource(message string) Source {
	pcs := make([]uintptr, 24)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if isApplicationFrame(frame.Function) {
			return Source{
				Path:     repoRelativePath(frame.File),
				Line:     frame.Line,
				Function: frame.Function,
				Message:  message,
			}
		}
		if !more {
			break
		}
	}
	return Source{Message: message}
}

func repoRelativePath(path string) string {
	workingDir, err := os.Getwd()
	if err != nil {
		return filepath.ToSlash(path)
	}
	relative, err := filepath.Rel(workingDir, path)
	if err != nil || strings.HasPrefix(relative, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func CallerSourceMapKey(message string) string {
	return SourceMapKey(CallerSource(message))
}

func isApplicationFrame(function string) bool {
	return function != "" &&
		!strings.Contains(function, "/internal/demolog.") &&
		!strings.Contains(function, "/internal/adapters/logging/")
}
