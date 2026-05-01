package pointer

func NilIfZeroValue[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}
