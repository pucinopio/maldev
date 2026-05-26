//go:build tui_trace

package tui

// Trace hook enabled by the `tui_trace` build tag. When set, every call to
// traceMsg appends a JSONL line to the file pointed to by MALDEV_TUI_TRACE.
// Stub variant lives in trace_noop.go for the default build.
//
// Format per line:
//
//	{"ts":<unix-nanos>, "stage":<entry|key|mouse|...>, "msg_type":"tea.KeyMsg",
//	 "msg":"<%+v dump>", "active":"<view-id>", "overlay_depth":<n>}
//
// The runner under cmd/tui-verify reads this file to assert that a specific
// sequence of key/mouse inputs produces the expected sequence of msgs.

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	traceMu   sync.Mutex
	traceFile *os.File
	traceEnc  *json.Encoder
)

func init() {
	path := os.Getenv("MALDEV_TUI_TRACE")
	if path == "" {
		return
	}
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tui_trace: create %s: %v\n", path, err)
		return
	}
	traceFile = f
	traceEnc = json.NewEncoder(f)
}

// traceMsg writes one JSONL line per tea.Msg observed at the rootModel
// entry point. Stage is a short label ("entry", "key", "mouse", "overlay")
// describing which dispatcher saw the msg.
func traceMsg(stage string, msg interface{}) {
	if traceEnc == nil {
		return
	}
	traceMu.Lock()
	defer traceMu.Unlock()
	_ = traceEnc.Encode(map[string]any{
		"ts":       time.Now().UnixNano(),
		"stage":    stage,
		"msg_type": fmt.Sprintf("%T", msg),
		"msg":      fmt.Sprintf("%+v", msg),
	})
}
