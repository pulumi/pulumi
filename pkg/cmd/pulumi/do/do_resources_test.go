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
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/autonames"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

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

//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdShowResourcesSubcommandJSON(t *testing.T) {
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
	cmd.SetArgs([]string{"--output", "json", "show-resources"})
	require.NoError(t, cmd.Execute())

	autoName := autonames.AvailableHashedIdent("myBucket", bucket, nil)
	assert.JSONEq(t, `{"`+autoName+`":"`+string(bucket)+`"}`, stdout.String())
}
