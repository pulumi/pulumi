// Copyright 2016-2024, Pulumi Corporation.
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

package stack

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/graph/dotconv"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

// Tests the output of 'pulumi stack graph'
// under different conditions.
func TestStackGraphCmd(t *testing.T) {
	t.Parallel()

	t.Run("Single node graph", func(t *testing.T) {
		t.Parallel()
		snap := deploy.Snapshot{
			Resources: []*resource.State{
				{
					URN:  "urn:pulumi",
					Type: resource.RootStackType,
				},
			},
		}

		t.Run("Smoke test", func(t *testing.T) {
			t.Parallel()

			opts := graphCommandOptions{}
			dg := makeDependencyGraph(&snap, &opts)

			var outputBuf bytes.Buffer
			require.NoError(t, dotconv.Print(dg, &outputBuf, opts.dotFragment))

			dotOutput := outputBuf.String()

			require.Equal(t, `strict digraph {
    Resource0 [label="urn:pulumi"];
}
`, dotOutput)
		})

		t.Run("dot fragment is inserted", func(t *testing.T) {
			t.Parallel()

			opts := graphCommandOptions{
				dotFragment: "[node shape=rect]\n[edge penwidth=2]",
			}
			dg := makeDependencyGraph(&snap, &opts)

			var outputBuf bytes.Buffer
			require.NoError(t, dotconv.Print(dg, &outputBuf, opts.dotFragment))

			dotOutput := outputBuf.String()

			require.Equal(t, `strict digraph {
[node shape=rect]
[edge penwidth=2]
    Resource0 [label="urn:pulumi"];
}
`, dotOutput)
		})
	})

	t.Run("graph with parent and child", func(t *testing.T) {
		t.Parallel()
		provider := resource.URN("urn:pulumi:dev::pets::random::provider")
		parent := resource.URN("urn:pulumi:dev::pets::random:index/randomPet:RandomPet::parent")
		child := resource.URN("urn:pulumi:dev::pets::random:index/randomPet:RandomPet::child")

		snap := deploy.Snapshot{
			Resources: []*resource.State{
				{
					URN:  provider,
					ID:   "provider-id",
					Type: "pulumi:provider:random",
				},
				{
					URN:  parent,
					ID:   "parent-id",
					Type: "random:index/randomPet:RandomPet",
				},
				{
					URN:    child,
					ID:     "child-id",
					Type:   "random:index/randomPet:RandomPet",
					Parent: parent,
				},
			},
		}

		t.Run("With default options", func(t *testing.T) {
			t.Parallel()
			expectedMaxNode := 2

			opts := graphCommandOptions{}
			dg := makeDependencyGraph(&snap, &opts)

			var outputBuf bytes.Buffer
			require.NoError(t, dotconv.Print(dg, &outputBuf, opts.dotFragment))

			dotOutput := outputBuf.String()

			for i := 0; i <= expectedMaxNode; i++ {
				require.Contains(t, dotOutput, fmt.Sprintf("Resource%d [label=", i))
			}
			for i := 1; i <= 4; i++ {
				require.NotContains(t, dotOutput, fmt.Sprintf("Resource%d", expectedMaxNode+i))
			}

			require.Contains(t, dotOutput, " -> ")
		})

		t.Run("with shortNodeName flag", func(t *testing.T) {
			t.Parallel()
			expectedLabels := []string{
				"provider", "parent", "child",
			}

			opts := graphCommandOptions{
				shortNodeName: true,
			}
			dg := makeDependencyGraph(&snap, &opts)

			var outputBuf bytes.Buffer
			require.NoError(t, dotconv.Print(dg, &outputBuf, opts.dotFragment))

			dotOutput := outputBuf.String()

			for _, label := range expectedLabels {
				require.Contains(t, dotOutput, fmt.Sprintf("[label=\"%s\"]", label))
			}
		})
	})

	t.Run("output is deterministic", func(t *testing.T) {
		t.Parallel()

		// Create a snapshot with multiple resources and dependencies
		// that would expose non-determinism in map iteration
		resA := resource.URN("urn:pulumi:dev::test::pkg:mod:ResA::a")
		resB := resource.URN("urn:pulumi:dev::test::pkg:mod:ResB::b")
		resC := resource.URN("urn:pulumi:dev::test::pkg:mod:ResC::c")
		resD := resource.URN("urn:pulumi:dev::test::pkg:mod:ResD::d")

		snap := deploy.Snapshot{
			Resources: []*resource.State{
				{URN: resA, ID: "a-id", Type: "pkg:mod:ResA"},
				{URN: resB, ID: "b-id", Type: "pkg:mod:ResB", Dependencies: []resource.URN{resA}},
				{URN: resC, ID: "c-id", Type: "pkg:mod:ResC", Dependencies: []resource.URN{resA, resB}},
				{
					URN:          resD,
					ID:           "d-id",
					Type:         "pkg:mod:ResD",
					Dependencies: []resource.URN{resA},
					PropertyDependencies: map[resource.PropertyKey][]resource.URN{
						"propZ": {resA},
						"propA": {resA},
						"propM": {resA},
					},
				},
			},
		}

		opts := graphCommandOptions{shortNodeName: true}

		var firstOutput string
		for i := range 10 {
			dg := makeDependencyGraph(&snap, &opts)

			var outputBuf bytes.Buffer
			require.NoError(t, dotconv.Print(dg, &outputBuf, opts.dotFragment))

			output := outputBuf.String()
			if i == 0 {
				firstOutput = output
			} else {
				require.Equal(t, firstOutput, output,
					"graph output should be deterministic across multiple runs")
			}
		}

		require.Contains(t, firstOutput, `label = "propA, propM, propZ"`,
			"property dependency labels should be sorted alphabetically")
	})
}
