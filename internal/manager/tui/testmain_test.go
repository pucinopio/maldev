package tui_test

import (
	"os"
	"testing"
	"time"

	"github.com/oioio-space/maldev/internal/manager/tui"
)

// fixedTestTime is the frozen clock used by all snapshot tests so the title bar
// timestamp is deterministic across runs and machines.
var fixedTestTime = time.Date(2026, 5, 21, 7, 54, 12, 0, time.UTC)

func TestMain(m *testing.M) {
	// Freeze the title-bar clock for the entire test binary so golden files
	// contain a stable timestamp rather than time.Now().
	restore := tui.SetTitleBarClock(func() time.Time { return fixedTestTime })
	code := m.Run()
	restore()
	os.Exit(code)
}
