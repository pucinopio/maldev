package license

import (
	"errors"
	"testing"
	"time"
)

func TestBindPasswordStampsDefaultParams(t *testing.T) {
	b, err := BindPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if b.Params == nil {
		t.Fatal("Params should be stamped")
	}
	def := DefaultArgon2idParams()
	if b.Params.ArgonTime != def.ArgonTime ||
		b.Params.ArgonMemory != def.ArgonMemory ||
		b.Params.ArgonThreads != def.ArgonThreads ||
		b.Params.ArgonKeyLen != def.ArgonKeyLen {
		t.Fatalf("Params=%+v want %+v", *b.Params, def)
	}
}

func TestBindPasswordWithParamsRespectsOverrides(t *testing.T) {
	override := BindingParams{ArgonTime: 1, ArgonMemory: 32 * 1024, ArgonThreads: 2, ArgonKeyLen: 16}
	b, err := BindPasswordWithParams("hunter2", override)
	if err != nil {
		t.Fatal(err)
	}
	if b.Params == nil || *b.Params != override {
		t.Fatalf("Params=%+v want %+v", b.Params, override)
	}
	if len(b.Hash) != int(override.ArgonKeyLen) {
		t.Fatalf("hash len %d want %d", len(b.Hash), override.ArgonKeyLen)
	}
}

func TestVerifyHonoursBindingParams(t *testing.T) {
	// Cheap params at issue: t=1, m=8 MiB. Verify must use these stored
	// params, not the package defaults — otherwise the comparison fails.
	cheap := BindingParams{ArgonTime: 1, ArgonMemory: 8 * 1024, ArgonThreads: 2, ArgonKeyLen: 32}
	bp, err := BindPasswordWithParams("h", cheap)
	if err != nil {
		t.Fatal(err)
	}
	data, pub, _ := issueFor(t, IssueOptions{
		NotAfter: time.Now().Add(time.Hour),
		Bindings: []Binding{bp},
	})
	if _, err := Verify(data, trustedFor(pub, "k1"), WithPassword("h")); err != nil {
		t.Fatalf("verify with stamped params failed: %v", err)
	}
	if _, err := Verify(data, trustedFor(pub, "k1"), WithPassword("wrong")); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("wrong password accepted")
	}
}
