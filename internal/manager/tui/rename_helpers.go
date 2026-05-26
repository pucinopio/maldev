package tui

// Shared helpers for the rename workflow across screens. Issuers, Recipients,
// and Identities each expose a `case "e":` that pushes an input overlay
// pre-filled with the current row name; on confirm the result is currently a
// stub OK overlay (the underlying services don't yet expose Rename). The
// helpers below centralise both halves so a future Rename API only changes
// one site.

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// pushRenameOverlayCmd returns a tea.Cmd that pushes an input overlay for a
// rename action. The overlay is pre-filled with currentName; on submit the
// operator's value flows back through pushOverlayMsg → InputResultMsg with
// the given overlayID, which the screen's handle*InputResult handler reads.
//
// Callers (3 screens):
//
//	OverlayIDIssuerRename     — Issuers (max 64)
//	OverlayIDRecipientRename  — Recipients (max 80)
//	OverlayIDIdentityRename   — Identities (max 80)
func pushRenameOverlayCmd(overlayID, title, currentName string, maxLen int) tea.Cmd {
	return func() tea.Msg {
		return pushOverlayMsg{newInputOverlay(overlayID, title, currentName, maxLen)}
	}
}

// stubRenameResultCmd returns a tea.Cmd that pushes the stub OK overlay used
// by every rename handler until the underlying service exposes Rename. Single
// source of truth for the French "stub — service non implémenté" wording.
func stubRenameResultCmd(newName string) tea.Cmd {
	return func() tea.Msg {
		return pushOverlayMsg{NewOKOverlay("Rename",
			fmt.Sprintf("Renommé en %q (stub — service non implémenté).", newName))}
	}
}
