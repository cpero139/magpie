package cli

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/yetone/magpie/internal/appdir"
)

// The desktop app's log goes to a stderr its user can't reach (gui/trace.go,
// gateway/held_report.go say so): a magpie that froze, or whose plugin host
// died and said why on the log alone, left nothing behind to read. logToFile
// copies every line to a file in magpie's cache, for the runs that go on
// serving agents. The commands an agent starts for one session never get
// here, as they return before the start-up moves.
const (
	logName    = "magpie.log"
	logOldName = "magpie.log.old"
	// A day of the gateway's log runs to some 10KB, so this is about a
	// hundred days of it. Nothing trims it while magpie is up: the cap is
	// checked at a start, by the next one.
	logKeep = 1 << 20
)

// logToFile points the standard logger at the file, and at stderr after it.
// The file comes first because io.MultiWriter stops at the first writer that
// fails, and a desktop build's stderr is a handle that always fails: last, it
// would take the file's copy of every line with it. What the runtime prints
// beside the log package — a panic, a fatal error — goes to stderr's own file
// descriptor and is not copied here.
func logToFile() {
	dir := appdir.Cache()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return // nowhere to put the copy: stderr keeps what it has
	}
	path := filepath.Join(dir, logName)
	// one rotation, at a start. Nothing holds the file open between lines
	// (logTail), which is what lets it be renamed on Windows at all: usage
	// closes its log after every write for the same reason (usage.go). A
	// rename that fails, with another program reading the file, leaves this
	// one to grow on until the next start.
	if fi, err := os.Stat(path); err == nil && fi.Size() > logKeep {
		old := filepath.Join(dir, logOldName)
		_ = os.Remove(old)
		_ = os.Rename(path, old)
	}
	log.SetOutput(io.MultiWriter(logTail(path), os.Stderr))
	// the arguments are left out of it on purpose: a command takes a key as
	// one of them (magpie <agent> <field> <value>), and a file in the cache
	// is read by more hands, and sent by more tools, than a terminal is.
	// It says which run every line below belongs to, which is what it's for
	log.Printf("magpie %s (pid %d): the log is copied to %s", version, os.Getpid(), path)
}

// logTail writes a line as it comes, opening and closing the file for each
// one so it is never held open when the next start comes to rotate it away.
// A copy that failed isn't worth losing the terminal's own line over, so the
// errors are swallowed and the write always counts as done.
type logTail string

func (p logTail) Write(b []byte) (int, error) {
	f, err := os.OpenFile(string(p), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return len(b), nil
	}
	_, _ = f.Write(b)
	_ = f.Close()
	return len(b), nil
}
