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
