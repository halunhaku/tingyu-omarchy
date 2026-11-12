//go:build linux

package mpv

import (
	"errors"
	"strings"
	"testing"
)

// fakeLoader records every SONAME the resolver asks for and resolves only the
// ones in resolves. It keeps the tests independent of what this machine has
// installed: no real libmpv is touched.
type fakeLoader struct {
	resolves map[string]bool
	tried    []string
	unloaded []uintptr
}

func (f *fakeLoader) unload(handle uintptr) error {
	f.unloaded = append(f.unloaded, handle)
	return nil
}

func (f *fakeLoader) load(name string) (uintptr, error) {
	f.tried = append(f.tried, name)
	if f.resolves[name] {
		return 1, nil
	}
	return 0, errors.New("not found")
}

func TestSonameLibraryPathTriesEveryNameInOrder(t *testing.T) {
	loader := &fakeLoader{}
	path, err := sonameLibraryPath(systemSonames, loader.load, loader.unload)
	if err == nil {
		t.Fatalf("expected an error when no SONAME resolves, got path %q", path)
	}
	if path != "" {
		t.Fatalf("expected no path, got %q", path)
	}
	want := []string{"libmpv.so.2", "libmpv.so.1", "libmpv.so.0"}
	if len(loader.tried) != len(want) {
		t.Fatalf("tried %v, want %v", loader.tried, want)
	}
	for i, name := range want {
		if loader.tried[i] != name {
			t.Fatalf("attempt %d was %q, want %q (full order: %v)", i, loader.tried[i], name, loader.tried)
		}
	}
}

func TestSonameLibraryPathStopsAtFirstHit(t *testing.T) {
	loader := &fakeLoader{resolves: map[string]bool{"libmpv.so.1": true}}
	path, err := sonameLibraryPath(systemSonames, loader.load, loader.unload)
	if err != nil {
		t.Fatalf("expected libmpv.so.1 to resolve, got %v", err)
	}
	if path != "libmpv.so.1" {
		t.Fatalf("got %q, want libmpv.so.1", path)
	}
	want := []string{"libmpv.so.2", "libmpv.so.1"}
	if strings.Join(loader.tried, ",") != strings.Join(want, ",") {
		t.Fatalf("tried %v, want %v (libmpv.so.0 must not be probed)", loader.tried, want)
	}
	if len(loader.unloaded) != 1 {
		t.Fatalf("probe handles unloaded %d times, want 1", len(loader.unloaded))
	}
}

func TestSonameLibraryPathErrorNamesEveryAttempt(t *testing.T) {
	loader := &fakeLoader{}
	_, err := sonameLibraryPath(systemSonames, loader.load, loader.unload)
	if err == nil {
		t.Fatal("expected an error")
	}
	message := err.Error()
	for _, name := range systemSonames {
		if !strings.Contains(message, name) {
			t.Errorf("error %q does not name %s", message, name)
		}
	}
	for _, hint := range []string{"Arch", "pacman -S mpv", "MPV_LIB"} {
		if !strings.Contains(message, hint) {
			t.Errorf("error %q does not mention %q", message, hint)
		}
	}
}

// systemLibraryPath must go through the injected loader, not a real dlopen, so
// the default candidate list and its order are covered too.
func TestSystemLibraryPathUsesInjectedLoader(t *testing.T) {
	original := dlopenSoname
	t.Cleanup(func() { dlopenSoname = original })
	loader := &fakeLoader{}
	dlopenSoname = loader.load
	if path, err := systemLibraryPath(); err == nil || path != "" {
		t.Fatalf("expected failure with no loadable SONAME, got %q, %v", path, err)
	}
	if strings.Join(loader.tried, ",") != strings.Join(systemSonames, ",") {
		t.Fatalf("tried %v, want %v", loader.tried, systemSonames)
	}
	if len(loader.unloaded) != 0 {
		t.Fatalf("unloaded %v, want nothing: no probe succeeded", loader.unloaded)
	}
}

func TestAcceptSoname(t *testing.T) {
	for _, name := range systemSonames {
		if !acceptSoname(name) {
			t.Errorf("acceptSoname(%q) = false, want true", name)
		}
	}
	for _, path := range []string{
		"",
		"/usr/lib/libmpv.so.2",
		"/usr/lib/libmpv.so",
		"libmpv.so",
		"libmpv-2.dll",
		"../libmpv.so.2",
	} {
		if acceptSoname(path) {
			t.Errorf("acceptSoname(%q) = true, want false: only bare SONAMEs skip validateLibrary", path)
		}
	}
}
