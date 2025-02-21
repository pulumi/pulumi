// Copyright 2016-2021, Pulumi Corporation.
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
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optimport"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optremove"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPermalink = "Permalink: https://gotest"

func TestGetPermalink(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		testee string
		want   string
		err    error
	}{
		"successful parsing": {testee: testPermalink + "\n", want: "https://gotest"},
		"failed parsing":     {testee: testPermalink, err: ErrParsePermalinkFailed},
	}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for name, test := range tests {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := GetPermalink(test.testee)
			if err != nil {
				if test.err == nil || test.err != err {
					t.Errorf("got '%v', want '%v'", err, test.err)
				}
			}

			if got != test.want {
				t.Errorf("got '%s', want '%s'", got, test.want)
			}
		})
	}
}

func TestUpdatePlans(t *testing.T) {
	t.Parallel()

	// TODO[pulumi/pulumi#18459]: This test should be reenabled on windows once we fix the flakyness
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows due to flakiness")
	}

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)

	opts := []LocalWorkspaceOption{
		SecretsProvider("passphrase"),
		EnvVars(map[string]string{
			"PULUMI_CONFIG_PASSPHRASE": "password",
		}),
	}

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, pName, func(ctx *pulumi.Context) error {
		ctx.Export("exp_static", pulumi.String("foo"))
		return nil
	}, opts...)
	require.NoError(t, err, "failed to initialize stack, err: %v", err)

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	// first load settings for created stack
	stackConfig, err := s.Workspace().StackSettings(ctx, stackName)
	require.NoError(t, err)
	stackConfig.SecretsProvider = "passphrase"
	assert.NoError(t, s.Workspace().SaveStackSettings(ctx, stackName, stackConfig))

	// -- pulumi preview --
	tempFile, err := os.CreateTemp("", "update_plan.json")
	defer os.Remove(tempFile.Name())

	_, err = s.Preview(ctx, optpreview.Plan(tempFile.Name()))
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}

	stat, err := tempFile.Stat()
	if err != nil {
		t.Errorf("state failed, err: %v", err)
		t.FailNow()
	}

	if stat.Size() == 0 {
		t.Errorf("expected update plan size to be non-zero")
		t.FailNow()
	}

	// -- pulumi up --

	upResult, err := s.Up(ctx, optup.Plan(tempFile.Name()))
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "update", upResult.Summary.Kind)
	assert.Equal(t, "succeeded", upResult.Summary.Result)

	// -- pulumi destroy --

	dRes, err := s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func TestAlwaysReadsCompleteLine(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.txt"
	go func() {
		f, err := os.Create(tmpFile)
		require.NoError(t, err)
		defer f.Close()
		parts := []string{
			`{"stdoutEvent": `,
			` {"message": "hello", "color": "blue"}}` + "\n",
			`{"stdoutEvent": {"message":`,
			` "world", "color": "red"}}` + "\n",
		}
		for _, part := range parts {
			_, err = f.WriteString(part)
			require.NoError(t, err)
			time.Sleep(200 * time.Millisecond)
		}
	}()
	engineEvents := make(chan events.EngineEvent, 20)
	watcher, err := watchFile(tmpFile, []chan<- events.EngineEvent{engineEvents})
	require.NoError(t, err)
	defer watcher.Close()
	event1 := <-engineEvents
	require.NoError(t, event1.Error)
	assert.Equal(t, "hello", event1.StdoutEvent.Message)
	assert.Equal(t, "blue", event1.StdoutEvent.Color)
	event2 := <-engineEvents
	require.NoError(t, event2.Error)
	assert.Equal(t, "world", event2.StdoutEvent.Message)
	assert.Equal(t, "red", event2.StdoutEvent.Color)
}

