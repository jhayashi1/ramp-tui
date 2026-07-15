package frames

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestRoundtrip(t *testing.T) {
	original := &Animation{
		SourceName: "giphy.gif",
		Width:      3,
		Height:     2,
		Frames: []string{
			"\x1b[38;2;255;0;0m@#.\x1b[0m\nabc",
			"héllo\n→ ünïcode",
		},
		Delays: []time.Duration{100 * time.Millisecond, 40 * time.Millisecond},
	}

	var buf bytes.Buffer
	if err := Encode(&buf, original); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(&buf)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if decoded.SourceName != original.SourceName {
		t.Errorf("SourceName = %q, want %q", decoded.SourceName, original.SourceName)
	}
	if decoded.Width != original.Width || decoded.Height != original.Height {
		t.Errorf("dims = %dx%d, want %dx%d", decoded.Width, decoded.Height, original.Width, original.Height)
	}
	if len(decoded.Frames) != len(original.Frames) {
		t.Fatalf("frame count = %d, want %d", len(decoded.Frames), len(original.Frames))
	}
	for i := range original.Frames {
		if decoded.Frames[i] != original.Frames[i] {
			t.Errorf("frame %d = %q, want %q", i, decoded.Frames[i], original.Frames[i])
		}
		if decoded.Delays[i] != original.Delays[i] {
			t.Errorf("delay %d = %v, want %v", i, decoded.Delays[i], original.Delays[i])
		}
	}
}

func TestDecodeRejectsBadMagic(t *testing.T) {
	_, err := Decode(strings.NewReader("NOTVALIDxxxxxxxx"))
	if err == nil || !strings.Contains(err.Error(), "not an ascii-tui frames file") {
		t.Errorf("err = %v, want bad magic error", err)
	}
}

func TestDecodeRejectsUnknownVersion(t *testing.T) {
	data := append([]byte("ASCIITUI"), 99)
	_, err := Decode(bytes.NewReader(data))
	if err == nil || !strings.Contains(err.Error(), "unsupported frames file version") {
		t.Errorf("err = %v, want version error", err)
	}
}

func TestDecodeRejectsTruncated(t *testing.T) {
	if _, err := Decode(strings.NewReader("ASCII")); err == nil {
		t.Error("want error for truncated input, got nil")
	}
}
