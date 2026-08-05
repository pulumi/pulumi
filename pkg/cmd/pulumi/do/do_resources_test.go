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

package do

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestAutoResourceNames_PrefersBareName(t *testing.T) {
	t.Parallel()
	bucket := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::myBucket")
	names := autoResourceNames(&deploy.Snapshot{
		Resources: []*pkgresource.State{
			{Type: bucket.Type(), URN: bucket, Custom: true},
		},
	})
	assert.Equal(t, map[string]string{"myBucket": string(bucket)}, names)
}

func TestAutoResourceNames_SkipsProvidersAndStackAndDeletes(t *testing.T) {
	t.Parallel()
	stack := resource.URN("urn:pulumi:dev::proj::pulumi:pulumi:Stack::proj-dev")
	provider := resource.URN("urn:pulumi:dev::proj::pulumi:providers:aws::default_1_2_3")
	bucket := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::myBucket")
	tombstone := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::deleted")
	names := autoResourceNames(&deploy.Snapshot{
		Resources: []*pkgresource.State{
			{Type: stack.Type(), URN: stack},
			{Type: provider.Type(), URN: provider, Custom: true},
			{Type: bucket.Type(), URN: bucket, Custom: true},
			{Type: tombstone.Type(), URN: tombstone, Custom: true, Delete: true},
		},
	})
	assert.Equal(t, map[string]string{"myBucket": string(bucket)}, names)
}

func TestAutoResourceNames_DisambiguatesConflicts(t *testing.T) {
	t.Parallel()
	// Two resources with the same URN.Name() but different types — the first (in URN order)
	// takes the bare name, the second falls through to the type-qualified candidate.
	a := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::shared")
	b := resource.URN("urn:pulumi:dev::proj::aws:ec2/vpc:Vpc::shared")
	names := autoResourceNames(&deploy.Snapshot{
		Resources: []*pkgresource.State{
			{Type: a.Type(), URN: a, Custom: true},
			{Type: b.Type(), URN: b, Custom: true},
		},
	})
	// URN order: "aws:ec2..." < "aws:s3...", so Vpc wins the bare name.
	assert.Equal(t, string(b), names["shared"])
	assert.Equal(t, string(a), names["Bucket_shared"])
	require.Len(t, names, 2)
}

func TestAutoResourceNames_SanitizesInvalidIdentifierChars(t *testing.T) {
	t.Parallel()
	// A URN name with characters that aren't valid in a PCL identifier.
	u := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket.v2")
	names := autoResourceNames(&deploy.Snapshot{
		Resources: []*pkgresource.State{{Type: u.Type(), URN: u, Custom: true}},
	})
	assert.Equal(t, map[string]string{"my_bucket_v2": string(u)}, names)
}

func TestAutoResourceNames_StableUnderUnrelatedInsertion(t *testing.T) {
	t.Parallel()
	a := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::a")
	b := resource.URN("urn:pulumi:dev::proj::aws:ec2/vpc:Vpc::b")
	before := autoResourceNames(&deploy.Snapshot{
		Resources: []*pkgresource.State{
			{Type: a.Type(), URN: a, Custom: true},
		},
	})
	after := autoResourceNames(&deploy.Snapshot{
		Resources: []*pkgresource.State{
			{Type: a.Type(), URN: a, Custom: true},
			{Type: b.Type(), URN: b, Custom: true},
		},
	})
	assert.Equal(t, before["a"], after["a"], "existing identifier must not shift when unrelated resource is added")
}

