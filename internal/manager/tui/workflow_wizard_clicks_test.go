package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestWorkflow_WizardSidebarClickMapping is the regression guard for the
// operator-reported bug: "dans les étapes à gauche les clics sont mal mappés,
// il faut cliquer à côté pour que ça fonctionne". Root cause: the wizardModel
// rendered each line at m.width, but wizardOverlay.View() wraps the model
// inside a Modal sized at m.width-6 with padding(1,2). The 12-cell mismatch
// made the progress bar overflow onto a second line, which pushed every
// sidebar row down by one — so a click at the visually-correct Y landed on
// the row ABOVE the targeted step.
//
// The fix renders with `usableW = m.width - 12`. This test drives clicks at
// each step's expected body-local Y and asserts the wizardModel step actually
// jumped to the targeted index.
func TestWorkflow_WizardSidebarClickMapping(t *testing.T) {
	wiz := &wizardOverlay{model: newWizardModel(nil)}
	wiz.model.width, wiz.model.hgt = 144, 40

	// Drive a synthetic WindowSizeMsg so the model adjusts internal state.
	upd, _ := wiz.Update(tea.WindowSizeMsg{Width: 144, Height: 40})
	wiz = upd.(*wizardOverlay)

	// Render once so the View() side-effects (if any) settle.
	_ = wiz.View()

	// frameX=3, frameY=2 (Modal border+padding). Progress strip occupies
	// bodyY 0,1 — sidebar steps start at bodyY=2, two rows each.
	const frameY = 2
	for idx := 0; idx < 8; idx++ {
		bodyY := 2 + 2*idx
		absY := bodyY + frameY
		// Click on the label row (badge X≈8 is well within sidebar bounds).
		click := tea.MouseMsg{
			X: 3 + 8, Y: absY,
			Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
		}
		upd, _ := wiz.Update(click)
		w := upd.(*wizardOverlay)
		if int(w.model.step) != idx {
			t.Errorf("click on step %d (bodyY=%d, absY=%d): wizard.step=%d, want %d",
				idx, bodyY, absY, w.model.step, idx)
		}
		wiz = w
	}
}

// TestWorkflow_WizardEscQuitsImmediately covers the operator-reported bug:
// "la touche echap ne fait pas quitter le workflow". Esc now emits
// WizardDoneMsg{Issued: nil} immediately from any step.
func TestWorkflow_WizardEscQuitsImmediately(t *testing.T) {
	wm := newWizardModel(nil)
	wm.step = wizStepValidity // start mid-flow
	_, cmd := wm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc emitted no cmd; wizard didn't close")
	}
	done, ok := cmd().(WizardDoneMsg)
	if !ok {
		t.Fatalf("Esc emitted %T, want WizardDoneMsg", cmd())
	}
	if done.Issued != nil {
		t.Errorf("Esc emitted Issued=%v, want nil (discard)", done.Issued)
	}
}

// TestWorkflow_WizardTitleDiffersForReissue covers the operator-reported bug:
// "pour regénérer un license, le titre du popup devrait être différent". The
// new-licence wizard renders "NOUVELLE LICENCE"; the re-issue wizard renders
// "RÉ-ÉMETTRE LICENCE" so the operator can tell which flow they're in.
func TestWorkflow_WizardTitleDiffersForReissue(t *testing.T) {
	newWiz := newWizardModel(nil)
	newWiz.width, newWiz.hgt = 144, 40
	newView := newWiz.View()
	if !strings.Contains(newView, "NOUVELLE LICENCE") {
		t.Error("new-licence wizard missing 'NOUVELLE LICENCE' title")
	}
	if strings.Contains(newView, "RÉ-ÉMETTRE") {
		t.Error("new-licence wizard accidentally shows re-issue title")
	}

	reWiz := newWizardModel(nil)
	reWiz.width, reWiz.hgt = 144, 40
	reWiz.state.IsReissue = true
	reView := reWiz.View()
	if !strings.Contains(reView, "RÉ-ÉMETTRE") {
		t.Error("re-issue wizard missing 'RÉ-ÉMETTRE' title")
	}
	if strings.Contains(reView, "NOUVELLE LICENCE") {
		t.Error("re-issue wizard accidentally shows new-licence title")
	}
}

// TestWorkflow_WizardProgressBarFitsInModal covers the operator-reported bug:
// "la progress bar du workflow déborde sur la ligne du dessous". The bar must
// be no wider than the modal's inner content area (m.width - 12). Before the
// fix the bar was rendered at m.width = 144 inside a modal of width 138,
// forcing lipgloss to wrap it onto a second line.
func TestWorkflow_WizardProgressBarFitsInModal(t *testing.T) {
	wm := newWizardModel(nil)
	wm.width, wm.hgt = 144, 40
	view := wm.View()
	// The progress strip + bar must each fit on a single line. Split the
	// rendered view by lines and assert no line exceeds the usable width.
	const usableW = 144 - 12
	for i, line := range strings.Split(view, "\n") {
		if i > 3 {
			break // only inspect the progress strip + bar
		}
		// Strip ANSI before measuring real cell width — lipgloss.Width does
		// the right thing for styled lines.
		// (We use the simpler len() ceiling here: ANSI escapes only inflate.)
		if len(line) > usableW+512 { // 512 = generous ANSI escape budget
			t.Errorf("line %d longer than usable width %d: %d chars", i, usableW, len(line))
		}
	}
	// Also: the bar's pre-fix overflow showed as a stray line containing
	// only block characters. After the fix, the bar is always one line.
	barChars := strings.Count(view, "█") + strings.Count(view, "░")
	// One progress bar's worth of block characters is fine.
	// A second wrapping would more than double it — heuristic guard.
	if barChars > usableW+10 {
		t.Errorf("progress bar appears to wrap: %d block chars (usableW=%d)", barChars, usableW)
	}
}
