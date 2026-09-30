package services

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"kapital/backend/models"
)

const render = "[00:00:01] [Render thread/INFO]: "

// gameLog is an instance folder with a game log the test writes and ages.
type gameLog struct {
	t    *testing.T
	dir  string
	path string
}

func newGameLog(t *testing.T) *gameLog {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "minecraft", "logs", "latest.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	return &gameLog{t: t, dir: dir, path: path}
}

// write replaces the log's contents.
func (g *gameLog) write(s string) {
	g.t.Helper()
	if err := os.WriteFile(g.path, []byte(s), 0o600); err != nil {
		g.t.Fatal(err)
	}
}

// append adds to the end of the log.
func (g *gameLog) append(s string) {
	g.t.Helper()
	f, err := os.OpenFile(g.path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		g.t.Fatal(err)
	}
	if _, err := f.WriteString(s); err != nil {
		g.t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		g.t.Fatal(err)
	}
}

func (g *gameLog) age(by time.Duration) {
	g.t.Helper()
	old := time.Now().Add(-by)
	if err := os.Chtimes(g.path, old, old); err != nil {
		g.t.Fatal(err)
	}
}

func TestFollowerIgnoresTheLogOfAnEarlierRun(t *testing.T) {
	g := newGameLog(t)
	g.write("[00:10:00] [main/INFO]: Loading Minecraft 1.20.6 with Fabric Loader 0.19.5\n" + render + "Stopping!\n")
	g.age(time.Hour)
	play := time.Now()
	f := newLogFollower(g.dir, SnapshotGameLog(g.dir), play)
	if phases, _ := f.poll(play); len(phases) != 0 || f.Fresh() {
		t.Fatalf("an old log is not this start's: %v", phases)
	}
}

func TestFollowerIgnoresAnOldRunStillWritingItsLog(t *testing.T) {
	// A game started from Prism itself is still running: the file changes
	// after Play, but its first line is the one the snapshot saw.
	g := newGameLog(t)
	g.write("[00:10:00] [main/INFO]: Loading Minecraft\n")
	play := time.Now().Add(-time.Second)
	before := SnapshotGameLog(g.dir)
	g.append(render + "Backend library: LWJGL\n")
	f := newLogFollower(g.dir, before, play)
	if phases, _ := f.poll(play); len(phases) != 0 || f.Fresh() {
		t.Fatalf("the snapshot's log is not this start's: %v", phases)
	}
}

func TestFollowerTakesANewLogAndReadsItAsItGrows(t *testing.T) {
	g := newGameLog(t)
	g.write("[00:10:00] [main/INFO]: Loading Minecraft\n")
	g.age(time.Hour)
	before := SnapshotGameLog(g.dir)
	play := time.Now().Add(-time.Second)
	f := newLogFollower(g.dir, before, play)
	now := play

	// The new run writes over the file: a different first line, written after Play.
	g.write("[00:30:00] [main/INFO]: Loading Minecraft\n")
	if phases, restarted := f.poll(now); !reflect.DeepEqual(phases, []string{"mods"}) || restarted || !f.Fresh() {
		t.Fatalf("got %v %v", phases, restarted)
	}
	// A line with no newline yet is kept until it is finished.
	g.append(render + "Backend library: LWJG")
	if phases, _ := f.poll(now); len(phases) != 0 {
		t.Fatalf("an unfinished line is not read: %v", phases)
	}
	g.append("L version 3\n" + render + "Reloading ResourceManager: vanilla\n")
	if phases, _ := f.poll(now); !reflect.DeepEqual(phases, []string{"window", "resources"}) {
		t.Fatalf("got %v", phases)
	}
	if phases, _ := f.poll(now); len(phases) != 0 {
		t.Fatalf("nothing new, nothing returned: %v", phases)
	}
}

func TestFollowerStartsOverWhenTheLogIsWrittenAgain(t *testing.T) {
	g := newGameLog(t)
	play := time.Now().Add(-time.Second)
	f := newLogFollower(g.dir, SnapshotGameLog(g.dir), play)
	g.write("[00:30:00] [main/INFO]: Loading Minecraft\n" + render + "Backend library: LWJGL\n" + render + "Stopping!\n")
	if phases, _ := f.poll(play); !reflect.DeepEqual(phases, []string{"mods", "window", "stopping"}) {
		t.Fatalf("got %v", phases)
	}
	// A second run: shorter, with a first line of its own.
	g.write("[00:40:00] [main/INFO]: Loading Minecraft\n")
	phases, restarted := f.poll(play)
	if !restarted || !reflect.DeepEqual(phases, []string{"mods"}) {
		t.Fatalf("a shorter file is a new run: %v %v", phases, restarted)
	}
	// Longer than the first run but with another first line: also new.
	g.write("[00:50:00] [main/INFO]: Loading Minecraft\n" + render + "Backend library: LWJGL\n" + render + "Reloading ResourceManager\n" + render + "Sound engine started\n")
	phases, restarted = f.poll(play)
	if !restarted || !reflect.DeepEqual(phases, []string{"mods", "window", "resources", "running"}) {
		t.Fatalf("a changed first line is a new run: %v %v", phases, restarted)
	}
}

func TestFollowerAlsoLooksInDotMinecraft(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".minecraft", "logs", "latest.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	play := time.Now().Add(-time.Second)
	f := newLogFollower(dir, SnapshotGameLog(dir), play)
	if err := os.WriteFile(path, []byte("first line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if phases, _ := f.poll(play); !reflect.DeepEqual(phases, []string{models.GamePhaseMods}) {
		t.Fatalf("got %v", phases)
	}
}

func TestFollowerWithNoInstanceFolderFollowsNothing(t *testing.T) {
	f := newLogFollower("", SnapshotGameLog(""), time.Now())
	if phases, restarted := f.poll(time.Now()); phases != nil || restarted || f.Fresh() {
		t.Fatal("no folder, no log")
	}
}

func TestFollowerTakesALongLineWithoutANewlineAsItStands(t *testing.T) {
	g := newGameLog(t)
	play := time.Now().Add(-time.Second)
	f := newLogFollower(g.dir, SnapshotGameLog(g.dir), play)
	long := make([]byte, partialMax+10)
	for i := range long {
		long[i] = 'x'
	}
	g.write("first\n" + string(long))
	if phases, _ := f.poll(play); !reflect.DeepEqual(phases, []string{"mods"}) {
		t.Fatalf("got %v", phases)
	}
	if len(f.partial) != 0 {
		t.Fatalf("an over-long line is not held: %d bytes", len(f.partial))
	}
}
