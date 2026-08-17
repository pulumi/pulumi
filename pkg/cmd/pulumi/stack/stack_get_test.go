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

package stack

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleCloudStack returns a fixture apitype.Stack with every cloud-only
// field populated; tests use it to assert the JSON envelope's cloud
// section.
func sampleCloudStack() apitype.Stack {
	return apitype.Stack{
		ID:           "abc123",
		OrgName:      "my-org",
		ProjectName:  "my-project",
		StackName:    "dev",
		ActiveUpdate: "11111111-2222-3333-4444-555555555555",
		Version:      42,
		Tags: map[apitype.StackTagName]string{
			"environment":    "production",
			"pulumi:project": "my-project",
		},
		CurrentOperation: &apitype.OperationStatus{
			Kind:    apitype.UpdateUpdate,
			Author:  "alice",
			Started: 1747142400, // 2025-05-13T13:20:00Z
		},
	}
}

func sampleIdentityInputs() stackJSONInputs {
	return stackJSONInputs{
		StackName:   "dev",
		Project:     "my-project",
		BackendName: "pulumi.com",
	}
}

// TestBuildStackJSON_Cloud asserts that the JSON envelope preserves every
// field the previous `pulumi stack get` envelope emitted, plus the new
// snapshot/identity fields.
func TestBuildStackJSON_Cloud(t *testing.T) {
	t.Parallel()

	in := sampleIdentityInputs()
	cs := sampleCloudStack()
	in.CloudStack = &cs
	in.ConsoleURL = "https://app.pulumi.com/my-org/my-project/dev"

	env := buildStackJSON(in)
	var buf bytes.Buffer
	require.NoError(t, renderStackJSON(&buf, env))

	assert.JSONEq(t, `{
		"organization": "my-org",
		"project": "my-project",
		"stack": "dev",
		"backend": "pulumi.com",
		"version": 42,
		"activeUpdate": "11111111-2222-3333-4444-555555555555",
		"currentOperation": {
			"kind": "update",
			"author": "alice",
			"started": "2025-05-13T13:20:00.000Z"
		},
		"tags": {
			"environment":    "production",
			"pulumi:project": "my-project"
		},
		"resources": [],
		"outputs": {},
		"consoleUrl": "https://app.pulumi.com/my-org/my-project/dev"
	}`, buf.String())
}

func TestBuildStackJSON_Cloud_NoOperation(t *testing.T) {
	t.Parallel()

	in := sampleIdentityInputs()
	cs := sampleCloudStack()
	cs.CurrentOperation = nil
	in.CloudStack = &cs

	env := buildStackJSON(in)
	assert.Nil(t, env.CurrentOperation)
	assert.Equal(t, "11111111-2222-3333-4444-555555555555", env.ActiveUpdate)
}

func TestBuildStackJSON_Cloud_NoTags(t *testing.T) {
	t.Parallel()

	in := sampleIdentityInputs()
	cs := sampleCloudStack()
	cs.Tags = nil
	in.CloudStack = &cs

	env := buildStackJSON(in)
	assert.Equal(t, map[string]string{}, env.Tags, "tags must be present (possibly empty) for JSON consumers")
}

// TestBuildStackJSON_DIY confirms cloud-only fields are absent on DIY
// backends while the identity fields still render.
func TestBuildStackJSON_DIY(t *testing.T) {
	t.Parallel()

	in := stackJSONInputs{
		StackName:   "dev",
		Project:     "proj",
		BackendName: "file:///tmp/state",
	}

	env := buildStackJSON(in)
	var buf bytes.Buffer
	require.NoError(t, renderStackJSON(&buf, env))

	assert.JSONEq(t, `{
		"project": "proj",
		"stack": "dev",
		"backend": "file:///tmp/state",
		"tags": {},
		"resources": [],
		"outputs": {}
	}`, buf.String())
}

