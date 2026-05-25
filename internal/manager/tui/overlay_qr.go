package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
)

// QRSavedMsg is emitted when the PEM has been written to disk.
type QRSavedMsg struct {
	Path string
	Err  error
}

// qrOverlay is a modal that displays the QR code (ASCII art) + PEM viewer
// for a freshly issued licence. It also provides a "save to file" action.
type qrOverlay struct {
	issued  *service.IssuedLicense
	pemOff  int    // scroll offset in PEM lines
	saved   string // path written on save
	saveErr string
	width   int
}

// NewQROverlay constructs the QR overlay from an IssuedLicense.
// Exported so tests can snapshot the overlay directly.
func NewQROverlay(issued *service.IssuedLicense) Overlay {
	return newQROverlay(issued)
}

func newQROverlay(issued *service.IssuedLicense) *qrOverlay {
	return &qrOverlay{issued: issued}
}

func (o *qrOverlay) Init() tea.Cmd { return nil }

func (o *qrOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		o.width = msg.Width
		return o, nil

	case QRSavedMsg:
		if msg.Err != nil {
			o.saveErr = msg.Err.Error()
		} else {
			o.saved = msg.Path
		}
		return o, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "enter":
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		case "s":
			return o, o.saveCmd()
		case "c":
			if o.issued != nil {
				_ = clipboard.WriteAll(string(o.issued.PEM))
			}
			return o, nil
		case "up", "k":
			if o.pemOff > 0 {
				o.pemOff--
			}
		case "down", "j":
			if n := len(o.pemLines()); o.pemOff < n-1 {
				o.pemOff++
			}
		}
	}
	return o, nil
}

func (o *qrOverlay) pemLines() []string {
	if o.issued == nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(o.issued.PEM)), "\n")
}

func (o *qrOverlay) saveCmd() tea.Cmd {
	issued := o.issued
	return func() tea.Msg {
		if issued == nil {
			return QRSavedMsg{Err: fmt.Errorf("no licence data")}
		}
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		path := filepath.Join(home, fmt.Sprintf("licence-%s.pem", issued.Row.LicenseUUID))
		if err := os.WriteFile(path, issued.PEM, 0o600); err != nil {
			return QRSavedMsg{Err: err}
		}
		return QRSavedMsg{Path: path}
	}
}

func (o *qrOverlay) View() string {
	// Local aliases — Dim is exactly fgDim; Glow* unbolded matches the calm
	// status-line look the QR overlay needs (compare to drawer_probe.go).
	fgDim := Dim
	green := GlowGreen.UnsetBold()
	red := GlowRed.UnsetBold()

	title := GlowGreen.Render("Licence Issued")

	var sections []string

	// QR code block (shown only when a TOTP binding was issued).
	if o.issued != nil && len(o.issued.TOTPs) > 0 {
		for i, t := range o.issued.TOTPs {
			header := fgDim.Render(fmt.Sprintf("TOTP binding %d — binding index %d", i+1, t.BindingIndex))
			qr := t.QRImageASCII
			if qr == "" {
				qr = fgDim.Render("(QR unavailable)")
			}
			sections = append(sections, header, qr, "")
		}
	}

	// PEM excerpt (first + last 3 lines visible, scrollable).
	if o.issued != nil {
		lines := o.pemLines()
		visible := lines
		const maxVisible = 8
		if len(lines) > maxVisible {
			start := o.pemOff
			end := start + maxVisible
			if end > len(lines) {
				end = len(lines)
				start = end - maxVisible
				if start < 0 {
					start = 0
				}
			}
			visible = lines[start:end]
		}
		pemBlock := strings.Join(visible, "\n")
		sections = append(sections,
			fgDim.Render("PEM (↑/↓ scroll):"),
			Base.Render(pemBlock),
			"",
		)
	}

	// Status line.
	switch {
	case o.saveErr != "":
		sections = append(sections, red.Render("save error: "+o.saveErr))
	case o.saved != "":
		sections = append(sections, green.Render("saved → "+o.saved))
	}

	sections = append(sections,
		"",
		fgDim.Render("s save   c copy PEM   esc/enter close"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		append([]string{title, ""}, sections...)...,
	)

	w := 70
	if o.width > 0 && o.width < w+4 {
		w = o.width - 4
	}
	return ModalOK.Width(w).Render(content)
}
