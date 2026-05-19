//go:build windows

package bof

import (
	"testing"
	"time"
)

// BenchmarkExecute_Inline measures the steady-state cost of an
// inline Execute call on a prepared, hot-path *BOF (hello_beacon
// is the cheapest fixture — no args parsing, one BeaconPrintf).
//
// Once a *BOF has been prepared, Execute should be dominated by the
// entry-thunk call + Beacon-API stub dispatch; any regression that
// re-runs prepare or re-VirtualAllocs shows up here as a 10×+ jump.
func BenchmarkExecute_Inline(b *testing.B) {
	data := loadTestBOF(b, "hello_beacon.o")
	bof, err := Load(data)
	if err != nil {
		b.Fatalf("Load: %v", err)
	}
	defer bof.Close()
	// Warm prepare so the bench measures the steady state, not the
	// one-shot loader cost.
	if _, err := bof.Execute(nil); err != nil {
		b.Fatalf("warm Execute: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := bof.Execute(nil); err != nil {
			b.Fatalf("Execute: %v", err)
		}
	}
}

// BenchmarkExecute_Sacrificial pins the per-call cost of the
// sacrificial-thread path: shared trampoline + sacFrame alloc +
// CreateThread / WaitForSingleObject. Pre-shared-trampoline this
// would have been ~3 syscalls + a 4 KB VirtualAlloc heavier per
// iteration. Numbers here are the proof point.
func BenchmarkExecute_Sacrificial(b *testing.B) {
	data := loadTestBOF(b, "hello_beacon.o")
	bof, err := Load(data)
	if err != nil {
		b.Fatalf("Load: %v", err)
	}
	defer bof.Close()
	if err := bof.SetSacrificialThread(5 * time.Second); err != nil {
		b.Fatalf("SetSacrificialThread: %v", err)
	}
	if _, err := bof.Execute(nil); err != nil {
		b.Fatalf("warm Execute: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := bof.Execute(nil); err != nil {
			b.Fatalf("Execute: %v", err)
		}
	}
}

// BenchmarkArgs_Pack measures the chunk-list pack — relevant when a
// caller packs a multi-MB AddBytes blob (runtime/pe wraps a full PE
// here). Reports allocations so a regression that copies the source
// blob inside Pack flags as 2x.
func BenchmarkArgs_Pack(b *testing.B) {
	payload := make([]byte, 1<<20) // 1 MB
	for i := range payload {
		payload[i] = byte(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a := NewArgs()
		a.AddInt(42)
		a.AddString("target.exe")
		a.AddBytes(payload)
		_ = a.Pack()
	}
}

