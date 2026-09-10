// Copyright 2026, Pulumi Corporation.
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

// Package editdistance measures how far apart two strings are in terms of
// single-character edits, for use in "did you mean" style suggestions.
//
// Plain Levenshtein distance counts an adjacent swap such as "lsit" for "list"
// as two edits, which makes the most common typo shape look as far away as
// two unrelated mistakes. The optimal string alignment distance implemented
// here counts that swap as one edit, so tight thresholds that keep false
// positives out of suggestions still catch transposition typos.
package editdistance

// OSA returns the optimal-string-alignment distance between a and b: the
// minimum number of single-rune insertions, deletions, substitutions, and
// adjacent transpositions needed to turn one into the other.
//
// The comparison is case-sensitive.
func OSA(a, b string) int {
	ra, rb := []rune(a), []rune(b)

	// Dynamic programming over prefix pairs: each cell takes the cheapest way an
	// edit sequence could end — delete a's last rune, insert b's, match or
	// substitute the last runes, or, when the prefixes end in the same two runes
	// swapped, transpose them. The transposition case reads two rows up, hence
	// the full matrix rather than the usual two rows.
	//
	// dist[i][j] is the edit distance between the first i runes of a and the
	// first j runes of b, so dist[i][0] = i (delete everything) and
	// dist[0][j] = j (insert everything).
	dist := make([][]int, len(ra)+1)
	for i := range dist {
		dist[i] = make([]int, len(rb)+1)
		dist[i][0] = i
	}
	for j := 1; j <= len(rb); j++ {
		dist[0][j] = j
	}

	for i := 1; i <= len(ra); i++ {
		for j := 1; j <= len(rb); j++ {
			deletion := dist[i-1][j] + 1
			insertion := dist[i][j-1] + 1
			substitution := dist[i-1][j-1]
			if ra[i-1] != rb[j-1] {
				substitution++
			}
			dist[i][j] = min(deletion, insertion, substitution)
			// The last two runes of each prefix appear swapped: one
			// transposition on top of whatever preceded them.
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				dist[i][j] = min(dist[i][j], dist[i-2][j-2]+1)
			}
		}
	}
	return dist[len(ra)][len(rb)]
}