func TestMergeResourceNames_UserWins(t *testing.T) {
	t.Parallel()
	auto := map[string]string{"foo": "urn:auto:foo", "bar": "urn:auto:bar"}
	user := map[string]string{"foo": "urn:user:foo", "baz": "urn:user:baz"}
	got := mergeResourceNames(auto, user)
	assert.Equal(t, map[string]string{
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
		assert.Equal(t, want, sanitizeIdent(in), "sanitizeIdent(%q)", in)
	}
}

func TestFilterReferencesByPCLUsage_KeepsOnlyUsedRoots(t *testing.T) {
	t.Parallel()
	refs := map[string]string{
		"myBucket": "urn:bucket",
		"teamRef":  "urn:team",
		"provider": "urn:provider",
		"unused":   "urn:unused",
	}
	src := []byte(`
name = myBucket.arn
tags = { owner = teamRef.name }
options { provider = provider }
`)
	got := filterReferencesByPCLUsage(refs, src, "test.pp")
	assert.Equal(t, map[string]string{
		"myBucket": "urn:bucket",
		"teamRef":  "urn:team",
		"provider": "urn:provider",
	}, got)
}

func TestFilterReferencesByPCLUsage_KeepsAllOnParseFailure(t *testing.T) {
	t.Parallel()
	// Unparseable input must not silently drop references — losing one the engine later needs
	// is a hard failure at update time, so keeping extras is the safer default.
	refs := map[string]string{"a": "urn:a", "b": "urn:b"}
	got := filterReferencesByPCLUsage(refs, []byte("this is =not= valid hcl"), "bad.pp")
	assert.Equal(t, refs, got)
}

func TestParseShowResourcesArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		wantArgs    []string
		wantBuiltin bool
	}{
		{
			name:        "bare command",
			args:        []string{"show-resources"},
			wantArgs:    []string{},
			wantBuiltin: true,
		},
		{
			name:        "command help",
			args:        []string{"show-resources", "--help"},
			wantArgs:    []string{"--help"},
			wantBuiltin: true,
		},
		{
			name:        "top-level flag before command",
			args:        []string{"--output", "default", "show-resources"},
			wantArgs:    []string{},
			wantBuiltin: true,
		},
		{
			name:        "dynamic package dispatch",
			args:        []string{"--package", "azure", "show-resources"},
			wantBuiltin: false,
		},
		{
			name:        "provider token",
			args:        []string{"azure:index:myResource"},
			wantBuiltin: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotArgs, gotBuiltin, err := parseShowResourcesArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBuiltin, gotBuiltin)
			assert.Equal(t, tt.wantArgs, gotArgs)
		})
	}
}

// TestDoCmdShowResourcesHelp asserts that `pulumi do show-resources --help` renders the
// subcommand's own help without touching the backend. The parent `do` command runs with
// DisableFlagParsing so cobra doesn't handle --help for it directly; the parent hands off to a
// real cobra subcommand to make --help behave normally.
func TestDoCmdShowResourcesHelp(t *testing.T) {
	t.Parallel()

	// A backend that panics if opened — proves --help never reaches the stack loader.
	panicWs := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) {
			t.Fatal("--help must not read the project")
			return nil, "", nil
		},
	}
	panicLm := &cmdBackend.MockLoginManager{
		CurrentF: func(
			context.Context, pkgWorkspace.Context, diag.Sink,
			string, *workspace.Project, bool,
		) (backend.Backend, error) {
			t.Fatal("--help must not open the backend")
			return nil, nil
		},
	}

	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(panicLm, panicWs, nil, testHost, panicLoadConverterPlugin, nil)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"show-resources", "--help"})
	require.NoError(t, cmd.Execute())

	out := stdout.String()
	assert.Contains(t, out, "show-resources")
	assert.Contains(t, out, "auto-assign")
}

// TestDoCmdShowResourcesRejectsArgs asserts that positional arguments are rejected by cobra's
// NoArgs check on the subcommand rather than silently ignored.
func TestDoCmdShowResourcesRejectsArgs(t *testing.T) {
	t.Parallel()

	nopWs := &pkgWorkspace.MockContext{}
	nopLm := &cmdBackend.MockLoginManager{}

	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(nopLm, nopWs, nil, testHost, panicLoadConverterPlugin, nil)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"show-resources", "extra"})
	require.Error(t, cmd.Execute())
}

//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdShowResourcesSubcommand(t *testing.T) {
	bucket := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::myBucket")
	mws, mlm := installMockUpsertBackend(t, &deploy.Snapshot{
		Resources: []*pkgresource.State{
			{Type: bucket.Type(), URN: bucket, Custom: true},
		},
	})

	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, nil, testHost, panicLoadConverterPlugin, nil)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"show-resources"})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, stdout.String(), "myBucket")
	assert.Contains(t, stdout.String(), string(bucket))
}
