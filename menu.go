package main

import "github.com/wailsapp/wails/v2/pkg/menu"

// appMenu is the window's menu bar: on macOS the App, Edit and Window menus
// Wails builds from their roles, and nothing elsewhere (#225). A Mac's Cmd+C,
// Cmd+V, Cmd+A, Cmd+Z, Cmd+Q and Cmd+M are key equivalents of those menus'
// items, and Wails installs no menu bar unless given one, so without it they
// reached nothing. Windows and Linux are frameless and draw the header's own
// buttons (ADR-10), so they keep no menu bar.
func appMenu(goos string) *menu.Menu {
	if goos != "darwin" {
		return nil
	}
	return menu.NewMenuFromItems(menu.AppMenu(), menu.EditMenu(), menu.WindowMenu())
}
