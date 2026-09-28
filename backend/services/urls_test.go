package services

import "testing"

func TestExternalURLAcceptsOnlyWebAddresses(t *testing.T) {
	for _, ok := range []string{"https://prismlauncher.org", "http://localhost:8080/x", "https://kapitel-kapital.pages.dev/wiki/x"} {
		if _, err := ExternalURL(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"file:///etc/passwd", "javascript:alert(1)", "prismlauncher://x", "https://", "not a url at all", ""} {
		if _, err := ExternalURL(bad); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
}
