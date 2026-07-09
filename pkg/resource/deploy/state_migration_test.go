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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestRedactStatesForLog(t *testing.T) {
	t.Parallel()

	states := []*resource.State{{
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

func TestSummarizeStateMigration(t *testing.T) {
	t.Parallel()

	custom := func(urn resource.URN, typ string, id resource.ID) *resource.State {
		return &resource.State{URN: urn, Type: tokens.Type(typ), Custom: true, ID: id}
	}

	t.Run("rename preserves identity", func(t *testing.T) {
		t.Parallel()
		members := []*resource.State{custom("urn:a", "pkg:m:t", "id-1")}
		migrated := []*resource.State{custom("urn:b", "pkg:m:t", "id-1")}
		s := SummarizeStateMigration(members, migrated)
		assert.Equal(t, []resource.URN{"urn:b"}, s.Added)
		assert.Equal(t, []resource.URN{"urn:a"}, s.Removed)
		// The ID survives under the new URN, so it is not unmanaged.
		assert.Empty(t, s.Unmanaged)
	})

	t.Run("forget leaves resource unmanaged", func(t *testing.T) {
		t.Parallel()
		members := []*resource.State{custom("urn:a", "pkg:m:t", "id-1")}
		migrated := []*resource.State{}
		s := SummarizeStateMigration(members, migrated)
		assert.Equal(t, []resource.URN{"urn:a"}, s.Removed)
		assert.Equal(t, []resource.URN{"urn:a"}, s.Unmanaged)
	})

	t.Run("ID collision across types does not mask a forget", func(t *testing.T) {
		t.Parallel()
		// Two resources of different types share the ID "default" — common for name-derived IDs.
		members := []*resource.State{
			custom("urn:x", "aws:iam:RolePolicy", "default"),
			custom("urn:y", "gcp:sql:Database", "default"),
		}
		// Forget x; keep y. Because the shared ID belongs to a different type, x is genuinely unmanaged.
		migrated := []*resource.State{custom("urn:y", "gcp:sql:Database", "default")}
		s := SummarizeStateMigration(members, migrated)
		assert.Equal(t, []resource.URN{"urn:x"}, s.Removed)
		assert.Equal(t, []resource.URN{"urn:x"}, s.Unmanaged,
			"a forget must not be masked by an unrelated resource of a different type sharing the ID")
	})
}
