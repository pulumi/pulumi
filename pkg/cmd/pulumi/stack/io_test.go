// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestStackLoadOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give       LoadOption
		offerNew   bool
		setCurrent bool
	}{
		{LoadOnly, false, false},
		{OfferNew, true, false},
		{SetCurrent, false, true},
		{OfferNew | SetCurrent, true, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.give), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t,
				tt.offerNew, tt.give.OfferNew(),
				"OfferNew did not match")
			assert.Equal(t,
				tt.setCurrent, tt.give.SetCurrent(),
				"SetCurrent did not match")
		})
	}
}

// Tests that CreateStack will send an appropriate initial state when it is asked to create a stack with a non-default
// secrets manager.
func TestCreateStack_InitialisesStateWithSecretsManager(t *testing.T) {
	t.Parallel()

	// Arrange.
	_, expectedSm, err := passphrase.NewPassphraseSecretsManager("test-passphrase")
	require.NoError(t, err)

	var actualDeployment apitype.DeploymentV3

	mockBackend := &backend.MockBackend{
		NameF: func() string {
			return "mock"
		},
		ValidateStackNameF: func(name string) error {
			assert.Equal(t, "dev", name, "stack name mismatch")
			return nil
		},
		CreateStackF: func(
			ctx context.Context,
			ref backend.StackReference,
			projectRoot string,
			initialState *apitype.UntypedDeployment,
			opts *backend.CreateStackOptions,
		) (backend.Stack, error) {
			err := json.Unmarshal(initialState.Deployment, &actualDeployment)
			require.NoError(t, err)
			return nil, nil
		},
		DefaultSecretManagerF: func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
			return expectedSm, nil
		},
	}

	stackRef := &backend.MockStackReference{}

	// Act.
	//nolint:errcheck
	CreateStack(
		context.Background(),
		cmdutil.Diag(),
		pkgWorkspace.Instance,
		mockBackend,
		stackRef,
		"",    /*root*/
		nil,   /*opts*/
		false, /*setCurrent*/
		"",    /*secretsProvider*/
		false, /* useRemoteConfig */
	)

	// Assert.
	assert.Equal(t, expectedSm.State(), actualDeployment.SecretsProviders.State)
}

// --- Conflict detection tests ---

// newRemoteStack returns a MockStack configured as a remote config stack.
// It calls LoadRemoteConfig to return an empty ProjectStack (no local config).
func newRemoteStack(t *testing.T, stackName string, escEnv string) *backend.MockStack {
	t.Helper()
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName(stackName)}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &escEnv}
		},
		LoadRemoteF: func(_ context.Context, _ *workspace.Project) (*workspace.ProjectStack, error) {
			return &workspace.ProjectStack{}, nil
		},
	}
}

// writePulumiYaml writes a minimal Pulumi.yaml project file to dir so that
// workspace.DetectProjectStackPath can locate the project root.
func writePulumiYaml(t *testing.T, dir string, projectName string) {
	t.Helper()
	content := "name: " + projectName + "\nruntime: go\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte(content), 0o600))
}

// writeConfigFile writes a Pulumi.<stack>.yaml to dir and chdirs into the temp dir
// so workspace.DetectProjectStackPath resolves there.
func writeConfigFile(t *testing.T, dir string, stackName string, content string) {
	t.Helper()
	name := "Pulumi." + stackName + ".yaml"
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

//nolint:paralleltest // t.Chdir cannot be used with t.Parallel
func TestLoadProjectStack_ConflictDetection_HardErrorWithConfigValues(t *testing.T) {
	dir := t.TempDir()
	writePulumiYaml(t, dir, "myproject")
	// Write a local config file with a non-empty config map.
	writeConfigFile(t, dir, "dev", "config:\n  myproject:host: localhost\n")

	t.Chdir(dir)

	s := newRemoteStack(t, "dev", "myproject/dev")
	project := &workspace.Project{Name: "myproject"}

	_, err := LoadProjectStack(context.Background(), cmdutil.Diag(), project, s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "both remote and local configuration exist")
}

//nolint:paralleltest // t.Chdir cannot be used with t.Parallel
func TestLoadProjectStack_ConflictDetection_HardErrorWithEnvironmentImports(t *testing.T) {
	dir := t.TempDir()
	writePulumiYaml(t, dir, "myproject")
	// Write a local config file that has ESC environment imports.
	writeConfigFile(t, dir, "dev", "environment:\n  - myorg/shared/creds\n")

	t.Chdir(dir)

	s := newRemoteStack(t, "dev", "myproject/dev")
	project := &workspace.Project{Name: "myproject"}

	_, err := LoadProjectStack(context.Background(), cmdutil.Diag(), project, s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "both remote and local configuration exist")
}

//nolint:paralleltest // t.Chdir cannot be used with t.Parallel
func TestLoadProjectStack_ConflictDetection_NoErrorForMetadataOnlyFile(t *testing.T) {
	dir := t.TempDir()
	writePulumiYaml(t, dir, "myproject")
	// Write a local config file with only encryption metadata (no config values).
	writeConfigFile(t, dir, "dev", "secretsprovider: passphrase\nencryptionsalt: v1:abc:def\n")

	t.Chdir(dir)

	s := newRemoteStack(t, "dev", "myproject/dev")
	project := &workspace.Project{Name: "myproject"}

	// Should succeed: metadata-only file does not conflict with remote config.
	ps, err := LoadProjectStack(context.Background(), cmdutil.Diag(), project, s)
	require.NoError(t, err)
	require.NotNil(t, ps)
}

//nolint:paralleltest // t.Chdir cannot be used with t.Parallel
func TestLoadProjectStack_ConflictDetection_NoErrorWhenNoLocalFile(t *testing.T) {
	dir := t.TempDir()
	writePulumiYaml(t, dir, "myproject")
	// No local config file written.
	t.Chdir(dir)

	s := newRemoteStack(t, "dev", "myproject/dev")
	project := &workspace.Project{Name: "myproject"}

	// Should succeed: no local file, no conflict.
	ps, err := LoadProjectStack(context.Background(), cmdutil.Diag(), project, s)
	require.NoError(t, err)
	require.NotNil(t, ps)
}
