package tui

import (
	"os"
	"path/filepath"
	"testing"
)

// fixtureGifDir builds a directory with a mix of gifs, a subdirectory,
// and files the completer should ignore.
func fixtureGifDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "clips"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"party.gif", "cat.GIF", "notes.txt", ".hidden.gif"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func suggestionNames(p pathInput) []string {
	names := make([]string, len(p.suggestions))
	for i, s := range p.suggestions {
		names[i] = s.name
	}
	return names
}

func TestPathInputListsDirsAndGifsOnly(t *testing.T) {
	dir := fixtureGifDir(t)
	p := newPathInput()
	p.input.SetValue(dir + string(os.PathSeparator))
	p.refresh()

	want := []string{"clips", "cat.GIF", "party.gif"}
	got := suggestionNames(p)
	if len(got) != len(want) {
		t.Fatalf("suggestions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("suggestion[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if !p.suggestions[0].dir {
		t.Error("clips not marked as directory")
	}
}

func TestPathInputFuzzyFilters(t *testing.T) {
	dir := fixtureGifDir(t)
	p := newPathInput()
	p.input.SetValue(filepath.Join(dir, "pty"))
	p.refresh()

	got := suggestionNames(p)
	if len(got) != 1 || got[0] != "party.gif" {
		t.Fatalf("fuzzy suggestions for \"pty\" = %v, want [party.gif]", got)
	}
	if len(p.suggestions[0].matches) == 0 {
		t.Error("fuzzy match has no highlighted indexes")
	}
}

func TestPathInputShowsHiddenWhenAskedFor(t *testing.T) {
	dir := fixtureGifDir(t)
	p := newPathInput()
	p.input.SetValue(filepath.Join(dir, ".h"))
	p.refresh()

	got := suggestionNames(p)
	if len(got) != 1 || got[0] != ".hidden.gif" {
		t.Fatalf("suggestions for \".h\" = %v, want [.hidden.gif]", got)
	}
}

func TestPathInputCompleteDescendsIntoDirectory(t *testing.T) {
	dir := fixtureGifDir(t)
	p := newPathInput()
	p.input.SetValue(filepath.Join(dir, "cl"))
	p.refresh()

	p.complete()
	want := filepath.Join(dir, "clips") + string(os.PathSeparator)
	if got := p.input.Value(); got != want {
		t.Fatalf("value after completing directory = %q, want %q", got, want)
	}
	if len(p.suggestions) != 0 {
		t.Errorf("expected empty suggestions inside empty dir, got %v", suggestionNames(p))
	}
}

func TestPathInputAcceptGif(t *testing.T) {
	dir := fixtureGifDir(t)
	p := newPathInput()
	p.input.SetValue(filepath.Join(dir, "party"))
	p.refresh()

	path, ok := p.accept()
	if !ok {
		t.Fatal("accept on gif suggestion returned ok=false")
	}
	if want := filepath.Join(dir, "party.gif"); path != want {
		t.Errorf("accepted path = %q, want %q", path, want)
	}
}

func TestPathInputAcceptDirectoryDescends(t *testing.T) {
	dir := fixtureGifDir(t)
	p := newPathInput()
	p.input.SetValue(filepath.Join(dir, "clips"))
	p.refresh()

	if _, ok := p.accept(); ok {
		t.Fatal("accept on directory suggestion returned ok=true")
	}
	want := filepath.Join(dir, "clips") + string(os.PathSeparator)
	if got := p.input.Value(); got != want {
		t.Errorf("value after accepting directory = %q, want %q", got, want)
	}
}

func TestPathInputTildeCompletion(t *testing.T) {
	home := fixtureGifDir(t)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	p := newPathInput()
	p.input.SetValue("~")
	p.refresh()

	got := suggestionNames(p)
	if len(got) != 3 {
		t.Fatalf("suggestions under ~ = %v, want 3 entries", got)
	}

	p.complete()
	want := "~" + string(os.PathSeparator) + "clips" + string(os.PathSeparator)
	if got := p.input.Value(); got != want {
		t.Errorf("value after completing under ~ = %q, want %q", got, want)
	}
}

func TestPathInputAcceptExpandsTypedTilde(t *testing.T) {
	home := t.TempDir() // empty: no suggestions, the literal text is used
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	p := newPathInput()
	p.input.SetValue("~/party.gif")
	p.refresh()

	path, ok := p.accept()
	if !ok {
		t.Fatal("accept on typed path returned ok=false")
	}
	if want := filepath.Join(home, "party.gif"); path != want {
		t.Errorf("accepted path = %q, want %q", path, want)
	}
}

func TestPathInputSelectionWraps(t *testing.T) {
	dir := fixtureGifDir(t)
	p := newPathInput()
	p.input.SetValue(dir + string(os.PathSeparator))
	p.refresh()

	p.moveSelection(-1)
	if p.sel != len(p.suggestions)-1 {
		t.Errorf("sel after up from top = %d, want %d", p.sel, len(p.suggestions)-1)
	}
	p.moveSelection(1)
	if p.sel != 0 {
		t.Errorf("sel after wrap down = %d, want 0", p.sel)
	}
}
