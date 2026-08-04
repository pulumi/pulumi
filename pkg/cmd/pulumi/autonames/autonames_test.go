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

package autonames

import (
	"testing"

	"github.com/stretchr/testify/require"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestResourceNames_PrefersBareName(t *testing.T) {
	t.Parallel()
	bucket := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::myBucket")
	names := ResourceNames(&deploy.Snapshot{
		Resources: []*pkgresource.State{
			{Type: bucket.Type(), URN: bucket, Custom: true},
		},
	})
	require.Equal(t, map[string]string{"myBucket": string(bucket)}, names)
}

func TestResourceNames_SkipsProvidersAndStackAndDeletes(t *testing.T) {
	t.Parallel()
	stack := resource.URN("urn:pulumi:dev::proj::pulumi:pulumi:Stack::proj-dev")
	provider := resource.URN("urn:pulumi:dev::proj::pulumi:providers:aws::default_1_2_3")
	bucket := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::myBucket")
	tombstone := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::deleted")
	names := ResourceNames(&deploy.Snapshot{
		Resources: []*pkgresource.State{
			{Type: stack.Type(), URN: stack},
			{Type: provider.Type(), URN: provider, Custom: true},
			{Type: bucket.Type(), URN: bucket, Custom: true},
			{Type: tombstone.Type(), URN: tombstone, Custom: true, Delete: true},
		},
	})
	require.Equal(t, map[string]string{"myBucket": string(bucket)}, names)
}

func TestResourceNames_DisambiguatesConflicts(t *testing.T) {
	t.Parallel()
	// Two resources with the same URN.Name() but different types — the first (in URN order)
	// takes the bare name, the second falls through to the type-qualified candidate.
	a := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::shared")
	b := resource.URN("urn:pulumi:dev::proj::aws:ec2/vpc:Vpc::shared")
	names := ResourceNames(&deploy.Snapshot{
		Resources: []*pkgresource.State{
			{Type: a.Type(), URN: a, Custom: true},
			{Type: b.Type(), URN: b, Custom: true},
		},
	})
	// URN order: "aws:ec2..." < "aws:s3...", so Vpc wins the bare name.
	require.Equal(t, string(b), names["shared"])
	require.Equal(t, string(a), names["Bucket_shared"])
	require.Len(t, names, 2)
}

func TestResourceNames_SanitizesInvalidIdentifierChars(t *testing.T) {
	t.Parallel()
	// A URN name with characters that aren't valid in a PCL identifier.
	u := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket.v2")
	names := ResourceNames(&deploy.Snapshot{
		Resources: []*pkgresource.State{{Type: u.Type(), URN: u, Custom: true}},
	})
	require.Equal(t, map[string]string{"my_bucket_v2": string(u)}, names)
}

func TestResourceNames_StableUnderUnrelatedInsertion(t *testing.T) {
	t.Parallel()
	a := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::a")
	b := resource.URN("urn:pulumi:dev::proj::aws:ec2/vpc:Vpc::b")
	before := ResourceNames(&deploy.Snapshot{
		Resources: []*pkgresource.State{
			{Type: a.Type(), URN: a, Custom: true},
		},
	})
	after := ResourceNames(&deploy.Snapshot{
		Resources: []*pkgresource.State{
			{Type: a.Type(), URN: a, Custom: true},
			{Type: b.Type(), URN: b, Custom: true},
		},
	})
	require.Equal(t, before["a"], after["a"], "existing identifier must not shift when unrelated resource is added")
}

func TestMerge_UserWins(t *testing.T) {
	t.Parallel()
	auto := map[string]string{"foo": "urn:auto:foo", "bar": "urn:auto:bar"}
	user := map[string]string{"foo": "urn:user:foo", "baz": "urn:user:baz"}
	got := Merge(auto, user)
	require.Equal(t, map[string]string{
		"foo": "urn:user:foo",
		"bar": "urn:auto:bar",
		"baz": "urn:user:baz",
	}, got)
}

func TestSanitizeIdent(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":                "",
		"abc":             "abc",
		"a-b.c":           "a_b_c",
		"123abc":          "_123abc",
		"__underscore_ok": "__underscore_ok",
	}
	for in, want := range cases {
		require.Equal(t, want, SanitizeIdent(in), "SanitizeIdent(%q)", in)
	}
}
