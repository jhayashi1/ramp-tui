package library

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jhayashi1/ascii-tui/internal/frames"
)

func sample(name string) *frames.Animation {
	return &frames.Animation{
		SourceName: name,
		Width:      1,
		Height:     1,
		Frames:     []string{"x"},
		Delays:     []time.Duration{time.Millisecond},
	}
}

func TestSaveDoesNotOverwriteExistingEntries(t *testing.T) {
	dir := t.TempDir()

	first, err := Save(dir, sample("cat.gif"))
	if err != nil {
		t.Fatalf("first Save: %v", err)
	}
	second, err := Save(dir, sample("cat.gif"))
	if err != nil {
		t.Fatalf("second Save: %v", err)
	}

	if first == second {
		t.Fatalf("second save reused path %s, want a unique name", first)
	}
	if !strings.HasSuffix(second, "cat-2.frames") {
		t.Errorf("second path = %s, want cat-2.frames suffix", second)
	}
	for _, path := range []string{first, second} {
		if _, err := Load(path); err != nil {
			t.Errorf("Load(%s): %v", path, err)
		}
	}
}

func TestRename(t *testing.T) {
	dir := t.TempDir()
	path, err := Save(dir, sample("cat.gif"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	newPath, err := Rename(path, "kitty")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if !strings.HasSuffix(newPath, "kitty.frames") {
		t.Errorf("new path = %s, want kitty.frames suffix", newPath)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("old path still exists after rename (stat err = %v)", err)
	}
	if _, err := Load(newPath); err != nil {
		t.Errorf("Load(%s): %v", newPath, err)
	}
}

func TestRenameKeepsDottedNames(t *testing.T) {
	dir := t.TempDir()
	path, err := Save(dir, sample("cat.gif"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Unlike Save, a typed name must not lose a fake "extension"; the
	// dot maps to a dash instead.
	newPath, err := Rename(path, "my.cat")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if !strings.HasSuffix(newPath, "my-cat.frames") {
		t.Errorf("new path = %s, want my-cat.frames suffix", newPath)
	}
}

func TestRenameRefusesCollision(t *testing.T) {
	dir := t.TempDir()
	first, err := Save(dir, sample("cat.gif"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	second, err := Save(dir, sample("dog.gif"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := Rename(second, "cat"); err == nil {
		t.Fatal("Rename onto existing entry succeeded, want error")
	}
	for _, path := range []string{first, second} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("entry %s disturbed by failed rename: %v", path, err)
		}
	}
}

func TestRenameRejectsEmptyNames(t *testing.T) {
	dir := t.TempDir()
	path, err := Save(dir, sample("cat.gif"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	for _, name := range []string{"", "   ", "///"} {
		if _, err := Rename(path, name); err == nil {
			t.Errorf("Rename(%q) succeeded, want error", name)
		}
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("entry disturbed by rejected rename: %v", err)
	}
}

func TestRenameToSameNameIsNoop(t *testing.T) {
	dir := t.TempDir()
	path, err := Save(dir, sample("cat.gif"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	newPath, err := Rename(path, "cat")
	if err != nil {
		t.Fatalf("Rename to same name: %v", err)
	}
	if newPath != path {
		t.Errorf("new path = %s, want unchanged %s", newPath, path)
	}
}
