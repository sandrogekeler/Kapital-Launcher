package services

import (
	"regexp"
	"strings"
)

// Game and Prism logs carry the Minecraft username, the user's home path and
// the server's address (docs/HANDOFF.md §6). Redact is what the "copy log"
// action runs a log through before anything is pasted into a bug report.
//
// It is deliberately conservative in what it recognises: an IPv4 address, a
// home directory, the account name and each known server. Anything else in
// the log is the user's to read before sharing.

var ipv4 = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}(?::\d{1,5})?\b`)

// Redactor holds the values that identify a user and replaces each with a
// fixed marker.
type Redactor struct {
	replacements []replacement
}

type replacement struct {
	from, to string
}

// NewRedactor builds a redactor for one user: their home directory, the
// account name Prism shows, and every server address the manifest names.
// Empty values are skipped so a blank setting never becomes a match-all.
func NewRedactor(home, username string, servers []string) *Redactor {
	r := &Redactor{}
	add := func(from, to string) {
		if strings.TrimSpace(from) != "" {
			r.replacements = append(r.replacements, replacement{from, to})
		}
	}
	for _, s := range servers {
		add(s, "[server]")
	}
	add(home, "[home]")
	// Windows logs mix separators; the same path with forward slashes is a
	// separate string to a plain replace.
	if strings.Contains(home, `\`) {
		add(strings.ReplaceAll(home, `\`, "/"), "[home]")
	}
	add(username, "[user]")
	return r
}

// Redact returns the text with every identifying value replaced.
func (r *Redactor) Redact(text string) string {
	for _, rep := range r.replacements {
		text = strings.ReplaceAll(text, rep.from, rep.to)
	}
	return ipv4.ReplaceAllString(text, "[ip]")
}
