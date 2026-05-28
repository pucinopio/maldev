package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
)

// totpQRPopup centres the TOTP QR ASCII art over the underlying view so
// the half-block grid can never get wrapped or clipped by the listBox/
// detailBox split that hosts the TOTP screen. The popup auto-sizes to
// the QR's natural width so any future change to the QR generator (e.g.
// higher error-correction level → wider bitmap) keeps rendering cleanly.
type totpQRPopup struct {
	view *service.TOTPSecretView
}

func newTOTPQRPopup(view *service.TOTPSecretView) *totpQRPopup {
	return &totpQRPopup{view: view}
}

func (o *totpQRPopup) Init() tea.Cmd { return nil }

func (o *totpQRPopup) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "esc", "q", "Q", "enter":
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		}
	case tea.MouseMsg:
		if m.Button == tea.MouseButtonLeft && m.Action == tea.MouseActionPress {
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		}
	}
	return o, nil
}

func (o *totpQRPopup) View() string {
	if o.view == nil {
		return Modal.Render(Dim.Render("aucun TOTP sélectionné"))
	}
	title := GlowMagent.Render("QR · ") + GlowCyan.Render(o.view.AccountLabel)
	qr := o.view.QRImageASCII
	// Measure the QR's natural width to size the modal — pre-popup design
	// constrained the QR by the host's Width() which wrapped each row.
	qrW := 0
	for _, line := range strings.Split(qr, "\n") {
		if w := lipgloss.Width(line); w > qrW {
			qrW = w
		}
	}
	footer := Dim.Render("secret: ") + Mute.Render(o.view.Secret)
	hint := Dim.Render("[esc/q] fermer")
	content := lipgloss.JoinVertical(lipgloss.Left, title, "", qr, "", footer, hint)
	return Modal.Render(content)
}
