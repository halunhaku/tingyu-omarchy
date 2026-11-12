package tokens

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestAccentedKeepsTheDesignsRecipe pins the derivation of the desktop accent:
// the accent family moves, everything else stays the design's, and the ink is
// chosen by contrast rather than inherited from the red it replaced.
func TestAccentedKeepsTheDesignsRecipe(t *testing.T) {
	// tokyo-night's accent is a light blue, so black ink reads on it.
	tokyo := ui.Hex("#7aa2f7")
	p := Dark.Accented(tokyo)
	if p.Accent != tokyo || p.Focus != tokyo {
		t.Errorf("accent/focus = %v/%v, want %v", p.Accent, p.Focus, tokyo)
	}
	if p.OnAccent != ui.RGB(0, 0, 0) {
		t.Errorf("ink on %v = %v, want black", tokyo, p.OnAccent)
	}
	if p.AccentSoft.A != Dark.AccentSoft.A || p.Selected.A != Dark.Selected.A {
		t.Errorf("soft/selected alpha = %d/%d, want the design's %d/%d",
			p.AccentSoft.A, p.Selected.A, Dark.AccentSoft.A, Dark.Selected.A)
	}
	if p.AccentHover.R < p.Accent.R || p.AccentHover.G < p.Accent.G || p.AccentHover.B < p.Accent.B {
		t.Errorf("hover %v is not lighter than %v", p.AccentHover, p.Accent)
	}
	if p.AccentPressed.R > p.Accent.R || p.AccentPressed.G > p.Accent.G || p.AccentPressed.B > p.Accent.B {
		t.Errorf("pressed %v is not darker than %v", p.AccentPressed, p.Accent)
	}
	// Surfaces and text are what the pages were laid out against.
	if p.Canvas != Dark.Canvas || p.SurfaceRaised != Dark.SurfaceRaised ||
		p.TextPrimary != Dark.TextPrimary || p.Danger != Dark.Danger {
		t.Errorf("Accented moved a non-accent token: %+v", p)
	}

	// catppuccin-latte's mauve is dark enough that white ink reads on it.
	latte := Light.Accented(ui.Hex("#8839ef"))
	if latte.OnAccent != ui.RGB(255, 255, 255) {
		t.Errorf("ink on %v = %v, want white", ui.Hex("#8839ef"), latte.OnAccent)
	}
}

// TestCornersFollowTheCompositor covers the three readings M4 must honour: a
// square desktop, a partially rounded one, and one that rounds at least as
// much as the design (where the design's own values stand).
func TestCornersFollowTheCompositor(t *testing.T) {
	radii := []float32{RadiusXS, RadiusSM, RadiusMD, RadiusLG, RadiusXL}

	square := Corners{Rounding: 0, Known: true}
	for _, r := range radii {
		if got := square.Radius(r); got != 0 {
			t.Errorf("rounding 0 left %v, want a square corner", got)
		}
	}
	if got := square.Radius(RadiusFull); got != RadiusFull {
		t.Errorf("rounding 0 turned the pill into %v", got)
	}

	unknown := Corners{}
	for _, r := range radii {
		if got := unknown.Radius(r); got != r {
			t.Errorf("an unknown preference changed %v to %v", r, got)
		}
	}
	if got := unknown.Radius(RadiusFull); got != RadiusFull {
		t.Errorf("an unknown preference changed the pill to %v", got)
	}

	half := Corners{Rounding: RadiusSM / 2, Known: true}
	if got, want := half.Radius(RadiusSM), RadiusSM/2; got != want {
		t.Errorf("half rounding on RadiusSM = %v, want %v", got, want)
	}
	if got, want := half.Radius(RadiusLG), RadiusLG/2; got != want {
		t.Errorf("half rounding on RadiusLG = %v, want %v", got, want)
	}

	// Hyprland may round more than the design does; the design keeps its
	// proportions instead of growing with every desktop.
	wide := Corners{Rounding: 48, Known: true}
	for _, r := range radii {
		if got := wide.Radius(r); got != r {
			t.Errorf("rounding 48 changed %v to %v", r, got)
		}
	}
	if got := wide.Radius(RadiusFull); got != RadiusFull {
		t.Errorf("rounding 48 turned the pill into %v", got)
	}
}

// TestApplyWritesTheCornersAndAccent locks the one place the built-in theme is
// written: controls follow the same corners and accent as the drawn elements.
func TestApplyWritesTheCornersAndAccent(t *testing.T) {
	square := Light.Apply(&ui.Theme{}, false, Corners{Rounding: 0, Known: true})
	if square.Radius != 0 {
		t.Errorf("theme radius = %v on a square desktop, want 0", square.Radius)
	}
	design := Light.Apply(&ui.Theme{}, false, Corners{})
	if design.Radius != RadiusSM {
		t.Errorf("theme radius = %v without a desktop reading, want %v", design.Radius, RadiusSM)
	}

	accent := ui.Hex("#7aa2f7")
	themed := Light.Accented(accent).Apply(&ui.Theme{}, false, Corners{})
	if themed.Accent != accent || themed.AccentText != Light.Accented(accent).OnAccent {
		t.Errorf("theme accent = %v/%v, want the desktop's %v", themed.Accent, themed.AccentText, accent)
	}
}

// TestDesktopDarkAndEqual covers the two readings the shell and the poller ask
// of a Desktop.
func TestDesktopDarkAndEqual(t *testing.T) {
	a := Desktop{Theme: "tokyo-night", Mode: "dark", Accent: ui.Hex("#7aa2f7"), HasAccent: true}
	if !a.Dark() {
		t.Errorf("tokyo-night (%q) is not dark", a.Mode)
	}
	if (Desktop{Mode: "light"}).Dark() {
		t.Errorf("a light theme reported dark")
	}
	if !a.Equal(a) {
		t.Errorf("a reading does not equal itself")
	}
	b := a
	b.HasRounding, b.Rounding = true, 6
	if a.Equal(b) {
		t.Errorf("a change of rounding went unnoticed")
	}
	if c := (Desktop{Theme: a.Theme, Mode: a.Mode, Accent: ui.Hex("#7aa2f8"), HasAccent: true}); a.Equal(c) {
		t.Errorf("a change of accent went unnoticed")
	}
	if got := (Desktop{Rounding: 4, HasRounding: true}).Corners(); got != (Corners{Rounding: 4, Known: true}) {
		t.Errorf("Corners() = %+v", got)
	}
}
