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

package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestRedactStatesForLog(t *testing.T) {
	t.Parallel()

	states := []*pkgresource.State{{
		URN:  "urn:pulumi:test::test::pkg:m:t::res",
		Type: "pkg:m:t",
		ID:   "id-1",
		Inputs: resource.PropertyMap{
			"plain":  resource.NewProperty("visible"),
			"secret": resource.MakeSecret(resource.NewProperty("s3cr3t")),
		},
	}}
	out := redactStatesForLog(states)
	assert.NotContains(t, out, "s3cr3t", "secret plaintext must not appear in the debug rendering")
	assert.Contains(t, out, "[secret]")
	assert.Contains(t, out, "visible")
	assert.Contains(t, out, "urn:pulumi:test::test::pkg:m:t::res")
}

func TestFinalStateMigrationSuccessors(t *testing.T) {
	t.Parallel()

	original := []apitype.ResourceV3{{URN: "urn:a"}}
	final := []apitype.ResourceV3{{URN: "urn:c"}}
	canonical, rewrite, err := finalStateMigrationSuccessors(original, final, map[resource.URN]resource.URN{
		"urn:a": "urn:b",
		"urn:b": "urn:c",
	})
	require.NoError(t, err)
	assert.Equal(t, map[resource.URN]resource.URN{"urn:a": "urn:c"}, canonical)
	assert.Equal(t, map[resource.URN]resource.URN{
		"urn:a": "urn:c",
		"urn:b": "urn:c",
	}, rewrite)
}
