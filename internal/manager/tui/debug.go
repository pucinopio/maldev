package tui

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// debugDumpEnv is the environment variable a developer sets to receive a
// chronological dump of every tea.Msg flowing through the root model. The
// value is the path to the log file; empty disables the dump entirely.
const debugDumpEnv = "LICENSE_MANAGER_TUI_DUMP"

var (
	debugDumpOnce sync.Once
	debugDumpW    io.Writer // nil until first init; stays nil when env unset
)

// dumpMsg writes a single tea.Msg to the debug log, or returns immediately
// when no log file is configured. Format: RFC3339Nano timestamp + %#v of msg.
// Safe for concurrent calls but designed for the single Update goroutine.
func dumpMsg(msg tea.Msg) {
	debugDumpOnce.Do(func() {
		path := os.Getenv(debugDumpEnv)
		if path == "" {
			return
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return
		}
		debugDumpW = f
	})
	if debugDumpW == nil {
		return
	}
	fmt.Fprintf(debugDumpW, "%s %#v\n", time.Now().Format(time.RFC3339Nano), msg)
}
