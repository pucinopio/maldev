package license

import (
	"encoding/json"
	"testing"
	"time"
)

func startsWith(b, p []byte) bool {
	if len(b) < len(p) {
		return false
	}
	for i := range p {
		if b[i] != p[i] {
			return false
		}
	}
	return true
}

func TestIssueProducesPEM(t *testing.T) {
	_, priv, _ := GenerateKey()
	data, err := Issue(IssueOptions{
		PrivateKey: priv,
		KeyID:      "k1",
		Subject:    "alice@example.com",
		NotAfter:   time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !startsWith(data, []byte("-----BEGIN MALDEV LICENSE-----")) {
		t.Fatalf("not a MALDEV LICENSE PEM: %s", data)
	}
}

func TestIssueRejectsMissingKey(t *testing.T) {
	if _, err := Issue(IssueOptions{Subject: "x"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestIssueRejectsMissingSubject(t *testing.T) {
	_, priv, _ := GenerateKey()
	if _, err := Issue(IssueOptions{PrivateKey: priv}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewOneLiner(t *testing.T) {
	_, priv, _ := GenerateKey()
	data, err := New(priv, "alice", 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	lic, err := Inspect(data)
	if err != nil {
		t.Fatal(err)
	}
	if lic.Subject != "alice" {
		t.Fatalf("subject=%q", lic.Subject)
	}
	if lic.NotAfter.Before(time.Now().Add(6 * 24 * time.Hour)) {
		t.Fatalf("expiry too short: %v", lic.NotAfter)
	}
	if len(lic.Payload) > 0 {
		var raw any
		if err := json.Unmarshal(lic.Payload, &raw); err != nil {
			t.Fatal(err)
		}
	}
}
