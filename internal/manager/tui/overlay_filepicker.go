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
	// File selected — call onPick and close overlay.
	var cmd tea.Cmd
	if o.onPick != nil {
		cmd = o.onPick(full)
	}
	return o, tea.Batch(
		cmd,
		func() tea.Msg { return OverlayDoneMsg{Result: nil} },
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
	fgDim := lipgloss.NewStyle().Foreground(Palette.FgDim)
	fg := lipgloss.NewStyle().Foreground(Palette.Fg)
	sel := lipgloss.NewStyle().Foreground(Palette.Magenta).Bold(true)
	dirStyle := lipgloss.NewStyle().Foreground(Palette.Cyan)
	red := lipgloss.NewStyle().Foreground(Palette.Red)

	title := GlowMagent.Render("File Picker")
	dirLine := fgDim.Render("  " + o.dir)

	var lines []string
	if o.errMsg != "" {
		lines = []string{red.Render("  " + o.errMsg)}
	} else if len(o.entries) == 0 {
		lines = []string{fgDim.Render("  (empty directory)")}
	} else {
		// Compute visible window.
		start, end := o.visibleRange(filePickerHeight)
		for i := start; i < end; i++ {
			e := o.entries[i]
			name := e.Name()
			if e.IsDir() {
				name = dirStyle.Render(name + "/")
			} else {
				name = fg.Render(name)
			}
			if i == o.cursor {
				lines = append(lines, sel.Render("> ")+name)
			} else {
				lines = append(lines, "  "+name)
			}
		}
	}

	hints := fgDim.Render("  ↑/↓ navigate   enter select/descend   ← up   esc cancel")
	content := lipgloss.JoinVertical(lipgloss.Left,
		append([]string{title, dirLine, ""},
			append(lines, "", hints)...)...,
	)

	w := 60
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
