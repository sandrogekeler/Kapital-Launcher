package services

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kapital/backend/models"
)

// phasesOf feeds a log's lines to a parser and returns the phases it moved
// through, in order.
func phasesOf(lines []string) []string {
	var p logParser
	var out []string
	for _, line := range lines {
		if phase, moved := p.Line(line); moved {
			out = append(out, phase)
		}
	}
	return out
}

func fixtureLines(t *testing.T, name, eol string) []string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "gamelog", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // read-only file, nothing to flush
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		lines = append(lines, sc.Text()+eol)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return lines
}

func TestParserFollowsTheMarkers(t *testing.T) {
	const render = "[00:00:01] [Render thread/INFO]: "
	cases := []struct {
		name  string
		lines []string
		want  []string
	}{
		{"first line of any kind is the log beginning",
			[]string{"Loading Minecraft 1.20.6 with Fabric Loader 0.19.5"},
			[]string{"mods"}},
		{"a marker on the first line counts as well",
			[]string{render + "Backend library: LWJGL version 3.3.3"},
			[]string{"window"}},
		{"vanilla order",
			[]string{"first", render + "Backend library: LWJGL", render + "Reloading ResourceManager: vanilla", render + "Sound engine started", render + "Stopping!"},
			[]string{"mods", "window", "resources", "running", "stopping"}},
		{"a marker for an earlier phase is ignored",
			[]string{"first", render + "Reloading ResourceManager", render + "Backend library: LWJGL", "noise"},
			[]string{"mods", "resources"}},
		{"a phase can be skipped",
			[]string{"first", render + "Sound engine started"},
			[]string{"mods", "running"}},
		{"sound engine is ready without FancyMenu",
			[]string{"first", render + "Sound engine started", render + "[FANCYMENU] Minecraft resource reload: FINISHED"},
			[]string{"mods", "running"}},
		{"FancyMenu before the sound engine moves ready to its FINISHED line",
			[]string{"first", render + "[FANCYMENU] Reloading FancyMenu..", render + "Sound engine started", "noise", render + "[FANCYMENU] Minecraft resource reload: FINISHED"},
			[]string{"mods", "running"}},
		{"FancyMenu after the sound engine does not undo ready",
			[]string{"first", render + "Sound engine started", render + "[FANCYMENU] Reloading FancyMenu.."},
			[]string{"mods", "running"}},
		{"a FancyMenu line that is not FINISHED is not ready",
			[]string{"first", render + "[FANCYMENU] Reloading FancyMenu..", render + "Sound engine started"},
			[]string{"mods"}},
		{"carriage returns are tolerated",
			[]string{"first\r\n", render + "Backend library: LWJGL\r\n", render + "Stopping!\r\n"},
			[]string{"mods", "window", "stopping"}},
		{"a mod's Stopping! is not the client's",
			[]string{"first", "[Worker-Main-1/INFO] [some.mod/]: Stopping!"},
			[]string{"mods"}},
		{"the client's Stopping! with the class name counts",
			[]string{"first", "[main/INFO] [net.minecraft.client.Minecraft/]: Stopping!"},
			[]string{"mods", "stopping"}},
		{"Stopping! inside a longer line is not the marker",
			[]string{"first", render + "Stopping! the world"},
			[]string{"mods"}},
		{"nothing after stopping moves it back",
			[]string{"first", render + "Stopping!", render + "Backend library: LWJGL", render + "Sound engine started"},
			[]string{"mods", "stopping"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := phasesOf(c.lines); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestParserOnRealShapedLogs(t *testing.T) {
	full := []string{models.GamePhaseMods, models.GamePhaseWindow, models.GamePhaseResources, models.GamePhaseRunning, models.GamePhaseStopping}
	cases := []struct {
		file string
		want []string
	}{
		{"frangfurd-neoforge.log", full},
		{"fabric-vanilla.log", full},
		{"crash-after-resources.log", full[:3]},
	}
	for _, c := range cases {
		for _, eol := range []string{"\n", "\r\n"} {
			t.Run(c.file+strings.ReplaceAll(eol, "\r\n", "-crlf"), func(t *testing.T) {
				if got := phasesOf(fixtureLines(t, c.file, eol)); !reflect.DeepEqual(got, c.want) {
					t.Fatalf("got %v want %v", got, c.want)
				}
			})
		}
	}
}

func TestParserReadyIsFancyMenuFinishedWhenFancyMenuSpokeFirst(t *testing.T) {
	// The NeoForge fixture has a FancyMenu line before "Sound engine started":
	// ready must wait for FINISHED, which comes seven lines later.
	lines := fixtureLines(t, "frangfurd-neoforge.log", "\n")
	var p logParser
	for _, line := range lines {
		_, moved := p.Line(line)
		if strings.Contains(line, "Sound engine started") && (moved || p.Phase() == models.GamePhaseRunning) {
			t.Fatalf("the sound engine alone is not ready once FancyMenu has spoken: %q", p.Phase())
		}
		if strings.Contains(line, "resource reload: FINISHED") && p.Phase() != models.GamePhaseRunning {
			t.Fatalf("FINISHED must be ready: %q", p.Phase())
		}
	}
}
