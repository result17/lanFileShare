package util

import (
	"github.com/mattn/go-runewidth"
	"strings"
)

func init() {
    // Disable special handling for East Asian ambiguous width characters.
    // This ensures that the width of characters like 'é' is calculated as 1 in any environment.
    runewidth.DefaultCondition.EastAsianWidth = false
}

// PadRight pads or truncates a string to a fixed width.
func PadRight(str string, width int) string {
	w := runewidth.StringWidth(str)
	if w > width {
		return runewidth.Truncate(str, width, "...")
	}
	return str + strings.Repeat(" ", width-w)
}
