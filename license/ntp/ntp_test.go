package ntp

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestQueryAgainstStubServer(t *testing.T) {
	addr := startStubNTP(t)
	got, _, err := Query(addr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got.Year() < 2000 {
		t.Fatalf("got=%v", got)
	}
}

func startStubNTP(t *testing.T) string {
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
			secs := uint32(time.Now().Unix() + ntpEpoch)
			binary.BigEndian.PutUint32(resp[40:], secs)
			_, _ = udp.WriteTo(resp[:], raddr)
		}
	}()
	return udp.LocalAddr().String()
}
