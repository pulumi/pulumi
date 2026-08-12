// Copyright 2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import "strings"

// JoinKey joins an object property key with the path to its parents, quoting and escaping appropriately.
func JoinKey(root, k string) string {
	if k == "" {
		return root
	}

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
