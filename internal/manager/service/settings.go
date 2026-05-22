package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// SettingsService reads and mutates the singleton Setting and ServerConfig rows.
type SettingsService struct {
	store *store.Store
	kek   *crypto.KEK
}

func NewSettingsService(s *store.Store, k *crypto.KEK) *SettingsService {
	return &SettingsService{store: s, kek: k}
}

func (s *SettingsService) Get(ctx context.Context) (*ent.Setting, error) {
	return s.store.Client.Setting.Get(ctx, 1)
}

func (s *SettingsService) Update(ctx context.Context, mut func(*ent.SettingUpdateOne)) (*ent.Setting, error) {
	q := s.store.Client.Setting.UpdateOneID(1)
	mut(q)
	return q.Save(ctx)
}

func (s *SettingsService) GetServerConfig(ctx context.Context) (*ent.ServerConfig, error) {
	return s.store.Client.ServerConfig.Get(ctx, 1)
}

func (s *SettingsService) UpdateServerConfig(ctx context.Context, mut func(*ent.ServerConfigUpdateOne)) (*ent.ServerConfig, error) {
	q := s.store.Client.ServerConfig.UpdateOneID(1)
	mut(q)
	return q.Save(ctx)
}

// RegenerateAdminToken creates a fresh 32-byte random token for the named
// server, encrypts it with the KEK and persists it on ServerConfig. Returns
// the cleartext token so the caller can display it exactly once — the
// encrypted form is the only thing kept on disk.
//
// Supported server names: "revocation". Heartbeat and Probe don't have
// admin tokens (read-only / public endpoints).
func (s *SettingsService) RegenerateAdminToken(ctx context.Context, server string) (string, error) {
	if s.kek == nil {
		return "", fmt.Errorf("kek not initialised")
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	token := hex.EncodeToString(raw[:])
	enc, err := s.kek.Wrap([]byte(token))
	if err != nil {
		return "", fmt.Errorf("wrap: %w", err)
	}
	q := s.store.Client.ServerConfig.UpdateOneID(1)
	switch server {
	case "revocation":
		q.SetRevocationAdminTokenEnc(enc)
	default:
		return "", fmt.Errorf("server %q has no admin token", server)
	}
	if _, err := q.Save(ctx); err != nil {
		return "", fmt.Errorf("save: %w", err)
	}
	return token, nil
}
