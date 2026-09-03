package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// useColor toggles ANSI styling. It is off in --once/--watch pipe modes so
// output stays clean and greppable.
var useColor bool

// ANSI SGR escapes (256-color palette).
const (
	colReset   = "\x1b[0m"
	colBold    = "\x1b[1m"
	colDim     = "\x1b[2m"
	colReverse = "\x1b[7m"
	colUnder   = "\x1b[4m"
)

func fg(n uint8) string { return "\x1b[38;5;" + strconv.Itoa(int(n)) + "m" }
func bg(n uint8) string { return "\x1b[48;5;" + strconv.Itoa(int(n)) + "m" }

// Palette.
const (
	cGreen  uint8 = 78
	cYellow uint8 = 228
	cRed    uint8 = 203
	cCyan   uint8 = 117
	cGray   uint8 = 245
	cOrange uint8 = 215
)

// style wraps s in styling escapes (no-op when useColor is false).
func style(s, esc string) string {
	if !useColor || esc == "" {
		return s
	}
	return esc + s + colReset
}

// statusColor returns the color for a status/decision string.
func statusColor(s string) uint8 {
	switch s {
	case "COMPLETED", "RESOLVED", "RECONCILED", "MATCHED":
		return cGreen
	case "RUNNING", "REVIEW", "INVESTIGATING":
		return cYellow
	case "ESCALATED", "ESCALATE", "FAILED":
		return cRed
	case "MATCHING", "VERIFYING", "PENDING":
		return cCyan
	default:
		return cGray
	}
}

// pill renders a bracketed, color-coded status like "[COMPLETED]".
func pill(s string) string {
	if s == "" {
		return "—"
	}
	return style("["+s+"]", fg(statusColor(s)))
}

// displayWidth approximates the terminal columns a string occupies. East Asian
// wide characters count as two columns; everything else as one.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r > 0x2e7f {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// formatMoney renders integer paise as ₹ with Indian digit grouping. Integer
// math only — never a float, mirroring the control plane's money rule.
func formatMoney(minor int64) string {
	neg := minor < 0
	if neg {
		minor = -minor
	}
	whole := minor / 100
	frac := minor % 100
	sign := ""
	if neg {
		sign = "-"
	}
	return sign + "₹" + groupIndian(strconv.FormatInt(whole, 10)) + "." + fmt.Sprintf("%02d", frac)
}

// groupIndian applies 1,23,456-style Indian grouping to a digit string.
func groupIndian(digits string) string {
	n := len(digits)
	if n <= 3 {
		return digits
	}
	rest := digits[:n-3]
	last := digits[n-3:]
	var groups []string
	for i := len(rest); i > 0; i -= 2 {
		start := i - 2
		if start < 0 {
			start = 0
		}
		groups = append([]string{rest[start:i]}, groups...)
	}
	return strings.Join(groups, ",") + "," + last
}

// formatPct renders a 0..1 confidence as "92%".
func formatPct(v *float64) string {
	if v == nil {
		return "—"
	}
	return fmt.Sprintf("%.0f%%", *v*100)
}

// formatTime renders an ISO timestamp compactly for table cells.
func formatTime(t time.Time) string { return t.Local().Format("2006-01-02 15:04") }

// truncate shortens s to at most n display columns, appending "…" when cut.
func truncate(s string, n int) string {
	if n < 0 {
		n = 0
	}
	if displayWidth(s) <= n {
		return s
	}
	var b strings.Builder
	width := 0
	for _, r := range s {
		w := 1
		if r > 0x2e7f {
			w = 2
		}
		if width+w > n-1 { // reserve one column for the ellipsis
			break
		}
		b.WriteRune(r)
		width += w
	}
	return b.String() + "…"
}

// wrapText word-wraps s to at most width display columns, breaking on whole
// words (each paragraph's line breaks preserved). Tokens wider than width are
// hard-split so nothing is silently dropped. Used for long advisory narratives.
func wrapText(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var lines []string
	split := func(word string) { // emit a single over-long token in width-chunks
		var b strings.Builder
		bw := 0
		for _, r := range word {
			rw := 1
			if r > 0x2e7f {
				rw = 2
			}
			if bw > 0 && bw+rw > width {
				lines = append(lines, b.String())
				b.Reset()
				bw = 0
			}
			b.WriteRune(r)
			bw += rw
		}
		if bw > 0 {
			lines = append(lines, b.String())
		}
	}
	for _, para := range strings.Split(s, "\n") {
		if strings.TrimSpace(para) == "" {
			lines = append(lines, "")
			continue
		}
		var cur []string
		curW := 0
		for _, word := range strings.Fields(para) {
			wordW := displayWidth(word)
			if wordW > width { // can't fit on any line — hard-split
				if len(cur) > 0 {
					lines = append(lines, strings.Join(cur, " "))
					cur, curW = nil, 0
				}
				split(word)
				continue
			}
			sep := 0
			if curW > 0 {
				sep = 1
			}
			if curW+sep+wordW <= width {
				cur = append(cur, word)
				curW += sep + wordW
			} else {
				lines = append(lines, strings.Join(cur, " "))
				cur = []string{word}
				curW = wordW
			}
		}
		if len(cur) > 0 {
			lines = append(lines, strings.Join(cur, " "))
		}
	}
	return lines
}

