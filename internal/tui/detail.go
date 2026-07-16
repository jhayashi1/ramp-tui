package tui

import (
	"fmt"
	"strconv"
	"strings"
)

// renderDetail builds the detail column's interior: labeled metadata
// rows for the selected entry, then a FILE section with on-disk facts.
// The surrounding renderColumn pads/clips rows to the column box.
func renderDetail(meta entryMeta, width int, st styles) string {
	rows := []string{
		kvRow("name", meta.name, width, st),
		kvRow("size", fmt.Sprintf("%dx%d", meta.width, meta.height), width, st),
		kvRow("frames", strconv.Itoa(meta.frames), width, st),
		kvRow("length", formatDuration(meta.length), width, st),
		kvRow("source", meta.source, width, st),
		kvRow("filter", onOff(meta.filter), width, st),
		kvRow("colored", yesNo(meta.colored), width, st),
		kvRow("complex", yesNo(meta.complex), width, st),
	}
	if meta.ramp != "" {
		rows = append(rows, kvRow("ramp", meta.ramp, width, st))
	}

	rows = append(rows, "", sectionRule("file", width, st))
	rows = append(rows, wrapDim(meta.path, width, 3, st)...)
	bytesText, modifiedText := "-", "-"
	if meta.fileSize > 0 {
		bytesText = formatBytes(meta.fileSize)
	}
	if !meta.modTime.IsZero() {
		modifiedText = meta.modTime.Format("2006-01-02 15:04")
	}
	rows = append(rows,
		kvRow("bytes", bytesText, width, st),
		kvRow("modified", modifiedText, width, st),
	)
	return strings.Join(rows, "\n")
}

// wrapDim hard-wraps s into at most maxLines dim-styled lines of width
// columns; anything longer is cut.
func wrapDim(s string, width, maxLines int, st styles) []string {
	if width < 1 {
		return nil
	}
	var lines []string
	for s != "" && len(lines) < maxLines {
		line := truncateLabel(s, width)
		if line == "" {
			break
		}
		lines = append(lines, st.dim.Render(line))
		s = s[len(line):]
	}
	return lines
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
