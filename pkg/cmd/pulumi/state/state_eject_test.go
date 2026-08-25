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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestEjectSnippetFromSnapshot_RemovesSnippetAndClearsResourceOwnership(t *testing.T) {
	t.Parallel()

	const snippetID = "3fa85f64-5717-4562-b3fc-2c963f66afa6"
	urn := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::bucket")
	otherURN := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::other")
	snap := &deploy.Snapshot{
		Snippets: []resource.Snippet{
			{UUID: "other-snippet", Name: "other", Type: string(otherURN.Type())},
			{UUID: snippetID, Name: "bucket", Type: string(urn.Type())},
		},
		Resources: []*pkgresource.State{
			{URN: urn, Type: urn.Type(), Custom: true, SnippetID: snippetID},
			{URN: otherURN, Type: otherURN.Type(), Custom: true, SnippetID: "other-snippet"},
		},
	}

	expected := snap.Snippets[1]
	cleared, err := ejectSnippetFromSnapshot(snap, expected)
	require.NoError(t, err)

	assert.Equal(t, 1, cleared)
	require.Len(t, snap.Snippets, 1)
	assert.Equal(t, "other-snippet", snap.Snippets[0].UUID)
	assert.Empty(t, snap.Resources[0].SnippetID)
	assert.Equal(t, "other-snippet", snap.Resources[1].SnippetID)
}

func TestEjectSnippetFromSnapshot_MissingSnippet(t *testing.T) {
	t.Parallel()

	_, err := ejectSnippetFromSnapshot(&deploy.Snapshot{}, resource.Snippet{UUID: "missing"})
	require.ErrorContains(t, err, `no snippet "missing" exists`)
}

func TestResolveSnippetForEject_ByName(t *testing.T) {
	t.Parallel()

	const snippetID = "3fa85f64-5717-4562-b3fc-2c963f66afa6"
	snap := &deploy.Snapshot{
		Snippets: []resource.Snippet{{UUID: snippetID, Name: "bucket", Type: "aws:s3/bucket:Bucket"}},
	}

	snippet, err := resolveSnippetForEject(snap, "bucket")
	require.NoError(t, err)
	assert.Equal(t, snippetID, snippet.UUID)
}

func TestResolveSnippetForEject_UnknownName(t *testing.T) {
	t.Parallel()

	_, err := resolveSnippetForEject(&deploy.Snapshot{}, "bucket")
	require.ErrorContains(t, err, `no snippet named "bucket" exists`)
}

func TestResolveSnippetForEject_AmbiguousName(t *testing.T) {
	t.Parallel()

	snap := &deploy.Snapshot{
		Snippets: []resource.Snippet{
			{UUID: "uuid-b", Name: "bucket", Type: "aws:s3/bucket:Bucket"},
			{UUID: "uuid-a", Name: "bucket", Type: "aws:s3/bucket:Bucket"},
		},
	}

	_, err := resolveSnippetForEject(snap, "bucket")
	require.ErrorContains(t, err, `snippet name "bucket" is ambiguous`)
}

func TestValidateSnippetEjectReferences_AllowsNonSnippetResource(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::source")
	snap := &deploy.Snapshot{
		Resources: []*pkgresource.State{
			{URN: urn, Type: urn.Type(), Custom: true},
		},
	}
	snippet := resource.Snippet{
		UUID:       "snippet-1",
		Name:       "bucket",
		Type:       string(urn.Type()),
		References: map[string]string{"source": string(urn)},
	}

	require.NoError(t, validateSnippetEjectReferences(snap, snippet))
}

func TestValidateSnippetEjectReferences_RejectsSnippetResource(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::source")
	snap := &deploy.Snapshot{
		Resources: []*pkgresource.State{
			{URN: urn, Type: urn.Type(), Custom: true, SnippetID: "source-snippet"},
		},
	}
	snippet := resource.Snippet{
		UUID:       "consumer-snippet",
		Name:       "bucket",
		Type:       string(urn.Type()),
		References: map[string]string{"source": string(urn)},
	}

	err := validateSnippetEjectReferences(snap, snippet)
	require.ErrorContains(t, err, "does not yet support references to other snippets")
	require.ErrorContains(t, err, "source-snippet")
}

func TestSnippetPCLSource(t *testing.T) {
	t.Parallel()

	src := snippetPCLSource(resource.Snippet{
		Name: "bucket",
		Type: "aws:s3/bucket:Bucket",
		Code: "bucket = \"example\"\nacl = \"private\"\n",
	})

	assert.Equal(t, "resource \"bucket\" \"aws:s3/bucket:Bucket\" {\n"+
		"\tbucket = \"example\"\n"+
		"\tacl = \"private\"\n"+
		"}\n", src)
}

func TestPrintGeneratedSnippetProgram(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	printGeneratedSnippetProgram(&out, resource.Snippet{
		Name: "bucket",
	}, map[string][]byte{
		"b.ts": []byte("export const b = 2\n"),
		"a.ts": []byte("export const a = 1"),
	})

	assert.Equal(t, "Generated code for snippet \"bucket\":\n"+
		"\na.ts\n====\nexport const a = 1\n"+
		"\nb.ts\n====\nexport const b = 2\n", out.String())
}