// tableLines renders a table. right[i] right-aligns column i. sel is the
// highlighted row index (-1 for none); statusCol is colorized by value.
func tableLines(headers []string, right []bool, rows [][]string, sel, statusCol, width int) []string {
	nc := len(headers)
	if nc == 0 {
		return nil
	}
	widths := make([]int, nc)
	for i, h := range headers {
		widths[i] = displayWidth(h)
	}
	for _, r := range rows {
		for i := 0; i < nc && i < len(r); i++ {
			if w := displayWidth(r[i]); w > widths[i] {
				widths[i] = w
			}
		}
	}
	// Keep the table inside the terminal: shrink the last column first, then
	// the others, never below a readable minimum.
	gap := 2
	total := 0
	for i, w := range widths {
		total += w
		if i < nc-1 {
			total += gap
		}
	}
	if max := width - 2; max > 0 && total > max {
		shrink := total - max
		last := nc - 1
		if widths[last]-shrink >= 8 {
			widths[last] -= shrink
		} else {
			remaining := shrink
			for i := 0; i < nc && remaining > 0; i++ {
				if i == last || widths[i] <= 8 {
					continue
				}
				red := remaining
				if widths[i]-red < 8 {
					red = widths[i] - 8
				}
				widths[i] -= red
				remaining -= red
			}
		}
	}

	line := make([]byte, 0, 256)
	emit := func(cells []string, isSel bool) string {
		line = line[:0]
		for i := 0; i < nc; i++ {
			if i > 0 {
				line = append(line, ' ', ' ')
			}
			cell := ""
			if i < len(cells) {
				cell = truncate(cells[i], widths[i])
			}
			if isSel {
				cell = style(cell, colReverse)
			} else if i == statusCol && useColor && statusColor(cell) != cGray {
				cell = style(cell, fg(statusColor(cell)))
			}
			pad := widths[i] - displayWidth(cell)
			if pad < 0 {
				pad = 0
			}
			if right[i] {
				line = append(line, strings.Repeat(" ", pad)...)
				line = append(line, cell...)
			} else {
				line = append(line, cell...)
				line = append(line, strings.Repeat(" ", pad)...)
			}
		}
		return string(line)
	}

	out := []string{style(emit(headers, false), colUnder)}
	for idx, r := range rows {
		if sel >= 0 && idx == sel {
			out = append(out, "▸ "+emit(r, true))
		} else {
			out = append(out, "  "+emit(r, false))
		}
	}
	return out
}

// headerBar builds a full-width title bar: bold title on the left, dim status
// on the right, on a dark-blue ground.
func headerBar(title, right string, width int) string {
	fill := width - 2 - displayWidth(title) - displayWidth(right)
	if fill < 0 {
		fill = 0
	}
	body := " " + style(title, colBold+fg(cCyan)) + strings.Repeat(" ", fill) + style(right, fg(cGray)) + " "
	return style(body, bg(24))
}

// footerBar builds a full-width footer. The status message, when non-empty,
// appears first (and may carry its own color, e.g. red for errors).
func footerBar(hints, msg string, width int) string {
	content := hints
	if msg != "" {
		content = msg + "   ·   " + hints
	}
	if displayWidth(content) > width-2 {
		content = truncate(content, width-2)
	}
	fill := width - 2 - displayWidth(content)
	if fill < 0 {
		fill = 0
	}
	return style(" "+content+strings.Repeat(" ", fill)+" ", bg(235)+fg(cGray))
}

// sectionHeader renders a box-drawing section divider,
// e.g. "─ Evidence ─────────────────────────".
func sectionHeader(title string, width int) string {
	if title == "" {
		return style(strings.Repeat("─", width), colDim)
	}
	n := displayWidth(title) + 2
	side := (width - n) / 2
	if side < 1 {
		side = 1
	}
	l := strings.Repeat("─", side)
	r := strings.Repeat("─", width-n-side)
	return style(l+" "+title+" "+r, fg(cGray))
}

// padTo pads/truncates a line to exactly width columns.
func padTo(s string, width int) string {
	if w := displayWidth(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return truncate(s, width)
}
