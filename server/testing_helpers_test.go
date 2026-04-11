package server

// ptr is a test helper for creating pointers to literal values.
func ptr[T any](v T) *T {
	return &v
}
