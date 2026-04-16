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

package neo

import "math/rand"

// thinkingVerbs contains Pulumi-themed verbs for the thinking indicator.
var thinkingVerbs = []string{
	"Puluminating", "Cloudforming", "Driftifying", "Ephemerizing",
	"Stacking", "Reconcifying", "Planifesting", "Speculating",
	"Dreamforming", "Outputting", "Resourcifying", "Providering",
	"Previewizing", "Pipelining", "Summoning", "Materializing", "Crunching",
}

// pickThinkingVerb returns a thinking verb: 60% "Thinking", 40% random themed.
func pickThinkingVerb() string {
	if rand.Intn(5) < 3 { //nolint:gosec
		return "Thinking"
	}
	return thinkingVerbs[rand.Intn(len(thinkingVerbs))] //nolint:gosec
}
