package util

import (
	"fmt"
	"math"
)

func FormatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	// Use integer arithmetic to avoid floating-point precision issues
	exp := int(math.Log(float64(size)) / math.Log(unit))
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}

	// Handle cases with very large units
	if exp >= len(units) {
		exp = len(units) - 1
	}

	// Calculate the value using integer division
	div := int64(math.Pow(unit, float64(exp)))
	value := size / div

	// Special case: omit decimals for integer values
	if size%div == 0 {
		return fmt.Sprintf("%d %s", value, units[exp])
	}

	// Calculate the decimal part (avoiding floating-point arithmetic)
	remainder := size % div
	decimal := (remainder * 1000) / div // Calculate three decimal places

	// Determine precision based on the decimal part
	switch {
	case decimal%10 != 0:
		return fmt.Sprintf("%d.%03d %s", value, decimal, units[exp])
	case decimal%100 != 0:
		return fmt.Sprintf("%d.%02d %s", value, decimal/10, units[exp])
	default:
		return fmt.Sprintf("%d.%d %s", value, decimal/100, units[exp])
	}
}
