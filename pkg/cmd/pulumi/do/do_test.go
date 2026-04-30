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
	"encoding/json"
	"io"
	"testing"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func panicLoader(context.Context, diag.Sink, string, string) (io.Closer, plugin.Provider, error) {
	panic("not implemented")
}

func TestDoCmdNoArgsPrintsHelp(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, panicLoader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	table := []struct {
		name string
		args []string
	}{
		{name: "no args", args: []string{}},
		{name: "with --help", args: []string{"--help"}},
	}

	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			require.NoError(t, err)

			output := stdout.String()
			assert.Contains(t, output, "Interact with any cloud")
			assert.Contains(t, output, "Usage:")
			assert.Contains(t, output, "pulumi do")
		})
	}
}

type nopCloser struct {
	closed bool
}

func (nc *nopCloser) Close() error {
	nc.closed = true
	return nil
}

func closer(t *testing.T) io.Closer {
	c := &nopCloser{}
	// Always assert that the plugin context is closed by the end of the test.
	t.Cleanup(func() {
		assert.True(t, c.closed, "expected closer to be closed")
	})
	return c
}

// TODO: CALL CONFIGURE
// GET PROVIDER OPTIONS
// ADD DRY_RUN

type testProvider struct {
	plugin.MockProvider
	spec schema.PackageSpec
}

func (p *testProvider) GetSchema(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
	schemaBytes, err := json.Marshal(p.spec)
	if err != nil {
		return plugin.GetSchemaResponse{}, err
	}
	schema := string(schemaBytes)
	return plugin.GetSchemaResponse{Schema: []byte(schema)}, nil
}

func TestDoCmdWithPkgArgPrintsHelp(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
		assert.Equal(t, "aws@4.1", source)
		spec := schema.PackageSpec{
			Name: "aws",
			Functions: map[string]schema.FunctionSpec{
				"aws:index:myFunction":         {},
				"aws:myModule:myOtherFunction": {},
			},
			Resources: map[string]schema.ResourceSpec{
				"aws:index:myResource":         {},
				"aws:myModule:myOtherResource": {},
			},
		}
		return closer(t), &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"aws@4.1"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `Interact with aws resources and functions.

Run 'pulumi do aws@4.1 <module/resource/function> --help' for more details on usage.

Usage:
  do aws@4.1 [command]

Functions
  myFunction  Invoke the myFunction function

Resources
  myResource  Operate on the myResource resource

Modules
  myModule    Functions and resources for the myModule module

Flags:
  -h, --help   help for aws@4.1

Use "do aws@4.1 [command] --help" for more information about a command.
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdWithModuleArgPrintsHelp(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
		assert.Equal(t, "aws@4.1", source)
		spec := schema.PackageSpec{
			Name: "aws",
			Functions: map[string]schema.FunctionSpec{
				"aws:index:myFunction":         {},
				"aws:myModule:myOtherFunction": {},
			},
			Resources: map[string]schema.ResourceSpec{
				"aws:index:myResource":         {},
				"aws:myModule:myOtherResource": {},
			},
		}
		return closer(t), &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"aws@4.1", "myModule"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `Functions and resources for the myModule module.

Run 'pulumi do aws@4.1 myModule <resource/function> --help' for more details on usage.

Usage:
  do aws@4.1 myModule [command]

Functions
  myOtherFunction Invoke the myOtherFunction function

Resources
  myOtherResource Operate on the myOtherResource resource

Flags:
  -h, --help   help for myModule

Use "do aws@4.1 myModule [command] --help" for more information about a command.
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdWithFunctionHelpArgPrintsHelp(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:myModule:myOtherFunction": {
					Description: "This is the other function in this package.",
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param1": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
								Description: "To set param1 things",
							},
						},
					},
				},
			},
		}
		return closer(t), &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure", "myModule", "myOtherFunction", "--help"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `Invoke the myOtherFunction function.

This is the other function in this package.

Usage:
  do azure myModule myOtherFunction [flags]

Flags:
  -h, --help            help for myOtherFunction
      --param1 string   To set param1 things
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvoke(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param1": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
							"param2": {
								TypeSpec: schema.TypeSpec{
									Type: "number",
								},
							},
							"param3": {
								TypeSpec: schema.TypeSpec{
									Type: "boolean",
								},
							},
						},
					},
				},
			},
		}
		return closer(t), &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.False(t, req.Preview, "expected Preview to be false")
					assert.Equal(t, "azure:index:myFunction", string(req.Tok))
					assert.Equal(t, "hello", req.Args["param1"].StringValue())
					assert.Equal(t, 42.0, req.Args["param2"].NumberValue())
					assert.Equal(t, true, req.Args["param3"].BoolValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"output1": resource.NewProperty("world"),
							"output2": resource.NewProperty(43.0),
							"output3": resource.NewProperty(false),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure", "myFunction", "--param1", "hello", "--param2", "42", "--param3"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "output1": "world",
  "output2": 43,
  "output3": false
}
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvoke_MissingRequiredFlag(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param1": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
							"param2": {
								TypeSpec: schema.TypeSpec{
									Type: "number",
								},
							},
							"param3": {
								TypeSpec: schema.TypeSpec{
									Type: "boolean",
								},
							},
						},
						Required: []string{"param1"},
					},
				},
			},
		}
		return closer(t), &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure", "myFunction"})
	err := cmd.Execute()
	require.ErrorContains(t, err, "missing required parameter --param1")
}

func TestDoCmdFunctionInvokeDryRun(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param1": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
							"param2": {
								TypeSpec: schema.TypeSpec{
									Type: "number",
								},
							},
							"param3": {
								TypeSpec: schema.TypeSpec{
									Type: "boolean",
								},
							},
						},
					},
				},
			},
		}
		return closer(t), &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Truef(t, req.Preview, "expected Preview to be true")
					assert.Equal(t, "azure:index:myFunction", string(req.Tok))
					assert.Equal(t, "hello", req.Args["param1"].StringValue())
					assert.Equal(t, 42.0, req.Args["param2"].NumberValue())
					assert.Equal(t, true, req.Args["param3"].BoolValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"output1": resource.NewProperty("world"),
							"output2": resource.NewProperty(43.0),
							"output3": resource.MakeComputed(resource.NewProperty("")),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"--dry-run", "azure", "myFunction", "--param1", "hello", "--param2", "42", "--param3"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "output1": "world",
  "output2": 43,
  "output3": "<unknown>"
}
`
	assert.Equal(t, expected, stdout.String())
}
