package extensions

func Pointer[T any](val T) *T {
	return &val
}

func Contains[T comparable](slice []T, element T) bool {
	for _, v := range slice {
		if v == element {
			return true
		}
	}
	return false
}

func Equal[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func AppendIfMissing[T comparable](slice []T, elems ...T) []T {
	for _, elem := range elems {
		slice = AppendElementIfMissing(slice, elem)
	}
	return slice
}

func AppendElementIfMissing[T comparable](slice []T, elem T) []T {
	for _, v := range slice {
		if v == elem {
			return slice
		}
	}
	return append(slice, elem)
}

func AddToSet(s []string, e string) []string {
	if !Contains(s, e) {
		s = append(s, e)
	}
	return s
}
