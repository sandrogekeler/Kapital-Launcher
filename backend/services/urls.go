package services

import (
	"fmt"
	"net/url"
)

// ExternalURL checks a URL before it is handed to the system browser. Only
// http and https ever reach it: a file:, javascript: or custom-scheme URL
// would open something other than a web page (agent_docs/SECURITY_CHECKLIST.md, S5).
func ExternalURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("open url: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("open url: %q is not a web address", raw)
	}
	if u.Host == "" {
		return "", fmt.Errorf("open url: %q has no host", raw)
	}
	return u.String(), nil
}
