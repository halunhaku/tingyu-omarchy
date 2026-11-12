//go:build linux

package tokens

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// colorsTOML is what Omarchy's tokyo-night theme writes, trimmed to the keys
// the client reads.
const colorsTOML = `mode = "dark"

accent = "#7aa2f7"
selection = "#292e42"
muted = "#414868"

background = "#1a1b26"
`

// TestReadThemeFollowsTheStateDir reads a theme the way Omarchy lays it out:
// the name beside the directory, the colours inside it.
func TestReadThemeFollowsTheStateDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	current := filepath.Join(dir, "omarchy", "current")
	if err := os.MkdirAll(filepath.Join(current, "theme"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "theme.name"), []byte("tokyo-night\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "theme", "colors.toml"), []byte(colorsTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	got := readTheme()
	if got.Theme != "tokyo-night" {
		t.Errorf("theme = %q, want tokyo-night", got.Theme)
	}
	if got.Mode != "dark" || !got.Dark() {
		t.Errorf("mode = %q, want dark", got.Mode)
	}
	if !got.HasAccent || got.Accent != ui.Hex("#7aa2f7") {
		t.Errorf("accent = %v (has %v), want #7aa2f7", got.Accent, got.HasAccent)
	}
	if got.HasRounding {
		t.Errorf("reading the theme alone claimed a rounding")
	}
}

// TestReadThemeWithoutAStateDirKeepsTheDesign covers a machine that is not
// Omarchy's: the zero Desktop is the answer, not an error.
func TestReadThemeWithoutAStateDirKeepsTheDesign(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if got := readTheme(); !got.Equal(Desktop{}) {
		t.Fatalf("readTheme = %+v, want the zero Desktop", got)
	}
}

// TestParseColorsIgnoresWhatItDoesNotUse keeps a malformed or unusual theme
// file from taking the window's colours with it.
func TestParseColorsIgnoresWhatItDoesNotUse(t *testing.T) {
	mode, accent, ok := parseColors([]byte("# a comment\nselection = \"#292e42\"\nmode = \"light\"\naccent = \"nonsense\"\n"))
	if mode != "light" || ok || accent.A != 0 {
		t.Errorf("parseColors = %q, %v, %v; want light and no accent", mode, accent, ok)
	}
	if mode, _, _ := parseColors([]byte(`mode = "sepia"`)); mode != "" {
		t.Errorf("mode %q was accepted", mode)
	}
}

// TestHexColor is the accent parser's contract: exactly the #rrggbb a theme
// writes, with or without the hash.
func TestHexColor(t *testing.T) {
	cases := []struct {
		in   string
		want ui.Color
		ok   bool
	}{
		{"#7aa2f7", ui.RGB(0x7a, 0xa2, 0xf7), true},
		{"7AA2F7", ui.RGB(0x7a, 0xa2, 0xf7), true},
		{" #0e0e10 ", ui.RGB(0x0e, 0x0e, 0x10), true},
		{"#7aa2", ui.Color{}, false},
		{"#7aa2f7ff", ui.Color{}, false},
		{"#gggggg", ui.Color{}, false},
		{"", ui.Color{}, false},
	}
	for _, c := range cases {
		got, ok := hexColor(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("hexColor(%q) = %v, %v; want %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}
