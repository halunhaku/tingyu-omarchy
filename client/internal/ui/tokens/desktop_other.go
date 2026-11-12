//go:build !linux

package tokens

// Read returns the zero Desktop on every platform that is not Omarchy's: there
// is no state directory to read and no Hyprland to ask, so the client keeps the
// design's accent and corners.
func Read() Desktop { return Desktop{} }
