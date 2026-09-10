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

func TestVerifyIntegrity_SnippetReferencesNeedNotExist(t *testing.T) {
	t.Parallel()

	manifest := apitype.ManifestV1{Version: "test"}
	manifest.Magic = manifest.NewMagic()
	snap := &apitype.DeploymentV3{
		Manifest: manifest,
		Snippets: []apitype.SnippetV1{
			{
				UUID: "e970a91d-4f4c-5793-8d27-dd27a0d96cf7",
				Name: "consumer",
				Type: "pkgA:index:res",
				Code: `propA = missing.id`,
				References: map[string]string{
					"missing": "urn:pulumi:stack::project::pkgA:index:res::missing",
				},
			},
		},
	}

	require.NoError(t, VerifyIntegrity(snap))
}

func TestVerifyIntegrity_SnippetUUID(t *testing.T) {
	t.Parallel()

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		manifest := apitype.ManifestV1{Version: "test"}
		manifest.Magic = manifest.NewMagic()
		snap := &apitype.DeploymentV3{
			Manifest: manifest,
			Snippets: []apitype.SnippetV1{
				{
					Name: "consumer",
					Type: "pkgA:index:res",
					Code: `propA = true`,
				},
			},
		}

		require.ErrorContains(t, VerifyIntegrity(snap), "snippet at index 0 missing required 'uuid' field")
	})

	t.Run("duplicate", func(t *testing.T) {
		t.Parallel()

		manifest := apitype.ManifestV1{Version: "test"}
		manifest.Magic = manifest.NewMagic()
		snap := &apitype.DeploymentV3{
			Manifest: manifest,
			Snippets: []apitype.SnippetV1{
				{
					UUID: "e970a91d-4f4c-5793-8d27-dd27a0d96cf7",
					Name: "consumer-a",
					Type: "pkgA:index:res",
					Code: `propA = true`,
				},
				{
					UUID: "e970a91d-4f4c-5793-8d27-dd27a0d96cf7",
					Name: "consumer-b",
					Type: "pkgA:index:res",
					Code: `propA = false`,
				},
			},
		}

		require.ErrorContains(t, VerifyIntegrity(snap),
			`duplicate snippet uuid "e970a91d-4f4c-5793-8d27-dd27a0d96cf7" at indexes 0 and 1`)
	})
}
