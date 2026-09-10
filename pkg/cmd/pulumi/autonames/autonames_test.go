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

func TestResourceNames_PrefersPlainSnippetName(t *testing.T) {
	t.Parallel()
	bucket := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::myBucket")
	names := ResourceNames(&deploy.Snapshot{
		Snippets: []resource.Snippet{{
			UUID: "snippet-1",
			Name: "plainBucket",
			Type: string(bucket.Type()),
		}},
		Resources: []*pkgresource.State{
			{Type: bucket.Type(), URN: bucket, Custom: true, SnippetID: "snippet-1"},
		},
	})
	require.Equal(t, map[string]string{"plainBucket": string(bucket)}, names)
}

func TestResourceNames_HashesNonSnippetResources(t *testing.T) {
	t.Parallel()
	bucket := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::myBucket")
	names := ResourceNames(&deploy.Snapshot{
		Resources: []*pkgresource.State{
			{Type: bucket.Type(), URN: bucket, Custom: true},
		},
	})
	require.Equal(t, map[string]string{AvailableHashedIdent("myBucket", bucket, nil): string(bucket)}, names)
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
	require.Equal(t, map[string]string{AvailableHashedIdent("myBucket", bucket, nil): string(bucket)}, names)
}

func TestResourceNames_SnippetConflictAppendsHash(t *testing.T) {
	t.Parallel()
	// Two snippet resources with the same snippet name: neither wins the bare name — both
	// fall through to hash-suffixed identifiers so the shorter identifier isn't handed out
	// arbitrarily based on URN order.
	a := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::shared")
	b := resource.URN("urn:pulumi:dev::proj::aws:ec2/vpc:Vpc::shared")
	names := ResourceNames(&deploy.Snapshot{
		Snippets: []resource.Snippet{
			{UUID: "snippet-a", Name: "shared", Type: string(a.Type())},
			{UUID: "snippet-b", Name: "shared", Type: string(b.Type())},
		},
		Resources: []*pkgresource.State{
			{Type: a.Type(), URN: a, Custom: true, SnippetID: "snippet-a"},
			{Type: b.Type(), URN: b, Custom: true, SnippetID: "snippet-b"},
		},
	})
	require.NotContains(t, names, "shared")
	require.Equal(t, string(a), names[AvailableHashedIdent("shared", a, nil)])
	require.Equal(t, string(b), names[AvailableHashedIdent("shared", b, nil)])
	require.Len(t, names, 2)
}

func TestResourceNames_ExtendsTakenHash(t *testing.T) {
	t.Parallel()
	bucket := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::bucket")
	conflict := resource.URN("urn:pulumi:dev::proj::aws:ec2/vpc:Vpc::vpc")
	shortHashName := AvailableHashedIdent("bucket", bucket, nil)

	names := ResourceNames(&deploy.Snapshot{
		Snippets: []resource.Snippet{{
			UUID: "snippet-conflict",
			Name: shortHashName,
			Type: string(conflict.Type()),
		}},
		Resources: []*pkgresource.State{
			{Type: bucket.Type(), URN: bucket, Custom: true},
			{Type: conflict.Type(), URN: conflict, Custom: true, SnippetID: "snippet-conflict"},
		},
	})

	require.Equal(t, string(conflict), names[shortHashName])
	require.Equal(t, string(bucket), names["bucket_d22a6ac"])
	require.Len(t, names, 2)
}

func TestResourceNames_SanitizesInvalidIdentifierChars(t *testing.T) {
	t.Parallel()
	// A URN name with characters that aren't valid in a PCL identifier.
	u := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket.v2")
	names := ResourceNames(&deploy.Snapshot{
		Resources: []*pkgresource.State{{Type: u.Type(), URN: u, Custom: true}},
	})
	require.Equal(t, map[string]string{AvailableHashedIdent("my-bucket.v2", u, nil): string(u)}, names)
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
	require.Equal(t, before[AvailableHashedIdent("a", a, nil)], after[AvailableHashedIdent("a", a, nil)],
		"existing identifier must not shift when unrelated resource is added")
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
