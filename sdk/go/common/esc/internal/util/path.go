// Copyright 2023, Pulumi Corporation.

package util

import "strings"

// JoinKey joins an object property key with the path to its parents, quoting and escaping appropriately.
func JoinKey(root, k string) string {
	if !MustEscapeKey(k) {
		if root == "" {
			return k
		}
		return root + "." + k
	}

	var b strings.Builder
	b.WriteString(`["`)
	for _, r := range k {
		if r == '"' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteString(`"]`)
	return root + b.String()
}

// MustEscapeKey returns true if the given key needs to be escaped.
func MustEscapeKey(k string) bool {
	for i, r := range k {
		switch {
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '_':
			// OK
		case r >= '0' && r <= '9':
			if i == 0 {
				return true
			}
		default:
			return true
		}
	}
	return false
}
