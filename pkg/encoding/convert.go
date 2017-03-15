// Copyright 2017 Pulumi, Inc. All rights reserved.

package encoding

// StringSlice checks a weakly typed interface ptr to see if it's a []string; if yes, the resulting converted array
// is returned with a "true"; otherwise, nil with a "false" is returned.  A copy may be made if needed.
func StringSlice(i interface{}) ([]string, bool) {
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

// StringStringMap checks a weakly typed interface ptr to see if it's a map[string]string; if yes, the result is
// returned with a "true"; otherwise, nil with a "false" is returned.  A copy may be made if needed.
func StringStringMap(i interface{}) (map[string]string, bool) {
	// First try a direct conversion.
	if ssm, ok := i.(map[string]string); ok {
		return ssm, true
	}

	// Otherwise, see if the keys are strings but the value type itself is unknown.
	if sim, ok := i.(map[string]interface{}); ok {
		ssm := make(map[string]string)
		for k, v := range sim {
			if s, ok := v.(string); ok {
				ssm[k] = s
			} else {
				return nil, false
			}
		}
		return ssm, true
	}

	return nil, false
}
