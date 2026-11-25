package extensions

import "fmt"

// FormatSize formats a number of bytes for humans, with binary units: 512 B,
// 1 KiB, 1.5 MiB, etc.
func FormatSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	value, unit := float64(n)/1024, 0
	for value >= 1024 && unit < 5 {
		value /= 1024
		unit++
	}
	return fmt.Sprintf("%.4g %ciB", value, "KMGTPE"[unit])
}
