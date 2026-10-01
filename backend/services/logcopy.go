package services

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"strings"
)

// LogCopyBytes is how much of the log the bug-report copy takes at most: the
// end of it, where the failure is.
const LogCopyBytes = 256 << 10

// ErrLogEmpty is returned when the log has no complete line to copy.
var ErrLogEmpty = errors.New("the log has nothing in it yet")

// RedactedLogTail reads the last maxBytes of the log at path, from the start
// of a whole line, and returns it with r applied and the number of lines in
// it. Nothing is written anywhere.
//
// The log is open for writing by the logger while this reads, so the file is
// opened read-only and a last line without its newline is dropped: it is
// either mid-write or cut, and half a home path would slip past a redactor
// that matches whole values.
func RedactedLogTail(path string, r *Redactor, maxBytes int64) (string, int, error) {
	raw, err := readLogTail(path, maxBytes)
	if err != nil {
		return "", 0, err
	}
	if len(raw) == 0 {
		return "", 0, ErrLogEmpty
	}
	text := r.Redact(string(raw))
	return text, strings.Count(text, "\n"), nil
}

// OSUserName is the login name of whoever runs the app, without a Windows
// domain prefix, for the redactor. Empty when the OS will not say.
func OSUserName() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return loginName(u.Username)
}

// loginName drops the DOMAIN\ part of a Windows account name.
func loginName(account string) string {
	if i := strings.LastIndex(account, `\`); i >= 0 {
		return account[i+1:]
	}
	return account
}

// readLogTail returns the whole lines within the last maxBytes of the file.
func readLogTail(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path) // read-only: the logger holds it open for append
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrLogEmpty
		}
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only handle, nothing to flush

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat log: %w", err)
	}
	// One byte before the window says whether it opens on a line start; the
	// limit reader keeps a log that grows while we read to the same bound.
	start := max(info.Size()-maxBytes-1, 0)
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek log: %w", err)
	}
	buf, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read log: %w", err)
	}

	if int64(len(buf)) > maxBytes {
		// The file is longer than the window, so buf opens on the byte before
		// it: up to and including the first newline is that byte's line (just
		// the newline itself when the window starts on a line).
		i := bytes.IndexByte(buf, '\n')
		if i < 0 {
			return nil, nil
		}
		buf = buf[i+1:]
	}
	if i := bytes.LastIndexByte(buf, '\n'); i >= 0 {
		return buf[:i+1], nil
	}
	return nil, nil
}
