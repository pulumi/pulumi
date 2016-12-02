// Copyright 2016 Marapongo, Inc. All rights reserved.

package encoding

// ArrayOfStrings checks a weakly typed interface ptr to see if it's a []string; if yes, the resulting converted array
// is returned with a "true"; otherwise, nil with a "false" is returned.
func ArrayOfStrings(i interface{}) ([]string, bool) {
	// First try a direct conversion.
	if s, ok := i.([]string); ok {
		return s, true
	}

	// Otherwise, see if it's an untyped array, and then convert each element.
	if a, ok := i.([]interface{}); ok {
		ss := make([]string, 0, len(a))
		for _, e := range a {
			if s, ok := e.(string); ok {
				ss = append(ss, s)
			} else {
				return nil, false
			}
		}
		return ss, true
	}

	return nil, false
}
