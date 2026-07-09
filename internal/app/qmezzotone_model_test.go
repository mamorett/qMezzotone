package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mamorett/qMezzotone/internal/app"

	tea "charm.land/bubbletea/v2"
)

func key(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func keyText(text string) tea.KeyPressMsg {
	var code rune
	if len(text) > 0 {
		code = []rune(text)[0]
	}
	return tea.KeyPressMsg(tea.Key{Text: text, Code: code})
}

func TestNewQMezzotoneModelInitReturnsCmd(t *testing.T) {
	m := app.NewQMezzotoneModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatalf("expected init command to be non-nil")
	}
}

func TestQMezzotoneModelWindowResizeRendersView(t *testing.T) {
	m := app.NewQMezzotoneModel()

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model, ok := updated.(*app.QMezzotoneModel)
	if !ok {
		t.Fatalf("expected updated model type *app.QMezzotoneModel")
	}

	view := model.View().Content
	if strings.TrimSpace(view) == "" {
		t.Fatalf("expected non-empty view after resize")
	}
}

func TestQMezzotoneModelEscFromFilePickerRequiresDoubleEscToQuit(t *testing.T) {
	m := app.NewQMezzotoneModel()

	updated, cmd := m.Update(key(tea.KeyEsc))
	if cmd != nil {
		t.Fatalf("expected first esc from file picker to not quit")
	}

	updatedModel, ok := updated.(*app.QMezzotoneModel)
	if !ok {
		t.Fatalf("expected updated model type *app.QMezzotoneModel")
	}

	updated, cmd = updatedModel.Update(key(tea.KeyEsc))
	if cmd == nil {
		t.Fatalf("expected quit command on second esc from file picker")
	}

	if msg := cmd(); msg == nil {
		t.Fatalf("expected quit command to return a message")
	}
}

func TestQMezzotoneModelEscQuitIsCanceledByOtherKey(t *testing.T) {
	m := app.NewQMezzotoneModel()

	updated, cmd := m.Update(key(tea.KeyEsc))
	if cmd != nil {
		t.Fatalf("expected first esc from file picker to not quit")
	}

	updatedModel, ok := updated.(*app.QMezzotoneModel)
	if !ok {
		t.Fatalf("expected updated model type *app.QMezzotoneModel")
	}

	updated, cmd = updatedModel.Update(keyText("j"))
	if cmd != nil {
		t.Fatalf("expected non-esc key to not quit")
	}

	updatedModel, ok = updated.(*app.QMezzotoneModel)
	if !ok {
		t.Fatalf("expected updated model type *app.QMezzotoneModel")
	}

	updated, cmd = updatedModel.Update(key(tea.KeyEsc))
	if cmd != nil {
		t.Fatalf("expected esc after reset to not quit")
	}

	updatedModel, ok = updated.(*app.QMezzotoneModel)
	if !ok {
		t.Fatalf("expected updated model type *app.QMezzotoneModel")
	}

	_, cmd = updatedModel.Update(key(tea.KeyEsc))
	if cmd == nil {
		t.Fatalf("expected second esc after reset to quit")
	}
}

func TestQMezzotoneModelHelpToggleRendersAndHidesHelp(t *testing.T) {
	m := app.NewQMezzotoneModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model, ok := updated.(*app.QMezzotoneModel)
	if !ok {
		t.Fatalf("expected updated model type *app.QMezzotoneModel")
	}

	updated, _ = model.Update(keyText("h"))
	model, ok = updated.(*app.QMezzotoneModel)
	if !ok {
		t.Fatalf("expected updated model type *app.QMezzotoneModel")
	}

	helpView := model.View().Content
	if !strings.Contains(helpView, "CONTROLS") {
		t.Fatalf("expected help overlay to render after pressing h")
	}

	updated, _ = model.Update(keyText("h"))
	model, ok = updated.(*app.QMezzotoneModel)
	if !ok {
		t.Fatalf("expected updated model type *app.QMezzotoneModel")
	}

	viewWithoutHelp := model.View().Content
	if strings.Contains(viewWithoutHelp, "CONTROLS") {
		t.Fatalf("expected help overlay to hide after pressing h again")
	}
}

// newJumpTestModel builds a model rooted in a temp dir containing alpha.png,
// beta.png and gamma.png, with the file picker loaded.
func newJumpTestModel(t *testing.T) *app.QMezzotoneModel {
	t.Helper()
	tmp := t.TempDir()
	for _, name := range []string{"alpha.png", "beta.png", "gamma.png"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("fake"), 0o644); err != nil {
			t.Fatalf("failed to write %q: %v", name, err)
		}
	}
	m := app.NewQMezzotoneModel()
	m.SetFilePickerDirectory(tmp)
	m.RefreshFilePicker()
	return m
}