func TestUpOptsConfigFileNestedSecretLocalBackend(t *testing.T) {
	t.Parallel()

	// Copy the test project to a temp directory.
	pDir := t.TempDir()
	err := fsutil.CopyFile(pDir, filepath.Join(".", "test", "testproj"), nil)
	require.NoError(t, err)

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName("organization", pName, sName)

	// Config with a nested secret.
	cfg := ConfigMap{
		"foo.bar": ConfigValue{
			Value:  "triggers-failure",
			Secret: true,
		},
	}

	// Use the DYI local backend at a temp directory.
	opts := []LocalWorkspaceOption{
		SecretsProvider("passphrase"),
		EnvVars(map[string]string{
			"PULUMI_CONFIG_PASSPHRASE": "password",
			"PULUMI_BACKEND_URL":       "file://" + filepath.ToSlash(t.TempDir()),
		}),
	}

	stack, err := UpsertStackLocalSource(ctx, stackName, pDir, opts...)
	require.NoError(t, err)

	defer func() {
		err = stack.Workspace().RemoveStack(ctx, stack.Name(), optremove.Force())
		assert.NoError(t, err, "failed to remove stack.")
	}()

	configFile := filepath.Join(stack.Workspace().WorkDir(), "test.yaml")

	err = stack.SetAllConfigWithOptions(ctx, cfg, &ConfigOptions{
		ConfigFile: configFile,
		Path:       true,
	})
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	res, err := stack.Up(ctx, optup.ConfigFile(configFile), optup.Diff())
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	dRes, err := stack.Destroy(ctx, optdestroy.ConfigFile(configFile))
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func TestDestroyOptsConfigFile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	pDir := filepath.Join(".", "test", "testproj")

	stack, err := NewStackLocalSource(ctx, stackName, pDir)
	require.NoError(t, err)

	args := destroyOptsToCmd(
		&optdestroy.Options{
			ConfigFile: filepath.Join(stack.workspace.WorkDir(), "test.yaml"),
		},
		&stack,
	)

	assert.Contains(t, args, "destroy")

	configFilePath := filepath.Join(stack.workspace.WorkDir(), "test.yaml")
	assert.Contains(t, args, "--config-file="+configFilePath)
}

func TestRefreshOptsConfigFile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	pDir := filepath.Join(".", "test", "testproj")

	stack, err := NewStackLocalSource(ctx, stackName, pDir)
	require.NoError(t, err)

	args := refreshOptsToCmd(
		&optrefresh.Options{
			ConfigFile: filepath.Join(stack.workspace.WorkDir(), "test.yaml"),
		},
		&stack,
		true,
	)

	assert.Contains(t, args, "refresh")

	configFilePath := filepath.Join(stack.workspace.WorkDir(), "test.yaml")
	assert.Contains(t, args, "--config-file="+configFilePath)
}

func TestRefreshOptsClearPendingCreates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	pDir := filepath.Join(".", "test", "testproj")

	stack, err := NewStackLocalSource(ctx, stackName, pDir)
	require.NoError(t, err)

	args := refreshOptsToCmd(
		&optrefresh.Options{
			ClearPendingCreates: true,
		},
		&stack,
		true,
	)

	assert.Contains(t, args, "--clear-pending-creates")
}

func TestPreviewImportResources(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)

	s, err := NewStackInlineSource(ctx, stackName, pName, func(ctx *pulumi.Context) error {
		ctx.Export("exp_static", pulumi.String("foo"))
		return nil
	})
	require.NoError(t, err, "failed to initialize stack")

	defer func() {
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.NoError(t, err, "failed to remove stack. Resources have leaked.")
	}()

	tempDir := t.TempDir()
	importFilePath := filepath.Join(tempDir, "import.json")
	resources := []byte(`{"resoures": [{"type":"my:module:MyResource","name":"imported-resource","id":"preview-bar"}]}`)
	err = os.WriteFile(importFilePath, resources, 0o600)
	assert.NoError(t, err, "error writing file")

	// Act
	result, err := s.ImportResources(ctx,
		optimport.Protect(false),
		optimport.GenerateCode(true),
		optimport.PreviewOnly(true),
		optimport.ImportFile(importFilePath),
	)

	// Assert
	require.NoError(t, err, "import failed")
	assert.Contains(t, result.StdOut, "Previewing")
	assert.NotContains(t, result.StdOut, "Importing")
}
