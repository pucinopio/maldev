package license

import (
	"errors"
	"testing"
	"time"

	"github.com/oioio-space/maldev/license/totp"
)

func TestBindTOTPAcceptsCurrentCode(t *testing.T) {
	secret, err := totp.NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	data, pub, _ := issueFor(t, IssueOptions{
		NotAfter: time.Now().Add(time.Hour),
		Bindings: []Binding{BindTOTP(secret)},
	})

	code, err := totp.Code(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(data, trustedFor(pub, "k1"), WithTOTPCode(code)); err != nil {
		t.Fatalf("current code rejected: %v", err)
	}
}

func TestBindTOTPRejectsWrongCode(t *testing.T) {
	secret, _ := totp.NewSecret()
	data, pub, _ := issueFor(t, IssueOptions{
		NotAfter: time.Now().Add(time.Hour),
		Bindings: []Binding{BindTOTP(secret)},
	})
	if _, err := Verify(data, trustedFor(pub, "k1"), WithTOTPCode("000000")); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("expected reject on wrong code")
	}
}

func TestBindTOTPRejectsMissingEvidence(t *testing.T) {
	secret, _ := totp.NewSecret()
	data, pub, _ := issueFor(t, IssueOptions{
		NotAfter: time.Now().Add(time.Hour),
		Bindings: []Binding{BindTOTP(secret)},
	})
	if _, err := Verify(data, trustedFor(pub, "k1")); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("expected reject when WithTOTPCode is omitted")
	}
}
