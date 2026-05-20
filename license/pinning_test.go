package license

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestHashFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "bin")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("hello")
	_ = f.Close()
	h, err := HashFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 64 {
		t.Fatalf("got %d hex chars", len(h))
	}
}

func TestVerifyIdentityPinningMatches(t *testing.T) {
	idBytes := []byte("identity-payload-32-bytes-long..")
	data, pub, _ := issueFor(t, IssueOptions{
		NotAfter:       time.Now().Add(time.Hour),
		IdentitySHA256: HashIdentity(idBytes),
	})
	if _, err := Verify(data, trustedFor(pub, "k1"),
		WithBinaryPinning(), WithIdentityBytes(idBytes)); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyIdentityPinningMismatch(t *testing.T) {
	data, pub, _ := issueFor(t, IssueOptions{
		NotAfter:       time.Now().Add(time.Hour),
		IdentitySHA256: HashIdentity([]byte("AAAA")),
	})
	if _, err := Verify(data, trustedFor(pub, "k1"),
		WithBinaryPinning(), WithIdentityBytes([]byte("BBBB"))); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("mismatch accepted")
	}
}
