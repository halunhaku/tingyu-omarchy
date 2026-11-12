//go:build linux

package mpv

import (
	"fmt"
	"strings"

	"github.com/ebitengine/purego"
)

// systemSonames lists the SONAMEs a distribution's libmpv can have, in search
// order. The SONAME only changes when the client ABI breaks: mpv 0.36 and
// newer (0.41 on Arch) ship libmpv.so.2, the 0.35 series libmpv.so.1, older
// builds libmpv.so.0.
var systemSonames = []string{"libmpv.so.2", "libmpv.so.1", "libmpv.so.0"}

// dlopenSoname loads a library by SONAME, lazily like openLibrary. It is a
// variable so tests can record the attempts without a real libmpv installed.
var dlopenSoname = func(name string) (uintptr, error) {
	return purego.Dlopen(name, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
}

// sonameLibraryPath returns the first of names that the dynamic loader can
// resolve. Each hit is unloaded again: this only probes, Open loads the
// library for real. Returning on the first hit is the point: a machine with
// libmpv.so.2 must never reach for libmpv.so.0. Both hooks are arguments so
// tests can drive the search order without a real libmpv.
func sonameLibraryPath(names []string, load func(string) (uintptr, error), unload func(uintptr) error) (string, error) {
	for _, name := range names {
		handle, err := load(name)
		if err != nil {
			continue
		}
		_ = unload(handle)
		return name, nil
	}
	return "", fmt.Errorf("找不到系统 libmpv（尝试过 %s）：请安装 mpv（Arch：sudo pacman -S mpv），或把 MPV_LIB 设为 libmpv.so 的完整路径",
		strings.Join(names, ", "))
}

// systemLibraryPath resolves the distribution's libmpv by SONAME. The dynamic
// loader owns the search path (ld.so.cache, ld.so.conf, LD_LIBRARY_PATH), so
// no absolute path is guessed here and ldconfig is never shelled out to.
func systemLibraryPath() (string, error) {
	return sonameLibraryPath(systemSonames, dlopenSoname, closeLibrary)
}

// acceptSoname reports whether path is a bare SONAME that only the dynamic
// loader can resolve. Everything else still goes through validateLibrary, so
// the macOS/Windows rule (absolute path to a regular file) stays intact.
func acceptSoname(path string) bool {
	for _, name := range systemSonames {
		if path == name {
			return true
		}
	}
	return false
}
