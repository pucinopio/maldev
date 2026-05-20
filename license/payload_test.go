package license

import (
	"crypto/ed25519"
	"errors"
	"slices"
	"testing"
	"time"
)

type testPayload struct {
	Endpoint string   `json:"endpoint"`
	Tier     int      `json:"tier"`
	Tags     []string `json:"tags,omitempty"`
}

func issueWithPayload(t *testing.T, v any) ([]byte, ed25519.PublicKey) {
	t.Helper()
	pub, priv, _ := GenerateKey()
	raw, err := MarshalPayload(v)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Issue(IssueOptions{
		PrivateKey: priv, KeyID: "k1", Subject: "x",
		NotAfter: time.Now().Add(time.Hour),
		Payload:  raw,
	})
	if err != nil {
		t.Fatal(err)
	}
	return data, pub
}

func TestPayloadRoundTripDecode(t *testing.T) {
	in := testPayload{Endpoint: "https://c2.example.com", Tier: 3, Tags: []string{"redteam", "research"}}
	data, pub := issueWithPayload(t, in)

	v, err := Verify(data, trustedFor(pub, "k1"))
	if err != nil {
		t.Fatal(err)
	}
	var out testPayload
	if err := v.Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Endpoint != in.Endpoint || out.Tier != in.Tier || !slices.Equal(out.Tags, in.Tags) {
		t.Fatalf("roundtrip mismatch: %+v vs %+v", out, in)
	}
}

func TestPayloadAsGeneric(t *testing.T) {
	in := testPayload{Endpoint: "wss://c2.example.com", Tier: 1}
	data, pub := issueWithPayload(t, in)

	v, err := Verify(data, trustedFor(pub, "k1"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := PayloadAs[testPayload](v)
	if err != nil {
		t.Fatal(err)
	}
	if out.Endpoint != in.Endpoint || out.Tier != in.Tier {
		t.Fatalf("got %+v", out)
	}
}

func TestPayloadNoPayloadSentinel(t *testing.T) {
	pub, priv, _ := GenerateKey()
	data, err := Issue(IssueOptions{
		PrivateKey: priv, KeyID: "k1", Subject: "x",
		NotAfter: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Verify(data, trustedFor(pub, "k1"))
	if err != nil {
		t.Fatal(err)
	}
	var anything any
	if err := v.Decode(&anything); !errors.Is(err, ErrNoPayload) {
		t.Fatalf("Decode err=%v want ErrNoPayload", err)
	}
	if _, err := PayloadAs[testPayload](v); !errors.Is(err, ErrNoPayload) {
		t.Fatalf("PayloadAs err=%v want ErrNoPayload", err)
	}
}

func TestPayloadDecodeWrongShape(t *testing.T) {
	in := testPayload{Endpoint: "x", Tier: 1}
	data, pub := issueWithPayload(t, in)
	v, err := Verify(data, trustedFor(pub, "k1"))
	if err != nil {
		t.Fatal(err)
	}
	type wrong struct {
		// Tier is a slice instead of int — type mismatch should fail.
		Tier []string `json:"tier"`
	}
	var out wrong
	if err := v.Decode(&out); err == nil {
		t.Fatal("expected JSON shape error")
	}
}

func TestMarshalPayloadAcceptsPrimitives(t *testing.T) {
	// Maps, slices, primitives all work.
	cases := []any{
		map[string]int{"a": 1, "b": 2},
		[]string{"x", "y"},
		"a string",
		42,
		true,
	}
	for _, c := range cases {
		if _, err := MarshalPayload(c); err != nil {
			t.Fatalf("MarshalPayload(%v): %v", c, err)
		}
	}
}
