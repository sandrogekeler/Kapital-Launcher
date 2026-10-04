package services

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// SyncCopy keeps a copy of the launcher's own executable in its data folder,
// for Prism's pre-launch command to run (issue 156, ADR-2's ninth amendment):
//
//	<data dir>/sync/kapital-launcher(.exe)         a release build
//	<data dir>/sync/kapital-launcher.version       what the copy was made from
//	<data dir>/sync-dev/...                        the same for a dev build
//
// The command names the copy and not the running launcher, so its path never
// moves: a Play from Prism directly keeps working after the launcher is moved or
// updated, and a macOS app run from a translocated path cannot break it. The
// copy is the same program that branches on --prelaunch-sync before it opens any
// window (main.go), so there is nothing else to ship.
//
// A dev build (`wails dev`, a plain go build: version "...-dev") has its own
// folder, so it never overwrites a release copy with a binary that is rebuilt on
// every change and points at nothing once the dev server is gone; the pre-launch
// command follows whichever build last played, as the rewrite before every Play
// names the copy of the build that runs it.
type SyncCopy struct {
	dataDir string
	version string
	goos    string
	// source is the running executable's path; a test swaps it.
	source func() (string, error)
	mu     sync.Mutex
}

// NewSyncCopy is the copy for a launcher of the given version (the build's
// Version), kept under dataDir.
func NewSyncCopy(dataDir, version string) *SyncCopy {
	return NewSyncCopyFrom(dataDir, version, os.Executable)
}

// NewSyncCopyFrom is NewSyncCopy with another file to copy than the running
// executable, which a test of the App needs: the test binary is tens of
// megabytes.
func NewSyncCopyFrom(dataDir, version string, source func() (string, error)) *SyncCopy {
	return &SyncCopy{dataDir: dataDir, version: version, goos: runtime.GOOS, source: source}
}

// isDevVersion is a build that is not a release: the "-dev" suffix of
// version.go.
func isDevVersion(version string) bool { return strings.HasSuffix(version, "-dev") }

// dir is the folder the copy lives in.
func (c *SyncCopy) dir() string {
	name := syncDirName
	if isDevVersion(c.version) {
		name = syncDevDirName
	}
	return filepath.Join(c.dataDir, name)
}

func (c *SyncCopy) exeName() string {
	if c.goos == "windows" {
		return syncExeBase + ".exe"
	}
	return syncExeBase
}

// Exists reports whether a copy is already kept for this kind of build. The
// launcher refreshes an existing copy when it starts, so a Play from Prism
// directly after an update runs this version's sync, and makes the first one
// only when an instance needs it.
func (c *SyncCopy) Exists() bool {
	info, err := os.Stat(filepath.Join(c.dir(), c.exeName()))
	return err == nil && info.Mode().IsRegular()
}

// Path is Ensure for a caller that goes on without a copy: the path, or "" with
// the reason logged, which makes the pre-launch command the packwiz one.
func (c *SyncCopy) Path() string {
	path, err := c.Ensure()
	if err != nil {
		slog.Warn("sync copy not available", "error", err)
		return ""
	}
	return path
}

// Ensure returns the copy's path, making or refreshing the copy first when it is
// missing, is not the size it was written at, or was made by another version of
// the launcher. A dev build also refreshes it when its own file changed, since
// every rebuild shares the version. The write is atomic: the executable goes to
// a temporary file, then over the copy, then the version file; a Prism that runs
// the copy in the middle sees the old one or the new one whole.
func (c *SyncCopy) Ensure() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	src, err := c.source()
	if err != nil {
		return "", fmt.Errorf("sync copy: find the running launcher: %w", err)
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return "", fmt.Errorf("sync copy: %w", err)
	}
	if !srcInfo.Mode().IsRegular() {
		return "", errors.New("sync copy: the running launcher is not a file")
	}
	dst := filepath.Join(c.dir(), c.exeName())
	if err := checkSyncExePath(dst); err != nil {
		return "", err
	}
	stamp := filepath.Join(c.dir(), syncExeBase+".version")
	wantStamp := c.stampFor(srcInfo.Size(), srcInfo.ModTime().UnixNano())
	if c.current(dst, stamp, wantStamp) {
		return dst, nil
	}
	if err := c.copyFile(src, dst); err != nil {
		return "", fmt.Errorf("sync copy: %w", err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		return "", fmt.Errorf("sync copy: %w", err)
	}
	if err := writeFileAtomic(stamp, []byte(wantStamp+strconv.FormatInt(info.Size(), 10)+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("sync copy: %w", err)
	}
	slog.Info("sync copy refreshed", "version", c.version, "dev", isDevVersion(c.version))
	return dst, nil
}

// stampFor is the first part of the version file: the version, and for a dev
// build the size and time of the file it was made from.
func (c *SyncCopy) stampFor(srcSize, srcMod int64) string {
	if isDevVersion(c.version) {
		return c.version + "\n" + strconv.FormatInt(srcSize, 10) + ":" + strconv.FormatInt(srcMod, 10) + "\n"
	}
	return c.version + "\n"
}

// current says whether the copy on disk is the one the stamp describes: the
// stamp is what wantStamp says followed by the copy's own size, and the copy is
// a regular file of that size.
func (c *SyncCopy) current(dst, stamp, wantStamp string) bool {
	raw, err := os.ReadFile(stamp)
	if err != nil || len(raw) > 512 {
		return false
	}
	rest, ok := strings.CutPrefix(string(raw), wantStamp)
	if !ok {
		return false
	}
	size, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
	if err != nil {
		return false
	}
	info, err := os.Stat(dst)
	return err == nil && info.Mode().IsRegular() && info.Size() == size
}

// copyFile writes src over dst through a temporary file beside it.
func (c *SyncCopy) copyFile(src, dst string) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // read-only file
	tmp, err := os.CreateTemp(dir, filepath.Base(dst)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	fail := func(err error) error {
		if closeErr := tmp.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		if rmErr := os.Remove(tmpName); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			err = errors.Join(err, rmErr)
		}
		return err
	}
	if _, err := io.Copy(tmp, in); err != nil {
		return fail(err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		if rmErr := os.Remove(tmpName); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			err = errors.Join(err, rmErr)
		}
		return err
	}
	if err := replaceExecutable(tmpName, dst, c.goos); err != nil {
		if rmErr := os.Remove(tmpName); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			err = errors.Join(err, rmErr)
		}
		return err
	}
	return nil
}

// replaceExecutable renames tmp over dst. A program that is running cannot be
// replaced on Windows, though it can be renamed, so a copy a sync is running
// from moves aside to ".old" first; the next refresh removes that.
func replaceExecutable(tmp, dst, goos string) error {
	err := os.Rename(tmp, dst)
	if err == nil || goos != "windows" {
		return err
	}
	old := dst + ".old"
	if rmErr := os.Remove(old); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return err
	}
	if mvErr := os.Rename(dst, old); mvErr != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
