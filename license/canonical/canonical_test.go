package canonical

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMarshalSortsKeys(t *testing.T) {
	in := map[string]any{"b": 2, "a": 1, "c": map[string]any{"y": "Y", "x": "X"}}
	out, err := Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":1,"b":2,"c":{"x":"X","y":"Y"}}`
	if string(out) != want {
		t.Fatalf("got %s want %s", out, want)
	}
}

func TestMarshalTimeRFC3339Nano(t *testing.T) {
	in := map[string]any{"t": time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)}
	out, err := Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"t":"2026-05-20T10:00:00Z"`) {
		t.Fatalf("got %s", out)
	}
}

func TestMarshalDeterministicRoundtrip(t *testing.T) {
	type X struct {
		B int            `json:"b"`
		A string         `json:"a"`
		M map[string]int `json:"m"`
	}
	v := X{B: 2, A: "hi", M: map[string]int{"y": 9, "x": 8}}
	a, _ := Marshal(v)
	b, _ := Marshal(v)
	if string(a) != string(b) {
		t.Fatalf("non-deterministic: %s vs %s", a, b)
	}
	var back X
	if err := json.Unmarshal(a, &back); err != nil {
		t.Fatal(err)
	}
	if back.A != v.A || back.B != v.B || len(back.M) != len(v.M) {
		t.Fatalf("roundtrip mismatch: %+v", back)
	}
}

func TestMarshalNoHTMLEscape(t *testing.T) {
	in := map[string]string{"k": "<a>&b"}
	out, _ := Marshal(in)
	if !strings.Contains(string(out), "<a>&b") {
		t.Fatalf("HTML was escaped: %s", out)
	}
}
