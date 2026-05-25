package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// filePickerOverlay is a list-based file-system navigator.
// Navigate with ↑/↓, Enter to select file or descend into dir, Backspace/Left
// to go up. Esc cancels without selecting.
type filePickerOverlay struct {
	dir    string
	entries []os.DirEntry
	cursor  int
	onPick  func(path string) tea.Cmd
	errMsg  string
	width   int
}

const filePickerHeight = 16

// newFilePickerOverlay constructs a file picker starting at the user's home dir.
// onPick is called with the absolute path when the user selects a file.
func newFilePickerOverlay(onPick func(path string) tea.Cmd) *filePickerOverlay {
	dir, err := os.UserHomeDir()
	if err != nil {
		dir = "."
	}
	o := &filePickerOverlay{dir: dir, onPick: onPick}
	o.load()
	return o
}

func (o *filePickerOverlay) load() {
	entries, err := os.ReadDir(o.dir)
	if err != nil {
		o.errMsg = err.Error()
		o.entries = nil
		return
	}
	o.errMsg = ""
	// Dirs first, then files, both sorted alphabetically.
	var dirs, files []os.DirEntry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue // skip hidden entries
		}
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	o.entries = append(dirs, files...)
	o.cursor = 0
}

func (o *filePickerOverlay) Init() tea.Cmd { return nil }

func (o *filePickerOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		o.width = msg.Width
		return o, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if o.cursor > 0 {
				o.cursor--
			}
		case "down", "j":
			if o.cursor < len(o.entries)-1 {
				o.cursor++
			}
		case "enter":
			return o.handleEnter()
		case "backspace", "left", "h":
			return o.navigateUp()
		case "esc":
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		}

	case tea.MouseMsg:
		if msg.Button != tea.MouseButtonLeft || msg.Action != tea.MouseActionPress {
			return o, nil
		}
		// Layout (overlay-relative): border(1) + padding(1) + header(1) +
		// keyHints(1) + blank(1) = 5. Entry rows start at Y=5.
		const entryStartY = 5
		row := msg.Y - entryStartY
		if row < 0 || row >= filePickerHeight {
			return o, nil
		}
		start, end := o.visibleRange(filePickerHeight)
		idx := start + row
		if idx < 0 || idx >= end {
			return o, nil
		}
		o.cursor = idx
		return o.handleEnter()
	}
	return o, nil
}

func (o *filePickerOverlay) handleEnter() (Overlay, tea.Cmd) {
	if len(o.entries) == 0 {
		return o, nil
	}
	entry := o.entries[o.cursor]
	full := filepath.Join(o.dir, entry.Name())
	if entry.IsDir() {
		o.dir = full
		o.load()
		return o, nil
	}
	// File selected — close overlay first, THEN deliver the path. Using
	// tea.Sequence guarantees OverlayDoneMsg lands before the onPick cmd
	// fires, so the picked-path message reaches whichever overlay/screen is
	// underneath (e.g. the wizard) rather than racing into the now-closing
	// file picker.
	var cmd tea.Cmd
	if o.onPick != nil {
		cmd = o.onPick(full)
	}
	return o, tea.Sequence(
		func() tea.Msg { return OverlayDoneMsg{Result: nil} },
		cmd,
	)
}

func (o *filePickerOverlay) navigateUp() (Overlay, tea.Cmd) {
	parent := filepath.Dir(o.dir)
	if parent == o.dir {
		return o, nil // already at root
	}
	o.dir = parent
	o.load()
	return o, nil
}

func (o *filePickerOverlay) View() string {
	// Header: "filepicker  cwd  <path>" with key hints.
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		GlowCyan.Render("filepicker"),
		"  ", Dim.Render("cwd"), " ", Base.Render(o.dir),
	)
	keyHints := lipgloss.JoinHorizontal(lipgloss.Top,
		HintKey.Render("↑↓"), HintText.Render(" nav  "),
		HintKey.Render("↵"), HintText.Render(" choisir  "),
		HintKey.Render("←"), HintText.Render(" remonter  "),
		HintKey.Render("esc"), HintText.Render(" annuler"),
	)

	var lines []string
	if o.errMsg != "" {
		lines = []string{FgRed.Render("  " + o.errMsg)}
	} else if len(o.entries) == 0 {
		lines = []string{Mute.Render("  (répertoire vide)")}
	} else {
		start, end := o.visibleRange(filePickerHeight)
		for i := start; i < end; i++ {
			e := o.entries[i]
			// Prototype icons: ▸ for dir, ● for exe/binary, · for other.
			var icon string
			var nameStyle lipgloss.Style
			if e.IsDir() {
				icon = GlowCyan.Render("▸")
				nameStyle = FgCyan
			} else {
				ext := filepath.Ext(e.Name())
				switch ext {
				case ".exe", ".elf", ".dmg", ".bin", ".sh":
					icon = GlowMagent.Render("●")
					nameStyle = FgMagenta
				default:
					icon = Dim.Render("·")
					nameStyle = Dim
				}
			}
			name := nameStyle.Render(e.Name())
			if e.IsDir() {
				name = nameStyle.Render(e.Name() + "/")
			}
			if i == o.cursor {
				lines = append(lines, GlowMagent.Render("> ")+icon+" "+name)
			} else {
				lines = append(lines, "  "+icon+" "+name)
			}
		}
	}

	// Footer: selected path.
	selectedPath := Mute.Render("— aucun —")
	if len(o.entries) > 0 && o.cursor < len(o.entries) && !o.entries[o.cursor].IsDir() {
		selectedPath = GlowMagent.Render(filepath.Join(o.dir, o.entries[o.cursor].Name()))
	}
	footer := lipgloss.JoinHorizontal(lipgloss.Top,
		Dim.Render("sélection : "), selectedPath,
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		append(
			[]string{header, keyHints, ""},
			append(lines, "", footer)...,
		)...,
	)

	w := 68
	if o.width > 0 && o.width < w+4 {
		w = o.width - 4
	}
	return Modal.Width(w).Render(content)
}

// visibleRange computes the [start, end) slice of entries to render,
// keeping the cursor visible within the given window height.
func (o *filePickerOverlay) visibleRange(windowH int) (start, end int) {
	n := len(o.entries)
	if n <= windowH {
		return 0, n
	}
	start = o.cursor - windowH/2
	if start < 0 {
		start = 0
	}
	end = start + windowH
	if end > n {
		end = n
		start = end - windowH
		if start < 0 {
			start = 0
		}
	}
	return start, end
}
