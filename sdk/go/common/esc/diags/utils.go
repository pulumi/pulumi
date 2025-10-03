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

	"github.com/pulumi/esc/internal/spell"
)

func sortByEditDistance(comparedTo string, words []string) []string {
	w := make([]string, len(words))
	copy(w, words)
	spell.SortByEditDistance(comparedTo, w)
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
