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

package newcmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// escStackFixture is a project directory plus a backend that records the environments created for it.
type escStackFixture struct {
	dir      string
	created  map[string]string
	stack    backend.Stack
	project  *workspace.Project
	existing map[string]bool
}

func newESCStackFixture(t *testing.T) *escStackFixture {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "Pulumi.yaml"), []byte("name: payments\nruntime: yaml\n"), 0o600))
	t.Chdir(dir)

	f := &escStackFixture{
		dir:      dir,
		created:  map[string]string{},
		existing: map[string]bool{},
		project:  &workspace.Project{Name: "payments"},
	}

	be := &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{NameF: func() string { return "test" }},
		GetEnvironmentDefinitionF: func(
			_ context.Context, org, envProject, envName, version string,
		) ([]byte, string, int, error) {
			ref := envProject + "/" + envName
			if !f.existing[ref] {
				return nil, "", 0, fmt.Errorf("%w: %v", backend.ErrEnvironmentNotFound, ref)
			}
			return []byte("values: {}\n"), "etag-1", 1, nil
		},
		CreateEnvironmentF: func(
			_ context.Context, org, envProject, envName string, yaml []byte,
		) (apitype.EnvironmentDiagnostics, error) {
			f.created[envProject+"/"+envName] = string(yaml)
			f.existing[envProject+"/"+envName] = true
			return nil, nil
		},
	}
	f.stack = &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV:             "dev",
				NameV:               tokens.MustParseStackName("dev"),
				FullyQualifiedNameV: "acme/payments/dev",
			}
		},
		OrgNameF: func() string { return "acme" },
		BackendF: func() backend.Backend { return be },
	}
	return f
}

// The wizard's config lands in the stack environment, with prompted secrets as `fn::secret`. The stack's
// secrets manager is never involved: no loader is passed at all, and no plaintext reaches the disk.
//
//nolint:paralleltest // changes directory for the process
func TestSaveTemplateConfigToEnvironment(t *testing.T) {
	f := newESCStackFixture(t)

	values := []templateConfigValue{
		{key: config.MustMakeKey("aws", "region"), value: "us-west-2"},
		{key: config.MustMakeKey("payments", "dbPassword"), value: "hunter2", secret: true},
		{key: config.MustMakeKey("payments", "unset"), value: ""},
	}
	commandLineConfig := config.Map{
		config.MustMakeKey("payments", "instanceCount"): config.NewValue("6"),
	}

	var out bytes.Buffer
	require.NoError(t, saveTemplateConfigToEnvironment(
		t.Context(), diag.DefaultSink(&out, &out, diag.FormatOptions{Color: colors.Never}), &out,
		f.project, f.stack, values, commandLineConfig, ""))

	assert.Equal(t, "values: {}\n", f.created["payments/base"])
	assert.Equal(t,
		"imports:\n  - payments/base\n"+
			"values:\n  pulumiConfig:\n"+
			"    aws:region: us-west-2\n"+
			"    payments:dbPassword:\n      fn::secret: hunter2\n"+
			"    payments:instanceCount: \"6\"\n",
		f.created["payments/dev"])

	stackFile, err := os.ReadFile(filepath.Join(f.dir, "Pulumi.dev.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(stackFile), "mainEnvironment: payments/dev")
	assert.NotContains(t, string(stackFile), "config:")
	// Neither the secret nor its ciphertext is ever written to the stack file or the log.
	assert.NotContains(t, string(stackFile), "hunter2")
	assert.NotContains(t, string(stackFile), "encryptionsalt")
	assert.NotContains(t, out.String(), "hunter2")
}

// A --config value wins over a prompted one for the same key, exactly as it does today.
//
//nolint:paralleltest // changes directory for the process
func TestSaveTemplateConfigToEnvironmentPrefersCommandLineConfig(t *testing.T) {
	f := newESCStackFixture(t)

	values := []templateConfigValue{
		{key: config.MustMakeKey("aws", "region"), value: "us-west-2"},
	}
	commandLineConfig := config.Map{
		config.MustMakeKey("aws", "region"): config.NewValue("eu-west-1"),
	}

	require.NoError(t, saveTemplateConfigToEnvironment(
		t.Context(), diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{Color: colors.Never}), &bytes.Buffer{},
		f.project, f.stack, values, commandLineConfig, ""))

	assert.Contains(t, f.created["payments/dev"], "aws:region: eu-west-1")
	assert.NotContains(t, f.created["payments/dev"], "us-west-2")
}

// A run with no config at all still gives the stack its environments and its `mainEnvironment`.
//
//nolint:paralleltest // changes directory for the process
func TestSaveTemplateConfigToEnvironmentWithoutConfig(t *testing.T) {
	f := newESCStackFixture(t)

	var out bytes.Buffer
	require.NoError(t, saveTemplateConfigToEnvironment(
		t.Context(), diag.DefaultSink(&out, &out, diag.FormatOptions{Color: colors.Never}), &out,
		f.project, f.stack, nil, nil, ""))

	assert.Equal(t, "imports:\n  - payments/base\nvalues: {}\n", f.created["payments/dev"])
	assert.NotContains(t, out.String(), "Saved config to")

	stackFile, err := os.ReadFile(filepath.Join(f.dir, "Pulumi.dev.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(stackFile), "mainEnvironment: payments/dev")
}

// An existing stack environment is reused, never overwritten by the values the wizard gathered.
//
//nolint:paralleltest // changes directory for the process
func TestSaveTemplateConfigToEnvironmentReusesExistingEnvironment(t *testing.T) {
	f := newESCStackFixture(t)
	f.existing["payments/base"] = true
	f.existing["payments/dev"] = true

	var out bytes.Buffer
	require.NoError(t, saveTemplateConfigToEnvironment(
		t.Context(), diag.DefaultSink(&out, &out, diag.FormatOptions{Color: colors.Never}), &out,
		f.project, f.stack,
		[]templateConfigValue{{key: config.MustMakeKey("aws", "region"), value: "us-west-2"}},
		nil, ""))

	assert.Empty(t, f.created)
	assert.Contains(t, out.String(), "Environment 'acme/payments/dev' already exists — reusing.")
}