func TestJumpToFilePrefix_MovesCursor(t *testing.T) {
	m := newJumpTestModel(t)

	updated, _ := m.Update(keyText("b"))
	model, ok := updated.(*app.QMezzotoneModel)
	if !ok {
		t.Fatalf("expected updated model type *app.QMezzotoneModel")
	}

	if got := model.HighlightedFileName(); got != "beta.png" {
		t.Fatalf("expected highlighted file beta.png, got %q", got)
	}
}

func TestJumpToFilePrefix_NoMatch_LeavesCursor(t *testing.T) {
	m := newJumpTestModel(t)
	before := m.HighlightedFileName()

	updated, _ := m.Update(keyText("z"))
	model, ok := updated.(*app.QMezzotoneModel)
	if !ok {
		t.Fatalf("expected updated model type *app.QMezzotoneModel")
	}

	if got := model.HighlightedFileName(); got != before {
		t.Fatalf("expected cursor unchanged (%q), got %q", before, got)
	}
}

// newRepeatJumpTestModel roots the picker in a temp dir with several files
// sharing the "a" prefix so repeated same-key presses can be exercised.
func newRepeatJumpTestModel(t *testing.T) *app.QMezzotoneModel {
	t.Helper()
	tmp := t.TempDir()
	for _, name := range []string{"alpha.png", "apple.png", "apricot.png", "beta.png", "gamma.png"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("fake"), 0o644); err != nil {
			t.Fatalf("failed to write %q: %v", name, err)
		}
	}
	m := app.NewQMezzotoneModel()
	m.SetFilePickerDirectory(tmp)
	m.RefreshFilePicker()
	return m
}

func TestJumpToFilePrefix_RepeatedKeyAdvancesThroughMatches(t *testing.T) {
	// The picker opens with the cursor on the first entry (alpha.png). Pressing
	// a letter advances to the next entry starting with that letter (wrapping),
	// so from alpha the first "a" press moves to apple, then apricot, then wraps
	// back to alpha.
	m := newRepeatJumpTestModel(t)

	updated, _ := m.Update(keyText("a"))
	model, ok := updated.(*app.QMezzotoneModel)
	if !ok {
		t.Fatalf("expected updated model type *app.QMezzotoneModel")
	}
	if got := model.HighlightedFileName(); got != "apple.png" {
		t.Fatalf("expected next match apple.png, got %q", got)
	}

	updated, _ = model.Update(keyText("a"))
	model, ok = updated.(*app.QMezzotoneModel)
	if !ok {
		t.Fatalf("expected updated model type *app.QMezzotoneModel")
	}
	if got := model.HighlightedFileName(); got != "apricot.png" {
		t.Fatalf("expected next match apricot.png, got %q", got)
	}

	// Wraps back to the first "a" entry.
	updated, _ = model.Update(keyText("a"))
	model, _ = updated.(*app.QMezzotoneModel)
	if got := model.HighlightedFileName(); got != "alpha.png" {
		t.Fatalf("expected wrap-around to alpha.png, got %q", got)
	}
}

// TestJumpToFilePrefix_FromNonMatchingCursorLandsOnFirstMatch verifies that
// when the cursor is not on a matching entry, the first press jumps to the
// first matching entry (not skipping past it).
func TestJumpToFilePrefix_FromNonMatchingCursorLandsOnFirstMatch(t *testing.T) {
	m := newRepeatJumpTestModel(t)

	// Move the cursor off the "a" group first by jumping to "b".
	updated, _ := m.Update(keyText("b"))
	model, ok := updated.(*app.QMezzotoneModel)
	if !ok {
		t.Fatalf("expected updated model type *app.QMezzotoneModel")
	}
	if got := model.HighlightedFileName(); got != "beta.png" {
		t.Fatalf("expected beta.png, got %q", got)
	}

	// Now pressing "a" should land on the first "a" entry, alpha.png.
	updated, _ = model.Update(keyText("a"))
	model, _ = updated.(*app.QMezzotoneModel)
	if got := model.HighlightedFileName(); got != "alpha.png" {
		t.Fatalf("expected first match alpha.png, got %q", got)
	}
}

func TestJumpToFilePrefix_IgnoreNonRunes(t *testing.T) {
	m := newJumpTestModel(t)
	before := m.HighlightedFileName()

	for _, k := range []tea.KeyPressMsg{key(tea.KeyEnter), key(tea.KeyEsc), keyText("j"), key(tea.KeyPgUp)} {
		updated, _ := m.Update(k)
		model, ok := updated.(*app.QMezzotoneModel)
		if !ok {
			t.Fatalf("expected updated model type *app.QMezzotoneModel")
		}
		m = model
	}

	if got := m.HighlightedFileName(); got != before {
		t.Fatalf("expected cursor unchanged (%q) after non-jump keys, got %q", before, got)
	}
}
