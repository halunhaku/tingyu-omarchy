package tokens

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Desktop is what the desktop environment says about its own look: the Omarchy
// theme in effect and the compositor's corner radius. The app reads it once
// before the window is built and polls it afterwards, because Omarchy replaces
// its whole theme directory on a switch (a file watcher over the old inode
// would go quiet) and Hyprland's rounding can change at any time.
//
// The zero value means the app could not ask — another platform, no Omarchy
// state, or a compositor that did not answer — and the design keeps its own
// accent and corners. That is also why the flags are explicit: a square
// desktop (Rounding 0) is a reading, not an absence.
type Desktop struct {
	// Theme is theme.name ("tokyo-night"), Mode its "dark" or "light" and
	// Accent the accent colour of colors.toml. HasAccent reports that the
	// colour was read and is opaque.
	Theme     string
	Mode      string
	Accent    ui.Color
	HasAccent bool

	// Rounding is decoration:rounding in DIPs; HasRounding reports that the
	// compositor answered. Zero is a square desktop, not "unknown".
	Rounding    float32
	HasRounding bool
}

// Dark reports the theme's own brightness. It only means something when Theme
// is set; the settings still decide whether the client follows it.
func (d Desktop) Dark() bool { return d.Mode == "dark" }

// Corners returns the desktop's corner preference.
func (d Desktop) Corners() Corners {
	return Corners{Rounding: d.Rounding, Known: d.HasRounding}
}

// Equal reports whether two readings say the same thing, which is how the
// poller knows it has nothing to redraw.
func (d Desktop) Equal(e Desktop) bool {
	return d.Theme == e.Theme && d.Mode == e.Mode &&
		d.HasAccent == e.HasAccent && d.Accent.R == e.Accent.R &&
		d.Accent.G == e.Accent.G && d.Accent.B == e.Accent.B &&
		d.Accent.A == e.Accent.A &&
		d.HasRounding == e.HasRounding && d.Rounding == e.Rounding
}

// Corners is the desktop's corner preference, which the design's radii follow.
type Corners struct {
	// Rounding is the compositor's radius in DIPs, and Known says it
	// answered; an unknown preference leaves the design untouched.
	Rounding float32
	Known    bool
}

// Radius maps a design radius onto the desktop's corners: a square corner
// where the compositor rounds nothing, the design's own value where it rounds
// at least as much as the design does (RadiusSM, the corner every control
// uses), and a proportional corner in between. RadiusFull stays a pill: that
// radius is a shape, not a corner, and squaring it would turn every tag and
// progress track into a rectangle.
func (c Corners) Radius(r float32) float32 {
	if r >= RadiusFull || !c.Known {
		return r
	}
	if c.Rounding <= 0 {
		return 0
	}
	if scale := c.Rounding / RadiusSM; scale < 1 {
		return r * scale
	}
	return r
}

// The desktop accent's hover and pressed faces. The design lightens on hover
// and darkens on press in both palettes (§4.2), so a desktop accent keeps that
// behaviour instead of gaining a second one.
const (
	accentHoverMix   float32 = 0.10
	accentPressedMix float32 = 0.12
)

// Accented returns the palette with the desktop's accent in place of the
// design's red. Only the accent family moves: the surfaces, the text and the
// status colours are what the pages were laid out against, and a desktop
// accent is a highlight, not a second palette.
//
// The derived faces keep the design's own recipe. Soft and Selected keep the
// alphas the palette already uses, hover and pressed move toward white and
// black, and the ink follows the accent's contrast so a light desktop accent
// (catppuccin-latte) reads as well as a dark one (tokyo-night).
func (p Palette) Accented(a ui.Color) Palette {
	p.Accent = a
	p.AccentHover = Mix(a, ui.RGB(255, 255, 255), accentHoverMix)
	p.AccentPressed = Mix(a, ui.RGB(0, 0, 0), accentPressedMix)
	p.AccentSoft = a.Alpha(float32(p.AccentSoft.A) / 255)
	p.Selected = a.Alpha(float32(p.Selected.A) / 255)
	p.OnAccent = onAccent(a)
	p.Focus = a
	return p
}

// onAccent is the ink that reads on an accent face: white or black, whichever
// has the higher WCAG contrast ratio. The design's accent is dark in the light
// palette and light in the dark one, so a desktop accent cannot inherit either.
func onAccent(a ui.Color) ui.Color {
	white, black := ui.RGB(255, 255, 255), ui.RGB(0, 0, 0)
	if contrastRatio(a, white) >= contrastRatio(a, black) {
		return white
	}
	return black
}

// contrastRatio is WCAG 2.1's (L1 + 0.05) / (L2 + 0.05).
func contrastRatio(a, b ui.Color) float64 {
	x, y := luminance(a), luminance(b)
	if y > x {
		x, y = y, x
	}
	return (x + 0.05) / (y + 0.05)
}

// luminance is WCAG 2.1's relative luminance: the sRGB channels linearised and
// weighted.
func luminance(c ui.Color) float64 {
	linear := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}
