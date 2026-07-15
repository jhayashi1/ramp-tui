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
		Delays:           []time.Duration{100 * time.Millisecond, 40 * time.Millisecond},
		SourceGIF:        []byte{'G', 'I', 'F', '8', '9', 'a', 0x01},
		Colored:          true,
		Complex:          true,
		FilterBackground: true,
		CustomRamp:       " .@",
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
	if !bytes.Equal(decoded.SourceGIF, original.SourceGIF) {
		t.Errorf("SourceGIF = %v, want %v", decoded.SourceGIF, original.SourceGIF)
	}
	if decoded.Colored != original.Colored || decoded.Complex != original.Complex ||
		decoded.FilterBackground != original.FilterBackground ||
		decoded.CustomRamp != original.CustomRamp {
		t.Errorf("options = (%v, %v, %v, %q), want (%v, %v, %v, %q)",
			decoded.Colored, decoded.Complex, decoded.FilterBackground, decoded.CustomRamp,
			original.Colored, original.Complex, original.FilterBackground, original.CustomRamp)
	}
}

// TestDecodeAcceptsVersion1 reconstructs a genuine v1 file: gob omits
// zero-valued fields, so encoding an animation without any v2 fields
// and patching the header version byte yields the v1 byte stream.
func TestDecodeAcceptsVersion1(t *testing.T) {
	anim := &Animation{
		SourceName: "old.gif",
		Width:      1,
		Height:     1,
		Frames:     []string{"x"},
		Delays:     []time.Duration{time.Millisecond},
	}
	var buf bytes.Buffer
	if err := Encode(&buf, anim); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	data := buf.Bytes()
	if data[8] != version {
		t.Fatalf("header version = %d, want %d", data[8], version)
	}
	data[8] = 1

	decoded, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode v1: %v", err)
	}
	if decoded.SourceName != "old.gif" || len(decoded.Frames) != 1 {
		t.Errorf("decoded v1 = %+v, want original content", decoded)
	}
	if decoded.SourceGIF != nil {
		t.Errorf("SourceGIF = %v, want nil for v1 files", decoded.SourceGIF)
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

func TestDecodeRejectsMismatchedLengths(t *testing.T) {
	anim := &Animation{
		Frames: []string{"a", "b"},
		Delays: []time.Duration{time.Millisecond},
	}
	var buf bytes.Buffer
	if err := Encode(&buf, anim); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	_, err := Decode(&buf)
	if err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("err = %v, want corrupt frames file error", err)
	}
}

func TestDecodeRejectsEmptyAnimation(t *testing.T) {
	var buf bytes.Buffer
	if err := Encode(&buf, &Animation{}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	_, err := Decode(&buf)
	if err == nil || !strings.Contains(err.Error(), "no frames") {
		t.Errorf("err = %v, want no frames error", err)
	}
}

func TestDecodeRejectsTruncated(t *testing.T) {
	if _, err := Decode(strings.NewReader("ASCII")); err == nil {
		t.Error("want error for truncated input, got nil")
	}
}
