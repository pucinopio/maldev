package testutil

import (
	"testing"

	wsyscall "github.com/oioio-space/maldev/win/syscall"
)

// CallerMethod bundles a Caller with its method enum and a human-readable name
// for table-driven tests. The Method field allows tests to pass it directly to
// WindowsConfig.SyscallMethod without maintaining a separate name→method map.
type CallerMethod struct {
	Name   string
	Method wsyscall.Method
	Caller *wsyscall.Caller
}

// CallerMethods returns the 4 standard Caller configurations for matrix testing.
// Each technique that accepts *wsyscall.Caller should be tested with all 4.
func CallerMethods(t *testing.T) []CallerMethod {
	t.Helper()
	chain := wsyscall.Chain(wsyscall.NewHellsGate(), wsyscall.NewHalosGate())
	return []CallerMethod{
		{"WinAPI", wsyscall.MethodWinAPI, nil},
		{"NativeAPI", wsyscall.MethodNativeAPI, wsyscall.New(wsyscall.MethodNativeAPI, nil)},
		{"Direct", wsyscall.MethodDirect, wsyscall.New(wsyscall.MethodDirect, chain)},
		{"Indirect", wsyscall.MethodIndirect, wsyscall.New(wsyscall.MethodIndirect, chain)},
	}
}

// CallerResolverMatrix returns every meaningful (Method, SSN-resolver)
// combination wsyscall exposes — 14 rows in total:
//
//   - 2 paths with no resolver: WinAPI + NativeAPI
//   - 3 syscall methods × 4 resolvers = 12: Direct / Indirect /
//     IndirectAsm × HellsGate / HalosGate / Tartarus / HashGate
//
// Each row carries a freshly built *wsyscall.Caller (so concurrent
// resolver state is per-row, not shared) and is automatically Closed
// via t.Cleanup when the test ends — sub-tests don't need to defer
// Close themselves. Names are slash-joined ("Direct/HellsGate") so
// they show up as readable t.Run paths in the failing-test list.
//
// Tests that exercise a primitive routed through *wsyscall.Caller —
// runtime/bof.SetCaller, the cross-process inject primitives,
// etc. — should run their assertions over the full matrix so a
// regression in any single (Method, Resolver) cell shows up before
// it can hide behind the green default-path case.
func CallerResolverMatrix(t *testing.T) []CallerMethod {
	t.Helper()
	resolvers := []struct {
		name string
		make func() wsyscall.SSNResolver
	}{
		{"HellsGate", func() wsyscall.SSNResolver { return wsyscall.NewHellsGate() }},
		{"HalosGate", func() wsyscall.SSNResolver { return wsyscall.NewHalosGate() }},
		{"Tartarus", func() wsyscall.SSNResolver { return wsyscall.NewTartarus() }},
		{"HashGate", func() wsyscall.SSNResolver { return wsyscall.NewHashGate() }},
	}

	out := []CallerMethod{
		{"WinAPI/nil_resolver", wsyscall.MethodWinAPI, wsyscall.New(wsyscall.MethodWinAPI, nil)},
		{"NativeAPI/nil_resolver", wsyscall.MethodNativeAPI, wsyscall.New(wsyscall.MethodNativeAPI, nil)},
	}
	for _, m := range []struct {
		name   string
		method wsyscall.Method
	}{
		{"Direct", wsyscall.MethodDirect},
		{"Indirect", wsyscall.MethodIndirect},
		{"IndirectAsm", wsyscall.MethodIndirectAsm},
	} {
		for _, r := range resolvers {
			out = append(out, CallerMethod{
				Name:   m.name + "/" + r.name,
				Method: m.method,
				Caller: wsyscall.New(m.method, r.make()),
			})
		}
	}
	for _, cm := range out {
		c := cm.Caller
		t.Cleanup(func() { c.Close() })
	}
	return out
}
