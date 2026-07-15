package library

import (
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
