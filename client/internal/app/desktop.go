package app

import (
	"fmt"
	"time"

	"github.com/halunhaku/tingyu/client/internal/ui/tokens"
)

// desktopPollInterval is how often the app re-reads the desktop's appearance.
// Omarchy switches a theme by removing and recreating its whole state
// directory, so nothing short of re-reading sees the change (a file watcher on
// the old inode would go quiet). Two seconds looks immediate to a person and
// costs almost nothing: two small file reads and one Hyprland query.
const desktopPollInterval = 2 * time.Second

// observeDesktop reads the desktop's appearance. It runs in prepare, before
// the window exists, so the first frame is already Omarchy's rather than
// flashing the design's red and then settling.
func (a *application) observeDesktop() {
	a.desktop = tokens.Read()
	if a.model != nil {
		a.model.SetDesktop(a.desktop)
	}
	a.logf("desktop: %s", describeDesktop(a.desktop))
}

// watchDesktop polls the desktop's appearance until the app quits, and redraws
// the window when it changes. current is the reading the frame was built with,
// which keeps the poller off the model (that belongs to the window's thread).
func (a *application) watchDesktop(current tokens.Desktop) {
	ticker := time.NewTicker(desktopPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-a.done:
			return
		case <-ticker.C:
		}
		next := tokens.Read()
		if next.Equal(current) {
			continue
		}
		current = next
		a.logf("desktop: %s", describeDesktop(next))
		a.updateWindow(func() { a.model.SetDesktop(next) })
	}
}

// describeDesktop is the one line the log and the M4 record quote.
func describeDesktop(d tokens.Desktop) string {
	theme, mode := d.Theme, d.Mode
	if theme == "" {
		theme = "none"
	}
	if mode == "" {
		mode = "unstated"
	}
	accent := "design"
	if d.HasAccent {
		accent = fmt.Sprintf("#%02x%02x%02x", d.Accent.R, d.Accent.G, d.Accent.B)
	}
	rounding := "unstated"
	if d.HasRounding {
		rounding = fmt.Sprintf("%g", d.Rounding)
	}
	return fmt.Sprintf("theme %s (%s), accent %s, rounding %s", theme, mode, accent, rounding)
}
