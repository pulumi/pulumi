// Copyright 2016-2026, Pulumi Corporation.
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

package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPulumispaceLoadBasic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "Pulumispace.yaml")
	content := `
name: baseinfra
stacks:
  - path: ./vpc
  - path: ./k8s-cluster
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	ps, err := LoadPulumispace(path)
	require.NoError(t, err)
	assert.Equal(t, "baseinfra", ps.Name)
	assert.Len(t, ps.Stacks, 2)
	assert.Equal(t, "./vpc", ps.Stacks[0].Path)
	assert.Equal(t, "./k8s-cluster", ps.Stacks[1].Path)
}

func TestPulumispaceLoadAllFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "Pulumispace.yaml")
	content := `
name: baseinfra
description: Base infrastructure for all regions
stacks:
  - path: ./vpc
    stack: ${STACK}
  - path: ./k8s-cluster
    stack: ${STACK}
  - path: ./rds
    stack: ${STACK}
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	ps, err := LoadPulumispace(path)
	require.NoError(t, err)
	assert.Equal(t, "baseinfra", ps.Name)
	assert.Equal(t, "Base infrastructure for all regions", ps.Description)
	assert.Len(t, ps.Stacks, 3)
	assert.Equal(t, "${STACK}", ps.Stacks[0].Stack)
}

func TestPulumispaceResolveStackVariable(t *testing.T) {
	t.Parallel()

	ps := &Pulumispace{
		Name: "test",
		Stacks: []PulumispaceStack{
			{Path: "./vpc", Stack: "${STACK}"},
			{Path: "./k8s-cluster", Stack: "${STACK}"},
		},
	}

	resolved, err := ps.Resolve("prod-east")
	require.NoError(t, err)
	assert.Equal(t, "prod-east", resolved.Stacks[0].Stack)
	assert.Equal(t, "prod-east", resolved.Stacks[1].Stack)

	// Original should be unchanged.
	assert.Equal(t, "${STACK}", ps.Stacks[0].Stack)
}

func TestPulumispaceResolveNoStackNameError(t *testing.T) {
	t.Parallel()

	ps := &Pulumispace{
		Name: "test",
		Stacks: []PulumispaceStack{
			{Path: "./vpc", Stack: "${STACK}"},
		},
	}

	_, err := ps.Resolve("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "${STACK}")
	assert.Contains(t, err.Error(), "no stack name was provided")
}

func TestPulumispaceValidateEmptyName(t *testing.T) {
	t.Parallel()

	ps := &Pulumispace{
		Stacks: []PulumispaceStack{
			{Path: "./vpc"},
		},
	}

	err := ps.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'name'")
}

func TestPulumispaceValidateEmptyStacks(t *testing.T) {
	t.Parallel()

	ps := &Pulumispace{
		Name:   "test",
		Stacks: []PulumispaceStack{},
	}

	err := ps.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one stack entry")
}

func TestPulumispaceValidateMissingPath(t *testing.T) {
	t.Parallel()

	ps := &Pulumispace{
		Name: "test",
		Stacks: []PulumispaceStack{
			{Path: ""},
		},
	}

	err := ps.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'path'")
}

func TestPulumispaceValidateAbsolutePath(t *testing.T) {
	t.Parallel()

	ps := &Pulumispace{
		Name: "test",
		Stacks: []PulumispaceStack{
			{Path: "/absolute/path"},
		},
	}

	err := ps.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absolute path")
}

func TestPulumispaceValidateDuplicatePaths(t *testing.T) {
	t.Parallel()

	ps := &Pulumispace{
		Name: "test",
		Stacks: []PulumispaceStack{
			{Path: "./vpc"},
			{Path: "./vpc"},
		},
	}

	err := ps.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestPulumispaceHeterogeneousStacks(t *testing.T) {
	t.Parallel()

	ps := &Pulumispace{
		Name: "heterogeneous-example",
		Stacks: []PulumispaceStack{
			{Path: "./vpc", Stack: "networking-prod-east"},
			{Path: "./k8s-cluster", Stack: "${STACK}"},
			{Path: "./rds", Stack: "data-prod-east"},
		},
	}

	resolved, err := ps.Resolve("prod-east")
	require.NoError(t, err)

	assert.Equal(t, "networking-prod-east", resolved.Stacks[0].Stack)
	assert.Equal(t, "prod-east", resolved.Stacks[1].Stack)
	assert.Equal(t, "data-prod-east", resolved.Stacks[2].Stack)
}

func TestPulumispaceLoadResolveValidateRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "Pulumispace.yaml")
	content := `
name: baseinfra
description: Base infrastructure for all regions
stacks:
  - path: ./vpc
    stack: ${STACK}
  - path: ./k8s-cluster
    stack: ${STACK}
  - path: ./rds
    stack: ${STACK}
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	ps, err := LoadPulumispace(path)
	require.NoError(t, err)

	resolved, err := ps.Resolve("prod-east")
	require.NoError(t, err)

	err = resolved.Validate()
	require.NoError(t, err)

	assert.Equal(t, "baseinfra", resolved.Name)
	assert.Equal(t, "prod-east", resolved.Stacks[0].Stack)
	assert.Equal(t, "prod-east", resolved.Stacks[1].Stack)
	assert.Equal(t, "prod-east", resolved.Stacks[2].Stack)
}

func TestPulumispaceLoadJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "Pulumispace.json")
	content := `{
	"name": "baseinfra",
	"stacks": [
		{"path": "./vpc", "stack": "prod"},
		{"path": "./k8s-cluster", "stack": "prod"}
	]
}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	ps, err := LoadPulumispace(path)
	require.NoError(t, err)
	assert.Equal(t, "baseinfra", ps.Name)
	assert.Len(t, ps.Stacks, 2)
	assert.Equal(t, "prod", ps.Stacks[0].Stack)
}

func TestPulumispaceResolveWithoutVariables(t *testing.T) {
	t.Parallel()

	ps := &Pulumispace{
		Name: "test",
		Stacks: []PulumispaceStack{
			{Path: "./vpc", Stack: "networking-prod"},
			{Path: "./k8s-cluster", Stack: "compute-prod"},
		},
	}

	// Even with empty stackName, resolve should succeed when no ${STACK} is used.
	resolved, err := ps.Resolve("")
	require.NoError(t, err)
	assert.Equal(t, "networking-prod", resolved.Stacks[0].Stack)
	assert.Equal(t, "compute-prod", resolved.Stacks[1].Stack)
}

func TestPulumispaceValidateNormalizedDuplicatePaths(t *testing.T) {
	t.Parallel()

	ps := &Pulumispace{
		Name: "test",
		Stacks: []PulumispaceStack{
			{Path: "./vpc"},
			{Path: "vpc"},
		},
	}

	err := ps.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}
