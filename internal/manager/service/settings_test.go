package service

import (
	"context"
	"testing"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

func TestSettingsGetUpdate(t *testing.T) {
	s := newTestStore(t)
	settings := NewSettingsService(s, nil)
	ctx := context.Background()

	row, err := settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row.DefaultTTLSeconds == 0 {
		t.Fatal("default TTL should be set by schema default")
	}
	_, err = settings.Update(ctx, func(u *ent.SettingUpdateOne) {
		u.SetOperatorName("alice")
	})
	if err != nil {
		t.Fatal(err)
	}
	row2, _ := settings.Get(ctx)
	if row2.OperatorName != "alice" {
		t.Fatalf("operator_name=%q", row2.OperatorName)
	}
}
