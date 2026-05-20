package server

import (
	"crypto/ed25519"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/oioio-space/maldev/license/revoke"
)

type RevocationOptions struct {
	PrivateKey ed25519.PrivateKey
	KeyID      string
	Store      RevocationStore
	ValidFor   time.Duration
	AdminToken string
	Logger     *slog.Logger
}

type revHandler struct {
	opts RevocationOptions
	log  *slog.Logger
}

func NewRevocationHandler(opts RevocationOptions) http.Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.ValidFor == 0 {
		opts.ValidFor = 7 * 24 * time.Hour
	}
	return &revHandler{opts: opts, log: opts.Logger}
}

func (h *revHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.serveList(w, r)
	case http.MethodPost:
		h.serveAdmin(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *revHandler) serveList(w http.ResponseWriter, r *http.Request) {
	list, err := h.opts.Store.Load(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	list.Version = 1
	list.KeyID = h.opts.KeyID
	list.Sequence++ // monotonic per serve
	now := time.Now().UTC()
	list.IssuedAt = now
	list.ExpiresAt = now.Add(h.opts.ValidFor)
	list.ServerTime = now
	signed, err := revoke.Sign(list, h.opts.PrivateKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.opts.Store.Save(r.Context(), list); err != nil {
		h.log.Warn("revocation store save failed", "err", err)
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	_, _ = w.Write(signed)
}

func (h *revHandler) serveAdmin(w http.ResponseWriter, r *http.Request) {
	if h.opts.AdminToken == "" {
		http.Error(w, "read-only", http.StatusForbidden)
		return
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") || auth[len("Bearer "):] != h.opts.AdminToken {
		http.Error(w, "unauthorised", http.StatusUnauthorized)
		return
	}
	var req struct {
		Add    []string `json:"add"`
		Remove []string `json:"remove"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	list, err := h.opts.Store.Load(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	set := map[string]struct{}{}
	for _, id := range list.Revoked {
		set[id] = struct{}{}
	}
	for _, id := range req.Add {
		set[id] = struct{}{}
	}
	for _, id := range req.Remove {
		delete(set, id)
	}
	list.Revoked = list.Revoked[:0]
	for id := range set {
		list.Revoked = append(list.Revoked, id)
	}
	if err := h.opts.Store.Save(r.Context(), list); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
