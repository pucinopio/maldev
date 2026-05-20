package store

import (
	"context"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// EnsureSingletons makes sure the Setting (id=1) and ServerConfig (id=1)
// rows exist. Idempotent — safe to call on every boot.
func (s *Store) EnsureSingletons(ctx context.Context, kekSalt, kekCanary []byte) error {
	_, err := s.Client.Setting.Get(ctx, 1)
	if ent.IsNotFound(err) {
		if _, err := s.Client.Setting.Create().
			SetID(1).
			SetKekSalt(kekSalt).
			SetKekCanary(kekCanary).
			Save(ctx); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	_, err = s.Client.ServerConfig.Get(ctx, 1)
	if ent.IsNotFound(err) {
		if _, err := s.Client.ServerConfig.Create().SetID(1).Save(ctx); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}
