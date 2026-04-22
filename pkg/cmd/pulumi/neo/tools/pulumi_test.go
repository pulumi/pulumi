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

package tools

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
)

func TestPulumi_NewPulumiRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	_, err := NewPulumi(t.TempDir(), nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace")
}

func TestPulumi_InvokeUnknownMethod(t *testing.T) {
	t.Parallel()

	p := &Pulumi{Cwd: t.TempDir()}
	_, err := p.Invoke(t.Context(), "pulumi_destroy", json.RawMessage(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown pulumi method")
}

func TestPulumi_InvokeRejectsBadJSON(t *testing.T) {
	t.Parallel()

	p := &Pulumi{Cwd: t.TempDir()}
	_, err := p.Invoke(t.Context(), "pulumi_preview", json.RawMessage(`not json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding")
}

func TestPulumi_RunRejectsMissingStackName(t *testing.T) {
	t.Parallel()

	p := &Pulumi{Cwd: t.TempDir()}
	_, err := p.Invoke(t.Context(), "pulumi_preview",
		json.RawMessage(`{"project_name":"p","local_pulumi_dir":"/tmp"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stack_name is required")
}

func TestPulumi_RunRejectsMissingLocalDir(t *testing.T) {
	t.Parallel()

	p := &Pulumi{Cwd: t.TempDir()}
	_, err := p.Invoke(t.Context(), "pulumi_preview",
		json.RawMessage(`{"project_name":"p","stack_name":"dev"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local_pulumi_dir is required")
}

func TestPulumi_RunRejectsRelativeLocalDir(t *testing.T) {
	t.Parallel()

	p := &Pulumi{Cwd: t.TempDir()}
	_, err := p.Invoke(t.Context(), "pulumi_preview",
		json.RawMessage(`{"project_name":"p","stack_name":"dev","local_pulumi_dir":"relative/path"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be an absolute path")
}

func TestPulumi_RunRejectsEscapingLocalDir(t *testing.T) {
	t.Parallel()

	sandbox, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)

	outside, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)

	p := &Pulumi{Cwd: sandbox}
	args, err := json.Marshal(map[string]any{
		"project_name":     "p",
		"stack_name":       "dev",
		"local_pulumi_dir": outside,
	})
	require.NoError(t, err)
	_, err = p.Invoke(t.Context(), "pulumi_preview", args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside")
}

func TestPulumi_RunRejectsMissingPulumiYaml(t *testing.T) {
	t.Parallel()

	root, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)

	p := &Pulumi{Cwd: root}
	args, err := json.Marshal(map[string]any{
		"project_name":     "p",
		"stack_name":       "dev",
		"local_pulumi_dir": root,
	})
	require.NoError(t, err)
	_, err = p.Invoke(t.Context(), "pulumi_preview", args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Pulumi.yaml not found")
}

func TestEnvValUnmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    envVal
		wantErr bool
	}{
		{name: "plain", input: `"hello"`, want: envVal{Plain: "hello"}},
		{name: "secret", input: `{"secret":"shh"}`, want: envVal{Secret: "shh"}},
		{name: "empty_secret", input: `{"secret":""}`, wantErr: true},
		{name: "number", input: `42`, wantErr: true},
		{name: "object_no_secret", input: `{"foo":"bar"}`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got envVal
			err := json.Unmarshal([]byte(tc.input), &got)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestEnvValValue(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "plain", envVal{Plain: "plain"}.Value())
	assert.Equal(t, "secret", envVal{Secret: "secret"}.Value())
	// Secret takes precedence over Plain.
	assert.Equal(t, "secret", envVal{Plain: "plain", Secret: "secret"}.Value())
}

func TestApplyEnvVarsSetsAndRestores(t *testing.T) {
	// t.Setenv precludes t.Parallel, which is what we want here — the test mutates
	// process-global state.
	const presentKey = "PULUMI_NEO_TEST_PRESENT"
	const absentKey = "PULUMI_NEO_TEST_ABSENT"

	t.Setenv(presentKey, "original")
	require.NoError(t, os.Unsetenv(absentKey))
	t.Cleanup(func() { _ = os.Unsetenv(absentKey) })

	restore := applyEnvVars(map[string]envVal{
		presentKey: {Plain: "modified"},
		absentKey:  {Secret: "secret-val"},
	})

	assert.Equal(t, "modified", os.Getenv(presentKey))
	assert.Equal(t, "secret-val", os.Getenv(absentKey))

	restore()

	assert.Equal(t, "original", os.Getenv(presentKey))
	_, absentStillSet := os.LookupEnv(absentKey)
	assert.False(t, absentStillSet, "absent key should be cleared after restore")
}

func TestApplyEnvVarsNoopOnEmpty(t *testing.T) {
	t.Parallel()

	restore := applyEnvVars(nil)
	require.NotNil(t, restore)
	restore()
}

func TestFormatCountsFromChanges(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", formatCountsFromChanges(nil))

	// Same-only counts produce an empty summary.
	assert.Equal(t, "", formatCountsFromChanges(display.ResourceChanges{
		deploy.OpSame: 5,
	}))

	// Zero-count ops are filtered; keys sort alphabetically.
	got := formatCountsFromChanges(display.ResourceChanges{
		deploy.OpUpdate: 2,
		deploy.OpCreate: 3,
		deploy.OpDelete: 0,
		deploy.OpSame:   5,
	})
	assert.Equal(t, "3 create, 2 update", got)
}

func TestFormatUpdateSummary(t *testing.T) {
	t.Parallel()

	out := formatUpdateSummary(
		"dev",
		display.ResourceChanges{deploy.OpCreate: 1},
		[]changeLine{{Op: "create", URN: "urn:a", Type: "aws:s3/Bucket:Bucket"}},
		3*time.Second,
	)
	assert.Contains(t, out, "stack: dev (3s)")
	assert.Contains(t, out, "changes: 1 create")
	assert.Contains(t, out, "resources:")
	assert.Contains(t, out, "create urn:a (aws:s3/Bucket:Bucket)")
}

func TestFormatUpdateSummaryNoChanges(t *testing.T) {
	t.Parallel()

	out := formatUpdateSummary("dev", nil, nil, time.Second)
	assert.Contains(t, out, "changes: none")
	assert.NotContains(t, out, "resources:")
}

func TestAppendLogRespectsCap(t *testing.T) {
	t.Parallel()

	p := &Pulumi{}
	var logs bytes.Buffer
	p.appendLog("hello", &logs)
	assert.Equal(t, "hello\n", logs.String())

	// Fill the buffer to just under the cap, then ensure further writes cannot
	// exceed the cap.
	logs.Reset()
	logs.WriteString(string(make([]byte, maxLogsBytes-1)))
	p.appendLog("extra-that-wont-fit", &logs)
	assert.LessOrEqual(t, logs.Len(), maxLogsBytes)
}

func TestFlattenChanges(t *testing.T) {
	t.Parallel()

	assert.Nil(t, flattenChanges(nil))
	got := flattenChanges(display.ResourceChanges{
		deploy.OpCreate: 2,
		deploy.OpDelete: 1,
	})
	assert.Equal(t, map[string]int{"create": 2, "delete": 1}, got)
}
