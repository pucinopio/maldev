package heartbeat

import (
	"context"
	"time"
)

const tagHeartbeatV1 = "maldev-heartbeat-v1\x00"

type Request struct {
	LicenseID string `json:"lid"`
	Nonce     []byte `json:"n"`
}

type Reply struct {
	Version    int       `json:"v"`
	KeyID      string    `json:"kid"`
	LicenseID  string    `json:"lid"`
	Ok         bool      `json:"ok"`
	Reason     string    `json:"r,omitempty"`
	NonceEcho  []byte    `json:"n"`
	ServerTime time.Time `json:"st"`
	ValidUntil time.Time `json:"vu,omitempty"`
}

// Client is the interface implemented by heartbeat clients. Returns the parsed
// reply, the raw signed PEM bytes for caller-side signature verification, and
// any transport-level error.
type Client interface {
	Ping(ctx context.Context, licenseID string, nonce []byte) (Reply, []byte, error)
}
