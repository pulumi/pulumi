// Copyright 2016-2022, Pulumi Corporation.
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

package myers

import (
	"strings"
	"testing"

	"github.com/hexops/gotextdiff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestBasicEdits(t *testing.T) {
	olds := strings.Repeat("x", 20)
	news := strings.Repeat("x", 15) + strings.Repeat("y", 5)
	edits, err := MyersComputeEdits(strings.NewReader(olds), strings.NewReader(news), 4)
	require.NoError(t, err)
	editSize := 0
	for _, e := range edits {
		editSize += len(e.NewText)
	}
	t.Logf("editSize: %v", editSize)
	assert.Less(t, editSize, len(news))
}

func TestLargeInserts(t *testing.T) {
	olds := strings.Repeat("x", 40)
	news := strings.Repeat("x", 15) + strings.Repeat("y", 20) + strings.Repeat("x", 100)
	edits, err := MyersComputeEdits(strings.NewReader(olds), strings.NewReader(news), 4)
	require.NoError(t, err)
	editSize := 0
	for _, e := range edits {
		editSize += len(e.NewText)
	}
	t.Log(edits)
	assert.Less(t, editSize, 100)
}

func TestRapid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := rapid.StringMatching("[xy\n]{0,12}")
		olds := g.Draw(t, "olds").(string)
		news := g.Draw(t, "news").(string)

		edits, err := MyersComputeEdits(strings.NewReader(olds), strings.NewReader(news), 4)
		if err != nil {
			t.Fatalf("MyersComputeEdits errored: %v", err)
		}

		actual := gotextdiff.ApplyEdits(olds, edits)
		if actual != news {
			t.Fatalf("Reapplying edits produced %q instead of %q", actual, news)
		}
	})
}
