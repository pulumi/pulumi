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

package state

import (
	"testing"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestResourceNotFoundError(t *testing.T) {
	t.Parallel()

	candidates := []resource.URN{
		"urn:pulumi:stk::proj::pkg:index:typ::my-bucket",
		"urn:pulumi:stk::proj::pkg:index:typ::something-else-entirely",
	}

	t.Run("suggests close matches", func(t *testing.T) {
		t.Parallel()

		err := resourceNotFoundError(candidates, "urn:pulumi:stk::proj::pkg:index:typ::my-bukcet")
		assert.ErrorContains(t, err, "No such resource")
		assert.ErrorContains(t, err, "Did you mean:")
		assert.ErrorContains(t, err, "urn:pulumi:stk::proj::pkg:index:typ::my-bucket")
		assert.NotContains(t, err.Error(), "something-else-entirely")
		assert.ErrorContains(t, err, "pulumi stack --show-urns")
		assert.ErrorContains(t, err, "pulumi stack export")
	})

	t.Run("no suggestions when nothing is close", func(t *testing.T) {
		t.Parallel()

		err := resourceNotFoundError(candidates, "urn:pulumi:other::other::other:index:other::unrelated")
		assert.ErrorContains(t, err, "No such resource")
		assert.NotContains(t, err.Error(), "Did you mean:")
		assert.ErrorContains(t, err, "pulumi stack --show-urns")
		assert.ErrorContains(t, err, "pulumi stack export")
	})

	t.Run("no candidates", func(t *testing.T) {
		t.Parallel()

		err := resourceNotFoundError(nil, "urn:pulumi:stk::proj::pkg:index:typ::res")
		assert.ErrorContains(t, err, "No such resource")
		assert.NotContains(t, err.Error(), "Did you mean:")
		assert.ErrorContains(t, err, "pulumi stack --show-urns")
	})

	t.Run("at most three suggestions, closest first", func(t *testing.T) {
		t.Parallel()

		many := []resource.URN{
			"urn:pulumi:stk::proj::pkg:index:typ::res-aaaa",
			"urn:pulumi:stk::proj::pkg:index:typ::res-abab",
			"urn:pulumi:stk::proj::pkg:index:typ::res-abbb",
			"urn:pulumi:stk::proj::pkg:index:typ::res-abba",
		}
		suggestions := similarURNs(many, "urn:pulumi:stk::proj::pkg:index:typ::res-aaab", 3)
		require.Len(t, suggestions, 3)
		assert.Equal(t, resource.URN("urn:pulumi:stk::proj::pkg:index:typ::res-aaaa"), suggestions[0])
	})

	t.Run("case-insensitive matching", func(t *testing.T) {
		t.Parallel()

		suggestions := similarURNs(candidates, "urn:pulumi:stk::proj::pkg:index:typ::MY-BUCKET", 3)
		assert.Equal(t, []resource.URN{"urn:pulumi:stk::proj::pkg:index:typ::my-bucket"}, suggestions)
	})
}

func TestSnapshotURNs(t *testing.T) {
	t.Parallel()

	t.Run("nil snapshot", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, snapshotURNs(nil))
	})

	t.Run("deduplicates same-URN states", func(t *testing.T) {
		t.Parallel()

		snap := &deploy.Snapshot{
			Resources: []*pkgresource.State{
				{URN: "urn:pulumi:stk::proj::pkg:index:typ::res"},
				{URN: "urn:pulumi:stk::proj::pkg:index:typ::res", Delete: true},
				{URN: "urn:pulumi:stk::proj::pkg:index:typ::other"},
			},
		}
		assert.Equal(t, []resource.URN{
			"urn:pulumi:stk::proj::pkg:index:typ::res",
			"urn:pulumi:stk::proj::pkg:index:typ::other",
		}, snapshotURNs(snap))
	})
}

func TestListURNsHint(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		"To list the resource URNs in the stack, run `pulumi stack --show-urns`; "+
			"to inspect the full state, run `pulumi stack export`.",
		listURNsHint(""))
	assert.Equal(t,
		"To list the resource URNs in the stack, run `pulumi stack --show-urns --stack org/proj/stk`; "+
			"to inspect the full state, run `pulumi stack export --stack org/proj/stk`.",
		listURNsHint("org/proj/stk"))
}