// TestBuildStackJSON_PreservesLegacyKeys is a regression guard. The shape
// emitted by the previous `pulumi stack get` JSON envelope is a strict
// subset of the new unified envelope; anything that consumed it must
// continue to work.
func TestBuildStackJSON_PreservesLegacyKeys(t *testing.T) {
	t.Parallel()

	in := sampleIdentityInputs()
	cs := sampleCloudStack()
	in.CloudStack = &cs

	var buf bytes.Buffer
	require.NoError(t, renderStackJSON(&buf, buildStackJSON(in)))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

	legacyKeys := []string{
		"organization", "project", "stack", "version",
		"activeUpdate", "currentOperation", "tags",
	}
	for _, k := range legacyKeys {
		assert.Contains(t, got, k, "legacy key %q must remain in the envelope", k)
	}

	// And legacy values should round-trip unchanged.
	assert.Equal(t, "my-org", got["organization"])
	assert.Equal(t, "my-project", got["project"])
	assert.Equal(t, "dev", got["stack"])
	assert.Equal(t, float64(42), got["version"])
	assert.Equal(t, "11111111-2222-3333-4444-555555555555", got["activeUpdate"])
}

// TestBuildStackJSON_WithSnapshot verifies that local snapshot data
// (manifest, resources) flows into the envelope when a snapshot is
// supplied.
func TestBuildStackJSON_WithSnapshot(t *testing.T) {
	t.Parallel()

	v123 := semver.MustParse("1.2.3")

	urn := resource.NewURN("dev", "proj", "", "aws:s3/bucket:Bucket", "my-bucket")
	snap := &deploy.Snapshot{
		Manifest: deploy.Manifest{
			Time:    time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC),
			Version: "v3.140.0",
			Plugins: []workspace.PluginInfo{
				{Name: "aws", Kind: "resource", Version: &v123},
			},
		},
		Resources: []*pkgresource.State{
			{URN: urn, Type: "aws:s3/bucket:Bucket", ID: "bucket-id-1"},
		},
	}

	in := sampleIdentityInputs()
	in.Snapshot = snap

	env := buildStackJSON(in)

	require.NotNil(t, env.Manifest)
	assert.Equal(t, "v3.140.0", env.Manifest.PulumiVersion)
	assert.Equal(t, "2026-05-13T12:00:00.000Z", env.Manifest.Time,
		"manifest time must use cmd.FormatTime (RFC 5424 ms, UTC)")
	require.Len(t, env.Manifest.Plugins, 1)
	assert.Equal(t, "aws", env.Manifest.Plugins[0].Name)
	assert.Equal(t, "1.2.3", env.Manifest.Plugins[0].Version)

	require.Len(t, env.Resources, 1)
	assert.Equal(t, string(urn), env.Resources[0].URN)
	assert.Equal(t, "aws:s3/bucket:Bucket", env.Resources[0].Type)
	assert.Equal(t, "my-bucket", env.Resources[0].Name)
	assert.Equal(t, "bucket-id-1", env.Resources[0].ID)
}

func TestCapitalizeFirst(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"update", "Update"},
		{"refresh", "Refresh"},
		{"destroy", "Destroy"},
		{"X", "X"},
		{"", "Operation"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, capitalizeFirst(c.in), "input %q", c.in)
	}
}

// TestRenderCloudStackText exercises the cloud-metadata text renderer used
// inside `pulumi stack`'s text path on cloud backends.
func TestRenderCloudStackText(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cs := sampleCloudStack()
	renderCloudStackText(&buf, &cs)

	out := buf.String()
	assert.Contains(t, out, "Version: 42")
	assert.Contains(t, out, "Active update: 11111111-2222-3333-4444-555555555555")
	assert.Contains(t, out, "Tags:")
	assert.Contains(t, out, "environment")
	assert.Contains(t, out, "production")
	assert.Contains(t, out, "pulumi:project")
}

func TestRenderCloudStackText_NilInfo(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	renderCloudStackText(&buf, nil)
	assert.Empty(t, buf.String())
}

