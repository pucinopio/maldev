package license

import (
	"errors"
	"testing"
	"time"
)

func TestBindMachineIDsMatchAny(t *testing.T) {
	data, pub, _ := issueFor(t, IssueOptions{
		NotAfter: time.Now().Add(time.Hour),
		Bindings: []Binding{BindMachineIDs("aaa", "bbb")},
	})
	if _, err := Verify(data, trustedFor(pub, "k1"), WithMachineID([]byte("bbb"))); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(data, trustedFor(pub, "k1"), WithMachineID([]byte("zzz"))); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("expected reject")
	}
	if _, err := Verify(data, trustedFor(pub, "k1")); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("missing evidence should reject")
	}
}

func TestBindPasswordArgon2id(t *testing.T) {
	bp, err := BindPassword("s3cr3t")
	if err != nil {
		t.Fatal(err)
	}
	if len(bp.Salt) != 16 || len(bp.Hash) == 0 {
		t.Fatal("salt/hash not populated")
	}
	data, pub, _ := issueFor(t, IssueOptions{
		NotAfter: time.Now().Add(time.Hour),
		Bindings: []Binding{bp},
	})
	if _, err := Verify(data, trustedFor(pub, "k1"), WithPassword("s3cr3t")); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(data, trustedFor(pub, "k1"), WithPassword("wrong")); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("wrong password accepted")
	}
}

func TestBindCustomMatch(t *testing.T) {
	data, pub, _ := issueFor(t, IssueOptions{
		NotAfter: time.Now().Add(time.Hour),
		Bindings: []Binding{BindCustom("project", "WRAITH")},
	})
	if _, err := Verify(data, trustedFor(pub, "k1"), WithCustom("project", "WRAITH")); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(data, trustedFor(pub, "k1"), WithCustom("project", "OTHER")); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("custom mismatch accepted")
	}
}

func TestRegisterVerifierExtensibility(t *testing.T) {
	RegisterVerifier("ip", func(b Binding, s *verifyState) bool {
		val := s.customVals["ip"]
		return contains(b.Value, val)
	})
	t.Cleanup(func() {
		verifierMu.Lock()
		delete(globalVerifiers, "ip")
		verifierMu.Unlock()
	})

	data, pub, _ := issueFor(t, IssueOptions{
		NotAfter: time.Now().Add(time.Hour),
		Bindings: []Binding{{Type: "custom:ip", Value: []string{"10.0.0.1", "10.0.0.2"}}},
	})
	if _, err := Verify(data, trustedFor(pub, "k1"), WithCustom("ip", "10.0.0.2")); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(data, trustedFor(pub, "k1"), WithCustom("ip", "10.0.0.9")); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("expected reject")
	}
}
