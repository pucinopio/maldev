// Package ntp performs a minimal unauthenticated SNTPv4 query suitable as a
// soft cross-check of the local clock. Not authenticated — must not be the
// sole guard against tamper.
package ntp

import (
	"encoding/binary"
	"errors"
	"net"
	"time"
)

const ntpEpoch = 2208988800 // seconds between 1900-01-01 and 1970-01-01

// Query asks server for its current time. Returns the server's stated time
// and the round-trip drift estimate ((reply - sent) / 2).
func Query(server string, timeout time.Duration) (time.Time, time.Duration, error) {
	conn, err := net.DialTimeout("udp", server, timeout)
	if err != nil {
		return time.Time{}, 0, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	req := make([]byte, 48)
	req[0] = 0x1B // LI=0 VN=3 Mode=3 (client)
	sent := time.Now()
	if _, err := conn.Write(req); err != nil {
		return time.Time{}, 0, err
	}

	resp := make([]byte, 48)
	n, err := conn.Read(resp)
	if err != nil {
		return time.Time{}, 0, err
	}
	if n < 48 {
		return time.Time{}, 0, errors.New("ntp: short reply")
	}
	received := time.Now()

	secs := binary.BigEndian.Uint32(resp[40:44])
	frac := binary.BigEndian.Uint32(resp[44:48])
	if secs == 0 {
		return time.Time{}, 0, errors.New("ntp: zero timestamp")
	}
	unixSecs := int64(secs) - ntpEpoch
	nsec := int64(float64(frac) / (1 << 32) * 1e9)
	serverT := time.Unix(unixSecs, nsec).UTC()
	drift := received.Sub(sent) / 2
	return serverT, drift, nil
}
