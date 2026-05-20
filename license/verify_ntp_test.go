package license

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"
)

// ntpEpochOffset is the number of seconds between the NTP epoch (1900-01-01)
// and the Unix epoch (1970-01-01), mirroring the unexported constant in license/ntp.
const ntpEpochOffset = 2208988800

// startFakeNTP launches a UDP stub that replies with the given offset from now.
func startFakeNTP(t *testing.T, offset time.Duration) string {
	t.Helper()
	udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = udp.Close() })
	go func() {
		buf := make([]byte, 48)
		for {
			_, raddr, err := udp.ReadFrom(buf)
			if err != nil {
				return
			}
			var resp [48]byte
			resp[0] = 0x1C // LI=0 VN=3 Mode=4 (server)
			secs := uint32(time.Now().Add(offset).Unix() + ntpEpochOffset)
			binary.BigEndian.PutUint32(resp[40:], secs)
			_, _ = udp.WriteTo(resp[:], raddr)
		}
	}()
	return udp.LocalAddr().String()
}

func TestVerifyNTPSoftDriftWarns(t *testing.T) {
	// Server clock is 10 minutes ahead — exceeds 1-minute maxDrift.
	addr := startFakeNTP(t, 10*time.Minute)
	data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(time.Hour)})

	v, err := Verify(data, trustedFor(pub, "k1"),
		WithNTPCheck(addr, time.Minute),
	)
	if err != nil {
		t.Fatalf("soft check must not fail Verify: %v", err)
	}
	found := false
	for _, w := range v.Warnings {
		if w == "ntp drift exceeds threshold" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected drift warning in %v", v.Warnings)
	}
}

func TestVerifyNTPStrictDriftRejects(t *testing.T) {
	// Server clock is 10 minutes ahead — strict mode must reject.
	addr := startFakeNTP(t, 10*time.Minute)
	data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(time.Hour)})

	_, err := Verify(data, trustedFor(pub, "k1"),
		WithNTPCheckStrict(addr, time.Minute),
	)
	if !errors.Is(err, ErrLicenseInvalid) {
		t.Fatalf("strict check must reject on drift, got %v", err)
	}
}

func TestVerifyNTPSoftQueryFailureWarns(t *testing.T) {
	// Port 1 is unreachable — network error must warn, not fail.
	data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(time.Hour)})

	v, err := Verify(data, trustedFor(pub, "k1"),
		WithNTPCheck("127.0.0.1:1", time.Minute),
	)
	if err != nil {
		t.Fatalf("soft check must not fail on unreachable server: %v", err)
	}
	found := false
	for _, w := range v.Warnings {
		if w == "ntp query failed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ntp query failed warning in %v", v.Warnings)
	}
}

func TestVerifyNTPStrictQueryFailureWarns(t *testing.T) {
	// Even strict mode must not fail when the NTP server is unreachable.
	data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(time.Hour)})

	v, err := Verify(data, trustedFor(pub, "k1"),
		WithNTPCheckStrict("127.0.0.1:1", time.Minute),
	)
	if err != nil {
		t.Fatalf("strict mode must not reject on network error: %v", err)
	}
	found := false
	for _, w := range v.Warnings {
		if w == "ntp query failed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ntp query failed warning in %v", v.Warnings)
	}
}

func TestVerifyNTPWithinDriftSucceeds(t *testing.T) {
	// Server clock matches local time — no warning, no error.
	addr := startFakeNTP(t, 0)
	data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(time.Hour)})

	v, err := Verify(data, trustedFor(pub, "k1"),
		WithNTPCheckStrict(addr, time.Minute),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, w := range v.Warnings {
		if w == "ntp drift exceeds threshold" {
			t.Fatalf("unexpected drift warning")
		}
	}
}
