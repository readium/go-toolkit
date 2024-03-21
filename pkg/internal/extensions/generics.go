package extensions

import "encoding/json"

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

func DeduplicateAndMarshalJSON[T any](s []T) ([]json.RawMessage, error) {
	if len(s) == 0 {
		// Shortcut if slice is empty
		return []json.RawMessage{}, nil
	}
	if len(s) == 1 {
		// Shortcut if only one element in slice
		bin, err := json.Marshal(s[0])
		if err != nil {
			return nil, err
		}
		return []json.RawMessage{bin}, nil
	}
	output := make([]json.RawMessage, 0, len(s))
	seen := make(map[string]struct{}, len(s))
	for _, v := range s {
		bin, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		str := string(bin)

		if _, ok := seen[str]; ok {
			continue
		}
		seen[str] = struct{}{}
		output = append(output, bin)
	}
	return output, nil
}
