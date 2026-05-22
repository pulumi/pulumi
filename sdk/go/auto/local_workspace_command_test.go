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

package auto

import (
	"context"
	"io"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingPulumiCommand captures every Run invocation. Each call appends a
// snapshot of the args, env, and stdin to a slice the test can inspect.
type recordingPulumiCommand struct {
	stdout string
	stderr string
	calls  []recordedCall
}

type recordedCall struct {
	args []string
	env  []string
}

func (r *recordingPulumiCommand) Version() semver.Version {
	return semver.Version{Major: 3, Minor: 200}
}

func (r *recordingPulumiCommand) Run(
	_ context.Context,
	_ string,
	_ io.Reader,
	_ []io.Writer,
	_ []io.Writer,
	additionalEnv []string,
	args ...string,
) (string, string, int, error) {
	r.calls = append(r.calls, recordedCall{
		args: append([]string(nil), args...),
		env:  append([]string(nil), additionalEnv...),
	})
	return r.stdout, r.stderr, 0, nil
}

// envContains reports whether KEY=VALUE is present in env.
func envContains(env []string, key, value string) bool {
	want := key + "=" + value
	for _, e := range env {
		if e == want {
			return true
		}
	}
	return false
}

// TestStackCancelDelegatesToCLIAPI verifies that Stack.Cancel reaches
// PulumiCommand.Run with the same CLI args the hand-written
// implementation used (`cancel --yes --stack <name>`), now via the
// auto-generated automation.API. The args are emitted in alphabetical
// flag order, matching the generator's emission rules.
func TestStackCancelDelegatesToCLIAPI(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cmd := &recordingPulumiCommand{stdout: "ok"}
	ws, err := NewLocalWorkspace(ctx, Pulumi(cmd), WorkDir(t.TempDir()))
	require.NoError(t, err)

	// Build the stack manually to avoid the `stack select` round trip
	// that NewStack would issue against the mock.
	stack := Stack{workspace: ws, stackName: "my-stack"}

	require.NoError(t, stack.Cancel(ctx))

	require.Len(t, cmd.calls, 1, "expected exactly one CLI invocation")
	assert.Equal(t,
		[]string{"cancel", "--yes", "--stack", "my-stack"},
		cmd.calls[0].args,
		"Stack.Cancel should reach the CLI with the hand-written shape")
	assert.True(t, envContains(cmd.calls[0].env, "PULUMI_DEBUG_COMMANDS", "true"),
		"stack-scoped CLI calls should set PULUMI_DEBUG_COMMANDS")
}

// TestOrgGetDefaultDelegatesToCLIAPI checks that
// LocalWorkspace.OrgGetDefault routes through the generated API and
// trims surrounding whitespace from the returned org.
func TestOrgGetDefaultDelegatesToCLIAPI(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cmd := &recordingPulumiCommand{stdout: "my-org\n"}
	ws, err := NewLocalWorkspace(ctx, Pulumi(cmd), WorkDir(t.TempDir()))
	require.NoError(t, err)

	got, err := ws.OrgGetDefault(ctx)
	require.NoError(t, err)
	assert.Equal(t, "my-org", got)

	require.Len(t, cmd.calls, 1)
	assert.Equal(t, []string{"org", "get-default"}, cmd.calls[0].args)
}

// TestOrgSetDefaultDelegatesToCLIAPI checks that
// LocalWorkspace.OrgSetDefault forwards the org name as a positional
// argument. Note the `--` separator inserted by the generator before
// positional args; the previous hand-written implementation omitted it
// but cobra accepts both forms identically.
func TestOrgSetDefaultDelegatesToCLIAPI(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cmd := &recordingPulumiCommand{}
	ws, err := NewLocalWorkspace(ctx, Pulumi(cmd), WorkDir(t.TempDir()))
	require.NoError(t, err)

	require.NoError(t, ws.OrgSetDefault(ctx, "my-org"))

	require.Len(t, cmd.calls, 1)
	assert.Equal(t,
		[]string{"org", "set-default", "--", "my-org"},
		cmd.calls[0].args)
}

// TestNewDelegatesToCLIAPI checks LocalWorkspace.New reaches the CLI
// via the generated API and forwards a representative subset of the
// NewOptions kwargs.
func TestNewDelegatesToCLIAPI(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cmd := &recordingPulumiCommand{stdout: "created"}
	ws, err := NewLocalWorkspace(ctx, Pulumi(cmd), WorkDir(t.TempDir()))
	require.NoError(t, err)

	res, err := ws.New(ctx, &NewOptions{
		TemplateOrURL: "aws-typescript",
		Name:          "my-project",
		Stack:         "dev",
		GenerateOnly:  true,
		Config:        []string{"aws:region=us-east-1", "foo:bar=baz"},
	})
	require.NoError(t, err)
	assert.Equal(t, "created", res.StdOut)

	require.Len(t, cmd.calls, 1)
	args := cmd.calls[0].args

	// `new --yes` is a preset; the generator emits user-supplied flags
	// in alphabetical order between the preset and the `--` separator.
	assert.Equal(t, "new", args[0])
	assert.Contains(t, args, "--yes")
	assert.Contains(t, args, "--config")
	assert.Contains(t, args, "aws:region=us-east-1")
	assert.Contains(t, args, "foo:bar=baz")
	assert.Contains(t, args, "--generate-only")
	assert.Contains(t, args, "--name")
	assert.Contains(t, args, "my-project")
	assert.Contains(t, args, "--stack")
	assert.Contains(t, args, "dev")
	// Positional templateOrURL is forwarded after a `--` separator.
	assert.Equal(t, "--", args[len(args)-2])
	assert.Equal(t, "aws-typescript", args[len(args)-1])
}
