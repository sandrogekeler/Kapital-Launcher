package services

import (
	"net"
	"path"
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
	// names are OS user names, matched as whole words after the home path has
	// gone, so a name that is also a common word only costs that word.
	names []*regexp.Regexp
}

type replacement struct {
	from, to string
}

// NewRedactor builds a redactor for one user: their home directory, the
// profile name Prism is given, and every server address the manifest names.
// The OS user name is masked too, wherever it appears outside the home path:
// the home directory's own last element, and any osUsers the caller adds (the
// login name can differ from the folder). Empty values are skipped so a blank
// setting never becomes a match-all.
func NewRedactor(home, username string, servers []string, osUsers ...string) *Redactor {
	r := &Redactor{}
	add := func(from, to string) {
		if strings.TrimSpace(from) != "" {
			r.replacements = append(r.replacements, replacement{from, to})
		}
	}
	for _, s := range servers {
		add(s, "[server]")
		// A log also names the host alone, without the port it was pinged on.
		if host, _, err := net.SplitHostPort(s); err == nil {
			add(host, "[server]")
		}
	}
	add(home, "[home]")
	// Windows logs mix separators; the same path with forward slashes is a
	// separate string to a plain replace.
	if strings.Contains(home, `\`) {
		add(strings.ReplaceAll(home, `\`, "/"), "[home]")
	}
	add(username, "[user]")
	for _, name := range append([]string{homeUser(home)}, osUsers...) {
		if name = strings.TrimSpace(name); name != "" && name != "." && name != "/" {
			r.names = append(r.names, regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`))
		}
	}
	return r
}

// homeUser is the last element of a home directory, whichever separator it
// uses: the OS user name on every platform's usual layout.
func homeUser(home string) string {
	return path.Base(strings.ReplaceAll(strings.TrimRight(home, `/\`), `\`, "/"))
}

// Redact returns the text with every identifying value replaced.
func (r *Redactor) Redact(text string) string {
	for _, rep := range r.replacements {
		text = strings.ReplaceAll(text, rep.from, rep.to)
	}
	for _, name := range r.names {
		text = name.ReplaceAllString(text, "[user]")
	}
	return ipv4.ReplaceAllString(text, "[ip]")
}
