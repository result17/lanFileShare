package util

import (
	"github.com/mattn/go-runewidth"
	"strings"
)

// PadRight pads or truncates a string to a fixed width.
func PadRight(str string, width int) string {
	w := runewidth.StringWidth(str)
	if w > width {
		return runewidth.Truncate(str, width, "...")
	}
	return str + strings.Repeat(" ", width-w)
}
