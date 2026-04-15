package size

// BytesToMB converts a byte count to megabytes using a bitwise shift.
func BytesToMB(b int64) int64 {
	return b >> 20
}
