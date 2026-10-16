// Copyright 2025, Pulumi Corporation.
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

package spell

import (
	"cmp"
	"iter"
	"slices"
	"strings"
	"unicode"
)

// Nearest returns the element of candidates nearest to x using the Levenshtein metric, or "" if none were promising.
func Nearest[S ~string](x S, candidates iter.Seq[S]) S {
	// Ignore underscores and case when matching.
	x = fold(x)

	var best S
	bestD := (len(x) + 1) / 2 // allow up to 50% typos
	for c := range candidates {
		d := levenshtein(x, fold(c), &bestD)
		if d < bestD {
			bestD = d
			best = c
		}
	}
	return best
}

// SortByEditDistance sorts the list of candidates by edit distance from x.
func SortByEditDistance[S ~string](x S, candidates []S) {
	// Ignore underscores and case when matching.
	x = fold(x)

	slices.SortStableFunc(candidates, func(ca, cb S) int {
		return cmp.Compare(levenshtein(x, fold(ca), nil), levenshtein(x, fold(cb), nil))
	})
}

// levenshtein returns the non-negative Levenshtein edit distance between the byte strings x and y.
//
// If the computed distance exceeds max, the function may return early with an approximate value > max.
func levenshtein[S ~string](x, y S, max *int) int {
	// This implementation is derived from one by Laurent Le Brun in Bazel that uses the single-row space efficiency
	// trick described at bitbucket.org/clearer/iosifovich.

	// Let x be the shorter string.
	if len(x) > len(y) {
		x, y = y, x
	}

	// Remove common prefix.
	for i := 0; i < len(x); i++ {
		if x[i] != y[i] {
			x = x[i:]
			y = y[i:]
			break
		}
	}
	if x == "" {
		return len(y)
	}

	if max != nil {
		if d := abs(len(x) - len(y)); d > *max {
			return d // excessive length divergence
		}
	}

	row := make([]int, len(y)+1)
	for i := range row {
		row[i] = i
	}

	for i := 1; i <= len(x); i++ {
		row[0] = i
		best := i
		prev := i - 1
		for j := 1; j <= len(y); j++ {
			a := prev + b2i(x[i-1] != y[j-1]) // substitution
			b := 1 + row[j-1]                 // deletion
			c := 1 + row[j]                   // insertion
			k := min(a, min(b, c))
			prev, row[j] = row[j], k
			best = min(best, k)
		}
		if max != nil && best > *max {
			return best
		}
	}
	return row[len(y)]
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func abs(x int) int {
	if x >= 0 {
		return x
	}
	return -x
}

func fold[S ~string](s S) S {
	return S(strings.Map(func(r rune) rune {
		if r == '_' {
			return -1
		}
		return unicode.ToLower(r)
	}, string(s)))
}
