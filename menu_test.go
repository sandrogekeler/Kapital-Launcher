package main

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/menu"
)

func TestAppMenuIsMacOnly(t *testing.T) {
	for _, goos := range []string{"windows", "linux"} {
		if m := appMenu(goos); m != nil {
			t.Errorf("%s: a menu bar of %d items, want none", goos, len(m.Items))
		}
	}
	m := appMenu("darwin")
	if m == nil {
		t.Fatal("darwin: no menu bar, so the Cmd shortcuts reach nothing")
	}
	want := []menu.Role{menu.AppMenuRole, menu.EditMenuRole, menu.WindowMenuRole}
	if len(m.Items) != len(want) {
		t.Fatalf("darwin: %d items, want %d", len(m.Items), len(want))
	}
	for i, role := range want {
		if m.Items[i].Role != role {
			t.Errorf("darwin item %d: role %v, want %v", i, m.Items[i].Role, role)
		}
	}
}
