package server

import (
	"crypto/ed25519"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/oioio-space/maldev/license/heartbeat"
)

type HeartbeatOptions struct {
	PrivateKey ed25519.PrivateKey
	KeyID      string
	Store      LicenseStore
	ValidFor   time.Duration
	Logger     *slog.Logger
}

type hbHandler struct {
	opts HeartbeatOptions
	log  *slog.Logger
}

func NewHeartbeatHandler(opts HeartbeatOptions) http.Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.ValidFor == 0 {
		opts.ValidFor = time.Hour
	}
	return &hbHandler{opts: opts, log: opts.Logger}
}

func (h *hbHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req heartbeat.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	status, err := h.opts.Store.Status(r.Context(), req.LicenseID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reply := heartbeat.Reply{
		Version: 1, KeyID: h.opts.KeyID, LicenseID: req.LicenseID,
		NonceEcho: req.Nonce, ServerTime: time.Now().UTC(),
	}
	switch status {
	case StatusActive:
		reply.Ok = true
		reply.ValidUntil = time.Now().Add(h.opts.ValidFor).UTC()
	case StatusRevoked:
		reply.Reason = "revoked"
	case StatusExpired:
		reply.Reason = "expired"
	default:
		reply.Reason = "unknown"
	}
	signed, err := heartbeat.SignReply(reply, h.opts.PrivateKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	_, _ = w.Write(signed)
}
