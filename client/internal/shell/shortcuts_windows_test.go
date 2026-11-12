package shell

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/halunhaku/tingyu/client/internal/ui/model"
)

func TestWindowsCtrlShortcutsMatchLabels(t *testing.T) {
	m := newModel(t)
	tt := ui.NewTester(m.View, DefaultWidth, DefaultHeight)
	frame(tt)
	tt.Key(ui.Ctrl, ui.KeyL)
	frame(tt)
	if m.State().Page() != model.PageLyrics {
		t.Fatal("Ctrl+L did not open lyrics")
	}
	tt.Key(ui.Ctrl, ui.KeyK)
	frame(tt)
	if !m.State().Palette().Open || !tt.HasText("Ctrl+L") {
		t.Fatalf("palette does not show Windows shortcut: %v", tt.Texts())
	}
	for _, command := range model.Commands {
		if strings.ContainsAny(command.Shortcut, "⌘⇧⌥") {
			t.Fatalf("macOS hint on Windows: %+v", command)
		}
	}
}
