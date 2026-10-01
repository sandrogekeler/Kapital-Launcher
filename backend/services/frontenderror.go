package services

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// A packaged build has no console, so an error in the view reaches the log
// file only if the frontend sends it here. The webview is not trusted with the
// log: the kind is a closed set, both texts are cut and stripped of control
// characters (a line break inside a message must not forge a log line), and a
// render loop is stopped after a few lines.

const (
	maxFrontendMessage = 1 << 10
	maxFrontendStack   = 8 << 10
	// maxFrontendErrors is how many errors one run writes. The next one writes
	// a single line saying the rest are dropped.
	maxFrontendErrors = 20
)

// frontendKinds are the three places an error is caught: React's boundary,
// window's error event, and an unhandled promise rejection.
var frontendKinds = map[string]bool{"render": true, "error": true, "rejection": true}

// FrontendErrorLog writes errors reported by the frontend to the log, capped
// per run.
type FrontendErrorLog struct {
	// logger is nil outside tests, which means the default logger at call time.
	logger *slog.Logger

	mu      sync.Mutex
	written int
	dropped bool
}

// NewFrontendErrorLog returns a log with a fresh per-run budget.
func NewFrontendErrorLog() *FrontendErrorLog {
	return &FrontendErrorLog{}
}

// Log records one frontend error. An unknown kind is refused and does not
// count against the cap.
func (l *FrontendErrorLog) Log(kind, message, stack string) error {
	if !frontendKinds[kind] {
		return fmt.Errorf("frontend error: %q is not a known kind", kind)
	}
	logger := l.logger
	if logger == nil {
		logger = slog.Default()
	}

	l.mu.Lock()
	if l.written >= maxFrontendErrors {
		first := !l.dropped
		l.dropped = true
		l.mu.Unlock()
		if first {
			logger.Warn("frontend errors dropped", "limit", maxFrontendErrors)
		}
		return nil
	}
	l.written++
	l.mu.Unlock()

	logger.Error("frontend error",
		"kind", kind,
		"message", cleanFrontendText(message, maxFrontendMessage),
		"stack", cleanFrontendText(stack, maxFrontendStack),
	)
	return nil
}

// cleanFrontendText drops control characters other than newline and tab, then
// cuts the result to at most max bytes without splitting a character.
func cleanFrontendText(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
