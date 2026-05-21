package service

import (
	"context"
	"fmt"

	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// withTx runs fn within a transaction, rolling back on error or panic.
// Use this helper inside services instead of the manual Tx/Commit/Rollback
// dance so behaviour is consistent across every mutating method.
func withTx(ctx context.Context, s *store.Store, fn func(ctx context.Context, tx *ent.Tx) error) error {
	tx, err := s.Client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()
	if err := fn(ctx, tx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			return fmt.Errorf("%w (rollback: %v)", err, rerr)
		}
		return err
	}
	return tx.Commit()
}
