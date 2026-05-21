package cmds

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/store"
)

// UnlockResultMsg is returned by TryUnlockCmd with the outcome of a passphrase
// attempt.
type UnlockResultMsg struct {
	// OK is true when the passphrase verified against the KEK canary.
	OK  bool
	Err error
}

// TryUnlockCmd attempts to derive a KEK from passphrase and verify the stored
// canary. Returns UnlockResultMsg.
func TryUnlockCmd(st *store.Store, passphrase string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		row, err := st.Client.Setting.Get(ctx, 1)
		if err != nil {
			return UnlockResultMsg{Err: err}
		}
		if len(row.KekSalt) != 16 {
			return UnlockResultMsg{Err: errors.New("corrupt kek_salt")}
		}
		var salt [16]byte
		copy(salt[:], row.KekSalt)
		kek := crypto.DeriveFromPassphrase(passphrase, salt)
		if !kek.VerifyCanary(row.KekCanary) {
			kek.Wipe()
			return UnlockResultMsg{OK: false}
		}
		kek.Wipe() // verified; actual KEK construction happens in main after success
		return UnlockResultMsg{OK: true}
	}
}
