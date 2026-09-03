// RAZE startup splash: a short animated wordmark shown when the interactive
// TUI opens. Purely decorative — any key skips it. Layout is stable across
// frames (still-hidden letters render as faint "ghosts" at full width), so the
// reveal never shifts the art around.
package main

import (
	"fmt"
	"strings"
	"time"
)

// introGlyphs holds the six-row RAZE wordmark in ANSI-Shadow box drawing. Every
// row is exactly 8 display columns so the four letters line up side by side.
var introGlyphs = [][]string{
	{ // R
		"██████╗ ",
		"██╔══██╗",
		"██████╔╝",
		"██╔══██╗",
		"██║  ██║",
		"╚═╝  ╚═╝",
	},
	{ // A
		" █████╗ ",
		"██╔══██╗",
		"███████║",
		"██╔══██║",
		"██║  ██║",
		"╚═╝  ╚═╝",
	},
	{ // Z
		"███████╗",
		"╚══███╔╝",
		"  ███╔╝ ",
		" ███╔╝  ",
		"███████╗",
		"╚══════╝",
	},
	{ // E
		"███████╗",
		"██╔════╝",
		"█████╗  ",
		"██╔══╝  ",
		"███████╗",
		"╚══════╝",
	},
}

// introColors tints each revealed letter with a distinct palette accent.
var introColors = [4]uint8{cCyan, cGreen, cYellow, cOrange}

const (
	introGlyphW  = 8                          // display columns of one letter
	introGap     = 2                          // spaces between letters
	introArtW    = 4*introGlyphW + 3*introGap // total wordmark width (38)
	introTagline = "AUTONOMOUS RECONCILIATION · RISK-AWARE CONTROL"
)

// introRow is one centered line: the styled text plus its display width BEFORE
// ANSI escapes are counted (escapes must not influence centering).
type introRow struct {
	text  string
	width int
}

// introRows builds the frame for elapsed seconds t. Timeline: letters reveal
// left→right every 0.3s from 0.15s (ghost → white pop → settled colour); the
// underline grows in after all four are in; the tagline types itself out.
func introRows(t float64) []introRow {
	rows := make([]introRow, 0, 10)
	for i := 0; i < 6; i++ {
		parts := make([]string, 4)
		for li, glyph := range introGlyphs {
			reveal := 0.15 + 0.30*float64(li)
			var esc string
			switch {
			case t < reveal:
				esc = fg(236) // ghost: faint outline holding the layout
			case t < reveal+0.14:
				esc = colBold + fg(231) // bright pop as it snaps in
			default:
				esc = fg(introColors[li])
			}
			parts[li] = style(glyph[i], esc)
		}
		rows = append(rows, introRow{strings.Join(parts, strings.Repeat(" ", introGap)), introArtW})
	}
	rows = append(rows, introRow{}, introRow{}) // gap below the wordmark

	uw := 0
	if t > 1.25 {
		frac := (t - 1.25) / 0.30
		if frac > 1 {
			frac = 1
		}
		uw = int(frac * introArtW)
	}
	rows = append(rows, introRow{style(strings.Repeat("─", uw), fg(cGray)), uw})
	rows = append(rows, introRow{}, introRow{}) // gap before the tagline

	tag := ""
	if t > 1.55 {
		n := int((t - 1.55) / 0.012)
		if n > len(introTagline) {
			n = len(introTagline)
		}
		tag = introTagline[:n]
		if n < len(introTagline) && int(t*2)%2 == 0 { // blinking caret while typing
			tag += "▌"
		}
	}
	rows = append(rows, introRow{style(tag, fg(cGray)), displayWidth(tag)})
	return rows
}

// drawIntroScreen clears the screen and draws one intro frame centred in a
// rows×cols terminal.
func drawIntroScreen(t float64, rows, cols int) {
	block := introRows(t)
	top := (rows - len(block)) / 2
	if top < 0 {
		top = 0
	}
	var sb strings.Builder
	sb.WriteString("\x1b[2J\x1b[H")
	for i := 0; i < rows; i++ {
		if i >= top && i < top+len(block) {
			r := block[i-top]
			pad := (cols - r.width) / 2
			if pad < 0 {
				pad = 0
			}
			sb.WriteString(strings.Repeat(" ", pad))
			sb.WriteString(r.text)
		}
		sb.WriteString("\r\n")
	}
	fmt.Print(sb.String())
}

// playIntro animates the RAZE wordmark for about 2.4s, or until the user
// presses any key. A key pressed here is handed to the main loop as a preKey so
// e.g. 'q' during the splash still quits. Skipped entirely on small terminals.
func (a *app) playIntro() {
	rows, cols := termSize()
	if rows < 12 || cols < introArtW+8 {
		return
	}
	start := time.Now()
	for time.Since(start) < 2400*time.Millisecond {
		select {
		case k := <-a.keys:
			cp := k
			a.preKey = &cp
			return
		default:
		}
		drawIntroScreen(time.Since(start).Seconds(), rows, cols)
		time.Sleep(33 * time.Millisecond)
	}
}
