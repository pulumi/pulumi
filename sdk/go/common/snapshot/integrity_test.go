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

package snapshot

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// makeValidManifest returns a ManifestV1 whose magic cookie matches Version, so VerifyIntegrity
// passes the magic check and can get to the snippet-reference validation.
func makeValidManifest() apitype.ManifestV1 {
	m := apitype.ManifestV1{Version: "test"}
	m.Magic = m.NewMagic()
	return m
}

// TestVerifyIntegrity_SnippetReferencesKnownURN is the positive case: a snippet whose References
// map points at a URN that exists in snap.Resources should pass integrity verification.
func TestVerifyIntegrity_SnippetReferencesKnownURN(t *testing.T) {
	t.Parallel()

	const knownURN = "urn:pulumi:stack::project::pkgA:index:res::target"
	snap := &apitype.DeploymentV3{
		Manifest: makeValidManifest(),
		Resources: []apitype.ResourceV3{
			{URN: knownURN, Type: "pkgA:index:res"},
		},
		Snippets: []apitype.SnippetV1{
			{
				Name: "consumer",
				Type: "pkgA:index:res",
				Code: `propA = target.id`,
				References: map[string]string{
					"target": knownURN,
				},
			},
		},
	}

	require.NoError(t, VerifyIntegrity(snap))
}

// TestVerifyIntegrity_SnippetReferencesUnknownURN is the negative case: a snippet that references
// a URN that is not present in snap.Resources should produce a snapshot integrity error naming the
// snippet, the bad URN, and the identifier through which it was referenced.
func TestVerifyIntegrity_SnippetReferencesUnknownURN(t *testing.T) {
	t.Parallel()

	const missingURN = "urn:pulumi:stack::project::pkgA:index:res::missing"
	snap := &apitype.DeploymentV3{
		Manifest: makeValidManifest(),
		// No resources at all — the snippet's reference is therefore dangling.
		Snippets: []apitype.SnippetV1{
			{
				Name: "consumer",
				Type: "pkgA:index:res",
				Code: `propA = ghost.id`,
				References: map[string]string{
					"ghost": missingURN,
				},
			},
		},
	}

	err := VerifyIntegrity(snap)
	require.Error(t, err)
	require.ErrorContains(t, err, "unknown URN")
	require.ErrorContains(t, err, missingURN)
	require.ErrorContains(t, err, `"ghost"`)
	// Should be reported as a snapshot integrity error so callers using AsSnapshotIntegrityError
	// can detect and react to it.
	_, ok := AsSnapshotIntegrityError(err)
	require.True(t, ok, "should be a SnapshotIntegrityError")
}
