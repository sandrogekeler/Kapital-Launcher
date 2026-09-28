package services

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// A packaged GUI app has no terminal, so anything written to stdout or stderr
// in a release build goes nowhere. The log file is what a bug report can
// carry, after Redact has run over it.

const (
	// LogFileName is the current log inside the app data dir.
	LogFileName = "kapital-launcher.log"
	// maxLogBytes is where the current log is rotated. One previous file is
	// kept, so the on-disk cost is bounded at roughly twice this.
	maxLogBytes = 2 << 20
)

var (
	logMu   sync.Mutex
	logFile *os.File
)

// LogPath is the current log file's path inside dataDir.
func LogPath(dataDir string) string {
	return filepath.Join(dataDir, LogFileName)
}

// InitLogger points slog's default logger at a file in dataDir and at stderr,
// and returns it. A failure to open the file is not fatal: the app must still
// start on a read-only data dir, so the logger falls back to stderr alone and
// the error is returned for the caller to log through it.
func InitLogger(dataDir string) (*slog.Logger, error) {
	logMu.Lock()
	defer logMu.Unlock()

	var openErr error
	var w io.Writer = os.Stderr

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		openErr = fmt.Errorf("create data dir for log: %w", err)
	} else {
		path := LogPath(dataDir)
		if err := rotateIfLarge(path); err != nil {
			openErr = fmt.Errorf("rotate log: %w", err)
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			openErr = fmt.Errorf("open log file: %w", err)
		} else {
			logFile = f
			w = io.MultiWriter(os.Stderr, f)
		}
	}

	logger := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	return logger, openErr
}

// CloseLogger releases the log file. Safe when InitLogger opened none, and
// safe to call twice.
func CloseLogger() error {
	logMu.Lock()
	defer logMu.Unlock()
	if logFile == nil {
		return nil
	}
	err := logFile.Close()
	logFile = nil
	return err
}

// rotateIfLarge renames the current log aside once it passes maxLogBytes,
// keeping exactly one previous file. A missing file is the first-run case.
func rotateIfLarge(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Size() < maxLogBytes {
		return nil
	}
	return os.Rename(path, path+".1")
}
