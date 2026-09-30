// Guards against silent coverage regressions in backend/services, the package
// that carries every side effect (agent_docs/CLAUDE.md, "Where things live").
// Same shape as Konnekt's scripts/coverage-floor and as the frontend floor in
// vite.config.ts: a script owns the threshold, and both .claude/suite.json and
// CI call it by name, so the check runner enforces it too.
//
// It reads the figure `go test -cover` prints rather than testing.Coverage() in
// a TestMain: Konnekt measured the two disagreeing on the same run, and a floor
// has to be measured the way the number people quote is measured.
//
// Run: go run ./scripts/coverage-floor
package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
)

// Scoped rather than ./... on purpose: the root package, backend/models and
// backend/design carry no logic worth a floor, and would dilute the figure.
const targetPackage = "./backend/services/"

// Floor = the last measured figure minus a little headroom, so an unrelated
// refactor does not redden the build. It is a ratchet: raise it when coverage
// rises, never lower it to make a red build green.
//
// Measure it where CI judges it, the Windows backend job: the build-tagged
// code differs by platform, and the same tests read 79.3% on Linux against
// 76.7% on Windows.
//
//	76.7% -> floor 74.5  the services as of the managed Prism (#26), on Windows
//
// Coverage is a proxy, not the goal. A test that would have caught a real bug is
// worth more than one that only moves this number.
const floorPercent = 74.5

// Matches the tail of `go test -cover` output: "coverage: 79.3% of statements".
var reCoverage = regexp.MustCompile(`coverage:\s+([0-9.]+)%\s+of\s+statements`)

func main() {
	out, err := exec.Command("go", "test", "-cover", targetPackage).CombinedOutput()
	// Print the test output either way: when tests fail, that is the thing worth
	// reading, not the coverage number.
	if _, werr := os.Stdout.Write(out); werr != nil {
		fmt.Fprintf(os.Stderr, "coverage floor: writing test output: %v\n", werr)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "coverage floor: tests failed, coverage not judged")
		os.Exit(1)
	}

	match := reCoverage.FindStringSubmatch(string(out))
	if match == nil {
		// A check that could not run is a failure to report, not a silent pass.
		fmt.Fprintf(os.Stderr, "coverage floor: no coverage line in the output of `go test -cover %s`\n", targetPackage)
		os.Exit(1)
	}

	got, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "coverage floor: could not parse %q as a percentage: %v\n", match[1], err)
		os.Exit(1)
	}

	if got < floorPercent {
		fmt.Fprintf(os.Stderr, "\ncoverage floor: %.1f%% is below the required %.1f%%\n", got, floorPercent)
		fmt.Fprintln(os.Stderr, "Add tests for what you changed, or justify moving the floor.")
		os.Exit(1)
	}

	fmt.Printf("coverage floor: %.1f%% meets the %.1f%% minimum\n", got, floorPercent)
}
