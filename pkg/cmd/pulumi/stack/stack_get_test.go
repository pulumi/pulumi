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
	"encoding/json"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
		Resources: []*resource.State{
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

// TestNewStackGetCmd_FlagDefaults guards the cobra surface: --output
// defaults to json, --stack and --show-secrets are present.
func TestNewStackGetCmd_FlagDefaults(t *testing.T) {
	t.Parallel()

	cmd := newStackGetCmd()
	assert.Equal(t, "get", cmd.Use)
	require.NotNil(t, cmd.RunE)

	output := cmd.Flags().Lookup("output")
	require.NotNil(t, output)
	assert.Equal(t, "o", output.Shorthand)
	assert.Equal(t, "json", output.DefValue, "stack get must default to --output=json")

	stack := cmd.Flags().Lookup("stack")
	require.NotNil(t, stack)
	assert.Equal(t, "s", stack.Shorthand)

	secrets := cmd.Flags().Lookup("show-secrets")
	require.NotNil(t, secrets)
	assert.Equal(t, "false", secrets.DefValue)
}
