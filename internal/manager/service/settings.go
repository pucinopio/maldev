package service

import (
	"context"

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
