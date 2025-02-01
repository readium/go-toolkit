// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package uritemplates

// Expand parses then expands a URI template with a set of values to produce
// the resultant URI. Two forms of the result are returned: one with all the
// elements escaped, and one with the elements unescaped.
func Expand(path string, values map[string]string) (escaped, unescaped string, err error) {
	template, err := parse(path)
	if err != nil {
		return "", "", err
	}
	escaped, unescaped = template.Expand(values)
	return escaped, unescaped, nil
}

// Values returns the names of the variables in the URI template.
func Values(path string) ([]string, error) {
	template, err := parse(path)
	if err != nil {
		return []string{}, err
	}
	parts := []string{}
	for _, part := range template.parts {
		for _, term := range part.terms {
			parts = append(parts, term.name)
		}
	}
	return parts, nil
}
