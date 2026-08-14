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
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/autonames"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func stateGetSnap(states ...*pkgresource.State) *deploy.Snapshot {
	return &deploy.Snapshot{Resources: states}
}

func customState(urn resource.URN) *pkgresource.State {
	return &pkgresource.State{Type: urn.Type(), URN: urn, Custom: true}
}

func TestResolveResourceRef_ByURN(t *testing.T) {
	t.Parallel()
	urn := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket")
	res, err := resolveResourceRef(stateGetSnap(customState(urn)), string(urn), &bytes.Buffer{})
	require.NoError(t, err)
	require.Equal(t, urn, res.URN)
}

func TestResolveResourceRef_ByAutoRef(t *testing.T) {
	t.Parallel()
	// A resource produced by a snippet is referenced by the snippet's plain name.
	urn := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket")
	snap := &deploy.Snapshot{
		Snippets: []resource.Snippet{{UUID: "snippet-1", Name: "myBucket", Type: string(urn.Type())}},
		Resources: []*pkgresource.State{
			{Type: urn.Type(), URN: urn, Custom: true, SnippetID: "snippet-1"},
		},
	}
	res, err := resolveResourceRef(snap, "myBucket", &bytes.Buffer{})
	require.NoError(t, err)
	require.Equal(t, urn, res.URN)
}

func TestResolveResourceRef_ByHashedRef(t *testing.T) {
	t.Parallel()
	// Non-snippet resources get hash-suffixed identifiers; those resolve too.
	urn := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket")
	ident := autonames.AvailableHashedIdent("my-bucket", urn, nil)
	res, err := resolveResourceRef(stateGetSnap(customState(urn)), ident, &bytes.Buffer{})
	require.NoError(t, err)
	require.Equal(t, urn, res.URN)
}

func TestResolveResourceRef_ByRawNameFallback(t *testing.T) {
	t.Parallel()
	// A non-snippet resource's identifier is its sanitized name plus a hash, so the plain
	// resource name is not in the auto-ref map; it must still resolve via the name fallback.
	urn := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket.v2")
	res, err := resolveResourceRef(stateGetSnap(customState(urn)), "my-bucket.v2", &bytes.Buffer{})
	require.NoError(t, err)
	require.Equal(t, urn, res.URN)
}

func TestResolveResourceRef_RefAndNameConflict(t *testing.T) {
	t.Parallel()
	// A snippet named "my_bucket" holds that identifier while a different resource is
	// literally named "my_bucket". The shared spelling must not resolve silently to either.
	byRef := resource.URN("urn:pulumi:dev::proj::aws:ec2/vpc:Vpc::my-vpc")
	byName := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my_bucket")
	snap := &deploy.Snapshot{
		Snippets: []resource.Snippet{{UUID: "snippet-1", Name: "my_bucket", Type: string(byRef.Type())}},
		Resources: []*pkgresource.State{
			{Type: byRef.Type(), URN: byRef, Custom: true, SnippetID: "snippet-1"},
			customState(byName),
		},
	}

	_, err := resolveResourceRef(snap, "my_bucket", &bytes.Buffer{})
	require.ErrorContains(t, err, "ambiguous")
	require.ErrorContains(t, err, string(byRef))
	require.ErrorContains(t, err, string(byName))
}

func TestResolveResourceRef_RefOwnerIsAlsoNameMatch(t *testing.T) {
	t.Parallel()
	// The resource holding the ref is also the only resource with that name — no conflict.
	urn := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my_bucket")
	snap := &deploy.Snapshot{
		Snippets: []resource.Snippet{{UUID: "snippet-1", Name: "my_bucket", Type: string(urn.Type())}},
		Resources: []*pkgresource.State{
			{Type: urn.Type(), URN: urn, Custom: true, SnippetID: "snippet-1"},
		},
	}
	res, err := resolveResourceRef(snap, "my_bucket", &bytes.Buffer{})
	require.NoError(t, err)
	require.Equal(t, urn, res.URN)
}

func TestResolveResourceRef_AmbiguousRawName(t *testing.T) {
	t.Parallel()
	a := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::a-b")
	b := resource.URN("urn:pulumi:dev::proj::aws:ec2/vpc:Vpc::a-b")
	_, err := resolveResourceRef(stateGetSnap(customState(a), customState(b)), "a-b", &bytes.Buffer{})
	require.ErrorContains(t, err, `2 resources in the stack are named "a-b"`)
	require.ErrorContains(t, err, string(a))
	require.ErrorContains(t, err, string(b))
}

func TestResolveResourceRef_NotFound(t *testing.T) {
	t.Parallel()
	urn := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket")
	_, err := resolveResourceRef(stateGetSnap(customState(urn)), "nope", &bytes.Buffer{})
	require.ErrorContains(t, err, `no resource identified by "nope"`)
}

func TestResolveResourceRef_URNPrefersLiveCopy(t *testing.T) {
	t.Parallel()
	urn := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket")
	tombstone := customState(urn)
	tombstone.Delete = true
	live := customState(urn)

	var warnings bytes.Buffer
	res, err := resolveResourceRef(stateGetSnap(tombstone, live), string(urn), &warnings)
	require.NoError(t, err)
	require.False(t, res.Delete)
	require.Contains(t, warnings.String(), "pending deletion")
}

func TestPrintResourceState_MasksSecrets(t *testing.T) {
	t.Parallel()
	urn := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket")
	res := customState(urn)
	res.ID = "bucket-1234"
	res.Outputs = resource.PropertyMap{
		"public": resource.NewProperty("visible"),
		"token":  resource.MakeSecret(resource.NewProperty("hunter2")),
		"__meta": resource.NewProperty("internal"),
	}

	render := func(showSecrets bool) string {
		cmd := &cobra.Command{}
		cmd.SetContext(t.Context())
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, renderResourceStateText(cmd, res, showSecrets))
		return out.String()
	}

	masked := render(false)
	require.Contains(t, masked, "visible")
	require.Contains(t, masked, "[secret]")
	require.NotContains(t, masked, "hunter2")
	require.NotContains(t, masked, "__meta")

	shown := render(true)
	require.Contains(t, shown, "hunter2")
	require.NotContains(t, shown, "[secret]")
}
