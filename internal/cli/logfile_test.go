package cli

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/appdir"
)

// isolatedCache points magpie's cache at a folder of this test's own and
// returns it. Without it the tests would write, and rotate, the real log of
// whoever runs them.
func isolatedCache(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)
	dir := appdir.Cache()
	// a portable magpie answers from beside its own program, whatever
	// XDG_CACHE_HOME says: the test would be writing there
	if !strings.HasPrefix(dir, filepath.Clean(root)) {
		t.Fatalf("magpie's cache is %s, outside this test's %s", dir, root)
	}
	return dir
}

// The copy of the log reaches magpie's cache, and a stderr that always fails
// — what a desktop build has — doesn't take it away: the file is the first of
// the two writers io.MultiWriter runs, and it stops at the first one that
// fails.
func TestLogFileCopiesPastAFailingStderr(t *testing.T) {
	wasLog, wasStderr := log.Writer(), os.Stderr
	t.Cleanup(func() { log.SetOutput(wasLog); os.Stderr = wasStderr })
	dir := isolatedCache(t)

	closed, err := os.Create(filepath.Join(t.TempDir(), "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stderr = closed // every write to it now fails, as one to no console does

	logToFile()
	log.Print("the plugin host went away")

	b, err := os.ReadFile(filepath.Join(dir, logName))
	if err != nil {
		t.Fatalf("the line went nowhere: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, "the plugin host went away") {
		t.Fatalf("the copy is missing the line: %q", got)
	}
	// the banner names the run every line below it belongs to
	if !strings.Contains(got, "pid") || !strings.Contains(got, version) {
		t.Fatalf("the start left no banner of its own: %q", got)
	}
	// and it says nothing else: a command takes a key as one of its arguments
	for _, arg := range os.Args[1:] {
		if len(arg) > 3 && strings.Contains(got, arg) {
			t.Fatalf("an argument of this test run was copied to the log: %q", got)
		}
	}
}

// A start rotates a log past the cap aside, keeping the one generation, and
// leaves a short one alone.
func TestLogFileRotatesAtTheCap(t *testing.T) {
	wasLog, wasStderr := log.Writer(), os.Stderr
	t.Cleanup(func() { log.SetOutput(wasLog); os.Stderr = wasStderr })
	dir := isolatedCache(t)
	path := filepath.Join(dir, logName)
	old := filepath.Join(dir, logOldName)
	// seeded before logToFile runs, so the folder is not there yet
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("an older run\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	logToFile()
	if _, err := os.Stat(old); err == nil {
		t.Fatal("a log under the cap was rotated away")
	}

	if err := os.WriteFile(path, make([]byte, logKeep+1), 0o600); err != nil {
		t.Fatal(err)
	}
	logToFile()
	fi, err := os.Stat(old)
	if err != nil {
		t.Fatalf("the past-cap log was not kept: %v", err)
	}
	if fi.Size() != logKeep+1 {
		t.Fatalf("the kept generation is %d bytes, not the one rotated", fi.Size())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 512 {
		t.Fatalf("the new log starts with %d bytes, not just its banner", len(b))
	}
}
