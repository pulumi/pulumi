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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	managedValidYAML = "environment:\n  values:\n    pulumiConfig:\n      proj:a: b\n"
	managedJSON      = `{"environment": {"values": {"pulumiConfig": {"proj:a": "b"}}}}`
	managedNoDef     = "%[1]s has no definition for managed environment acme/proj/renamed. " +
		"Restore it with `pulumi env get acme/proj/renamed --definition` and put the output under " +
		"`environment:` in %[1]s. For a stack that has never been updated, this is enough:\n\n" +
		"  environment:\n" +
		"    values:\n" +
		"      pulumiConfig: {}"
	managedRequiresYAML = "stack-managed environments require a YAML stack file"
)

var managedLoc = backend.StackConfigLocation{ManagedEnvironment: "proj/renamed"}

func loadManagedTestStack(t *testing.T, contents string, json bool) *workspace.ProjectStack {
	marshaler := encoding.YAML
	if json {
		marshaler = encoding.JSON
	}
	ps, err := workspace.LoadProjectStackBytes(diagtest.LogSink(t), &workspace.Project{Name: "proj"},
		[]byte(contents), "Pulumi.dev.yaml", marshaler)
	require.NoError(t, err)
	return ps
}

func TestManagedEnvironmentDefinition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		loc        backend.StackConfigLocation
		contents   string
		json       bool
		configFile string
		wantErr    string
	}{
		{
			name:       "environment deleted, checked before the stack file",
			loc:        backend.StackConfigLocation{ManagedEnvironment: "proj/renamed", ManagedEnvironmentMissing: true},
			contents:   "{}",
			json:       true,
			configFile: "Pulumi.dev.json",
			wantErr: "environment proj/renamed managed by stack dev was deleted. Restore it in ESC (Enterprise). " +
				"A new environment named proj/renamed can't decrypt the existing secrets in stack dev's config.",
		},
		{
			name:       "JSON stack file without a definition, detected by extension",
			loc:        managedLoc,
			contents:   "{}",
			json:       true,
			configFile: "Pulumi.dev.json",
			wantErr:    managedRequiresYAML,
		},
		{
			name:       "empty definition",
			loc:        managedLoc,
			contents:   "environment: {}",
			configFile: "config/Pulumi.dev.yaml",
			wantErr:    fmt.Sprintf(managedNoDef, "config/Pulumi.dev.yaml"),
		},
		{
			name:       "values under config",
			loc:        managedLoc,
			contents:   managedValidYAML + "config:\n  proj:old: x\n",
			configFile: "Pulumi.dev.yaml",
			wantErr: "Pulumi.dev.yaml has values under `config:`, which stacks with an ESC-managed environment " +
				"don't use. Remove the `config:` section, then set each value with `pulumi config set` " +
				"(`--secret` for secrets)",
		},
		{name: "valid definition", loc: managedLoc, contents: managedValidYAML, configFile: "Pulumi.dev.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ps := loadManagedTestStack(t, tt.contents, tt.json)
			node, err := ManagedEnvironmentDefinition(tt.loc, ps, "acme", "dev", tt.configFile)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				assert.Nil(t, node)
				return
			}
			require.NoError(t, err)
			want, err := ps.Environment.DefinitionNode()
			require.NoError(t, err)
			assert.Same(t, want, node)
		})
	}
}

func TestManagedEnvironmentNotSupportedError(t *testing.T) {
	t.Parallel()

	assert.EqualError(t, ManagedEnvironmentNotSupportedError("`pulumi config refresh`"),
		"`pulumi config refresh` is not supported on stacks with an ESC-managed environment yet")
}

//nolint:paralleltest // uses t.Chdir
func TestManagedEnvironmentDefinitionDetectsStackFile(t *testing.T) {
	tests := []struct {
		name        string
		projectFile string
		project     string
		contents    string
		json        bool
		wantFile    string
		wantErr     string
	}{
		{
			name:        "stackConfigDir",
			projectFile: "Pulumi.yaml",
			project:     "name: proj\nruntime: yaml\nstackConfigDir: config\n",
			contents:    "{}",
			wantFile:    filepath.Join("config", "Pulumi.dev.yaml"),
		},
		{
			name:        "yml project",
			projectFile: "Pulumi.yml",
			project:     "name: proj\nruntime: yaml\n",
			contents:    "{}",
			wantFile:    "Pulumi.dev.yml",
		},
		{
			name:        "JSON project",
			projectFile: "Pulumi.json",
			project:     `{"name": "proj", "runtime": "yaml"}`,
			contents:    "{}",
			json:        true,
			wantErr:     managedRequiresYAML,
		},
		{
			name:     "no project, JSON definition detected by content",
			contents: managedJSON,
			json:     true,
			wantErr:  managedRequiresYAML,
		},
		{
			name:     "no project",
			contents: "{}",
			wantErr:  fmt.Sprintf(managedNoDef, "Pulumi.dev.yaml"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			wantErr := tt.wantErr
			if tt.projectFile != "" {
				require.NoError(t, os.WriteFile(tt.projectFile, []byte(tt.project), 0o600))
			}
			if tt.wantFile != "" {
				cwd, err := os.Getwd()
				require.NoError(t, err)
				wantErr = fmt.Sprintf(managedNoDef, filepath.Join(cwd, tt.wantFile))
			}

			ps := loadManagedTestStack(t, tt.contents, tt.json)
			node, err := ManagedEnvironmentDefinition(managedLoc, ps, "acme", "dev", "")
			assert.EqualError(t, err, wantErr)
			assert.Nil(t, node)
		})
	}
}
