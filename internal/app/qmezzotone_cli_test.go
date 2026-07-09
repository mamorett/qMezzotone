package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mamorett/qMezzotone/internal/app"

	tea "charm.land/bubbletea/v2"
)

func copyFixtureToTemp(t *testing.T, src string) string {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read fixture %q: %v", src, err)
	}
	tmp := t.TempDir()
	dst := filepath.Join(tmp, filepath.Base(src))
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("failed to write fixture copy %q: %v", dst, err)
	}
	return dst
}

func TestInitialImagePath_SelectsFileAndJumpsToRenderOptions(t *testing.T) {
	fixture := copyFixtureToTemp(t, "../services/testdata/gradient_edges.png")

	m := app.NewQMezzotoneModelWithConfig(app.QMezzotoneModelConfig{
		ImagePath: fixture,
	})

	// Init() performs the initial-image validation and auto-select.
	m.Init()
	// WindowSizeMsg (sent by the program on start) finalizes the layout so the
	// message viewport renders its content.
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	if m.CurrentActiveMenu() != app.RenderOptionsMenu {
		t.Fatalf("expected to boot into render options menu, got %d", m.CurrentActiveMenu())
	}
	if m.SelectedFile() != fixture {
		t.Fatalf("expected selectedFile %q, got %q", fixture, m.SelectedFile())
	}
}

func TestInitialImagePath_InvalidPath_ShowsErrorAndStaysInPicker(t *testing.T) {
	m := app.NewQMezzotoneModelWithConfig(app.QMezzotoneModelConfig{
		ImagePath: "/this/path/does/not/exist.png",
	})

	m.Init()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	if m.CurrentActiveMenu() != app.FilePickerMenu {
		t.Fatalf("expected to stay in file picker menu, got %d", m.CurrentActiveMenu())
	}
	if !strings.Contains(m.MessageViewContent(), "Invalid image path") {
		t.Fatalf("expected warning message for invalid image path, got message viewport %q", m.MessageViewContent())
	}
}
