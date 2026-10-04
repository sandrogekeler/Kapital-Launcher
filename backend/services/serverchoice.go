package services

import (
	"fmt"
	"regexp"
	"strings"

	"kapital/backend/models"
)

// serverLabelPattern is what a server address's label may be: short, starting
// with a letter or digit, then letters, digits, spaces, dots and hyphens. A
// label is shown on the settings screen and saved as a chapter's choice, never
// run or put on a command line, so the set is about legibility.
var serverLabelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 .-]{0,23}$`)

// validateServer holds a manifest chapter's server to the rules a schema
// cannot state: at least one address, every address host[:port] (the one rule
// that keeps it safe for Prism's --server and the ping), every label
// well-formed and unique within the chapter. A nil server is a chapter with
// none.
func validateServer(where string, s *models.Server) error {
	if s == nil {
		return nil
	}
	if len(s.Addresses) == 0 {
		return fmt.Errorf("%s: server: no addresses", where)
	}
	labels := map[string]bool{}
	for i, a := range s.Addresses {
		if !serverLabelPattern.MatchString(a.Label) {
			return fmt.Errorf("%s: server: addresses[%d] label %q is not 1 to 24 letters, digits, spaces, dots and hyphens", where, i, a.Label)
		}
		key := strings.ToLower(a.Label)
		if labels[key] {
			return fmt.Errorf("%s: server: label %q appears twice", where, a.Label)
		}
		labels[key] = true
		if _, _, err := ParseServerAddress(a.Address); err != nil {
			return fmt.Errorf("%s: server: addresses[%d]: %w", where, i, err)
		}
	}
	return nil
}

// ServerAddress is the address a chapter's server is reached at: the one its
// saved choice names, else the chapter's first when the choice is missing or
// names a label no longer in the manifest. It is the one place that turns a
// choice into an address, and the ping, the ticker and Play's --server all go
// through it. "" for a chapter with no server.
func ServerAddress(chapter models.Chapter, choices map[string]string) string {
	if chapter.Server == nil || len(chapter.Server.Addresses) == 0 {
		return ""
	}
	if label, ok := choices[chapter.ID]; ok {
		for _, a := range chapter.Server.Addresses {
			if a.Label == label {
				return a.Address
			}
		}
	}
	return chapter.Server.Addresses[0].Address
}

// ServerAddresses lists every address of every chapter's server, for what
// must mask them all (the redactor), whichever one is chosen.
func ServerAddresses(chapters []models.Chapter) []string {
	var all []string
	for _, c := range chapters {
		if c.Server == nil {
			continue
		}
		for _, a := range c.Server.Addresses {
			all = append(all, a.Address)
		}
	}
	return all
}

// ValidateServerChoices refuses a save whose choices name a chapter the
// manifest does not have, a chapter with no server, or a label that is not in
// that chapter's own list. What is saved is a label, so this is the whole
// check: no address typed on the settings screen goes anywhere.
func ValidateServerChoices(chapters []models.Chapter, choices map[string]string) error {
	for id, label := range choices {
		var server *models.Server
		for _, c := range chapters {
			if c.ID == id {
				server = c.Server
			}
		}
		if server == nil {
			return fmt.Errorf("settings: server choice for %q: no such chapter with a server", id)
		}
		found := false
		for _, a := range server.Addresses {
			found = found || a.Label == label
		}
		if !found {
			return fmt.Errorf("settings: server choice %q is not one of %s's addresses", label, id)
		}
	}
	return nil
}

// PruneServerChoices drops what the manifest no longer lists: a choice for a
// chapter or label that is gone. GetSettings hands the screen the pruned map,
// so a stale label on file is never written back (and so refused) by a save
// of something else.
func PruneServerChoices(chapters []models.Chapter, choices map[string]string) map[string]string {
	var kept map[string]string
	for id, label := range choices {
		if ValidateServerChoices(chapters, map[string]string{id: label}) != nil {
			continue
		}
		if kept == nil {
			kept = map[string]string{}
		}
		kept[id] = label
	}
	return kept
}
