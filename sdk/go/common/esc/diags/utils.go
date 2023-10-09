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

package diags

import (
	"fmt"
	"sort"
)

// editDistance calculates the Levenshtein distance between words a and b.
func editDistance(a, b string) int {
	// Algorithm taken from https://en.wikipedia.org/wiki/Levenshtein_distance
	d := make([][]int, len(a)+1)
	for i := range d {
		d[i] = make([]int, len(b)+1)
	}
	for i := 0; i < len(a)+1; i++ {
		d[i][0] = i
	}
	for j := 0; j < len(b)+1; j++ {
		d[0][j] = j
	}

	min := func(i, j int) int {
		if i < j {
			return i
		}
		return j
	}
	for i := 1; i < len(a)+1; i++ {
		for j := 1; j < len(b)+1; j++ {
			var subCost int
			if a[i-1] != b[j-1] {
				subCost = 1
			}
			d[i][j] = min(d[i-1][j]+1, // deletion
				min(d[i][j-1]+1, // insertion
					d[i-1][j-1]+subCost), // substitution
			)
		}
	}
	return d[len(a)][len(b)]
}

func sortByEditDistance(words []string, comparedTo string) []string {
	w := make([]string, len(words))
	copy(w, words)
	m := map[string]int{}
	v := func(s string) int {
		d, ok := m[s]
		if !ok {
			d = editDistance(s, comparedTo)
			m[s] = d
		}
		return d
	}
	sort.Strings(w)
	sort.SliceStable(w, func(i, j int) bool {
		return v(w[i]) < v(w[j])
	})
	return w
}

// A list that displays in the human readable format: "a, b and c".
type AndList []string

func (h AndList) String() string {
	return displayList(h, "and")
}

// A list that displays in the human readable format: "a, b or c".
type OrList []string

func (h OrList) String() string {
	return displayList(h, "or")
}

func displayList(h []string, conjuctor string) string {
	switch len(h) {
	case 0:
		return ""
	case 1:
		return h[0]
	case 2:
		return fmt.Sprintf("%s %s %s", h[0], conjuctor, h[1])
	default:
		return fmt.Sprintf("%s, %s", h[0], displayList(h[1:], conjuctor))
	}
}