func TestRenderCloudStackText_EmptyInfo(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	renderCloudStackText(&buf, &apitype.Stack{}) // zero-value: no version, no update, no tags
	assert.Empty(t, buf.String(),
		"zero-value cloud info must produce no output (each field is conditional)")
}

func TestRenderCloudStackText_OnlyTags(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	renderCloudStackText(&buf, &apitype.Stack{
		Tags: map[apitype.StackTagName]string{
			"b": "two",
			"a": "one",
		},
	})

	out := buf.String()
	assert.NotContains(t, out, "Version:")
	assert.NotContains(t, out, "Active update:")
	assert.Contains(t, out, "Tags:")
	// Tags must be sorted by key.
	aIdx := stringIndex(out, "a  ")
	bIdx := stringIndex(out, "b  ")
	require.NotEqual(t, -1, aIdx)
	require.NotEqual(t, -1, bIdx)
	assert.Less(t, aIdx, bIdx, "tags must render in sorted key order")
}

func stringIndex(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// newMockStack builds a backend.Stack stub whose Snapshot/Ref/Backend
// hooks return the values supplied. It deliberately does NOT implement
// httpstate.Stack, so the cloud-info path stays unexercised.
func newMockStack(snap *deploy.Snapshot, snapErr error) backend.Stack {
	mockBe := &backend.MockBackend{
		NameF: func() string { return "mock://test" },
	}
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV:    tokens.MustParseStackName("dev"),
				ProjectV: "proj",
				StringV:  "dev",
			}
		},
		BackendF: func() backend.Backend { return mockBe },
		SnapshotF: func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
			return snap, snapErr
		},
	}
}

// TestLoadStackJSONInputs_NonCloud_NilSnapshot exercises the non-cloud
// path through loadStackJSONInputs end-to-end (no GetStack call).
func TestLoadStackJSONInputs_NonCloud_NilSnapshot(t *testing.T) {
	t.Parallel()

	in, err := loadStackJSONInputs(t.Context(), newMockStack(nil, nil), false)
	require.NoError(t, err)
	assert.Equal(t, "dev", in.StackName)
	assert.Equal(t, "proj", in.Project)
	assert.Equal(t, "mock://test", in.BackendName)
	assert.Nil(t, in.CloudStack, "non-cloud backends must not populate CloudStack")
	assert.Empty(t, in.ConsoleURL)
	assert.Nil(t, in.Snapshot, "nil snapshot must propagate (it is a legitimate 'no updates yet' state)")
}

// TestLoadStackJSONInputs_PropagatesSnapshotError checks that snapshot
// load failures are surfaced rather than swallowed.
func TestLoadStackJSONInputs_PropagatesSnapshotError(t *testing.T) {
	t.Parallel()

	want := assert.AnError
	_, err := loadStackJSONInputs(t.Context(), newMockStack(nil, want), false)
	require.ErrorIs(t, err, want)
}

// TestFetchCloudStackInfo_NonCloud confirms the helper returns
// (nil, "", nil) when the stack isn't on the Pulumi Cloud backend, so
// callers can skip the cloud block without an error path.
func TestFetchCloudStackInfo_NonCloud(t *testing.T) {
	t.Parallel()

	info, url, err := fetchCloudStackInfo(t.Context(), newMockStack(nil, nil))
	require.NoError(t, err)
	assert.Nil(t, info)
	assert.Empty(t, url)
}

// TestRunStackJSON_NonCloud_NilSnapshot end-to-end-tests runStackJSON
// against a non-cloud backend with no snapshot; the JSON must contain
// the identity fields and the empty resource/output/tag scaffolding.
func TestRunStackJSON_NonCloud_NilSnapshot(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runStackJSON(t.Context(), newMockStack(nil, nil), &buf, stackArgs{})
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"project": "proj",
		"stack": "dev",
		"backend": "mock://test",
		"tags": {},
		"resources": [],
		"outputs": {}
	}`, buf.String())
}
