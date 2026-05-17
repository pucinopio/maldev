//go:build windows

package bof

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExecuteStream_RoundsTripChunks drives a real BOF through
// ExecuteStream and verifies that:
//   - the chunks flowing through the channel reconstruct the full
//     output buffer the sync Execute would have returned,
//   - the channel is closed once Execute returns,
//   - the returned full slice matches the joined chunks (within the
//     drop tolerance — the test channel is buffered so no chunks
//     should be dropped in practice).
func TestExecuteStream_RoundsTripChunks(t *testing.T) {
	bytes, err := os.ReadFile(filepath.Join("testdata", "realworld_calls.o"))
	require.NoError(t, err)

	b, err := Load(bytes)
	require.NoError(t, err)
	b.SetSpawnTo(`C:\Windows\System32\notepad.exe`)

	ch := make(chan []byte, 64)
	var streamed []byte
	doneStream := make(chan struct{})
	go func() {
		for chunk := range ch {
			streamed = append(streamed, chunk...)
		}
		close(doneStream)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	full, err := b.ExecuteStream(ctx, nil, ch)
	require.NoError(t, err)
	<-doneStream // wait for consumer to drain
	require.NotEmpty(t, streamed, "stream channel must have received chunks")
	require.NotEmpty(t, full, "sync return value must mirror streamed bytes")
	assert.Equal(t, string(full), string(streamed),
		"stream chunks concatenated must equal the sync full buffer")
	assert.True(t, strings.Contains(string(streamed), "host="),
		"streamed output should contain the BOF's first printf line")
}

// TestExecuteStream_NilChannelFallsBackToExecute asserts the
// documented behaviour: passing nil for `out` is equivalent to
// calling Execute directly.
func TestExecuteStream_NilChannelFallsBackToExecute(t *testing.T) {
	bytes, err := os.ReadFile(filepath.Join("testdata", "hello_beacon.o"))
	require.NoError(t, err)
	b, err := Load(bytes)
	require.NoError(t, err)
	full, err := b.ExecuteStream(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, full)
}
