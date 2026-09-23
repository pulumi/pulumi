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

package deploy

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

type deliveryTestClient struct {
	*deploytest.BackendClient
	window    string
	subgroups []int
	closed    []string
}

func (c *deliveryTestClient) CoherenceWindow() string { return c.window }

func (c *deliveryTestClient) CreateCoherenceWindowSubgroup(_ context.Context, size int) (string, error) {
	c.subgroups = append(c.subgroups, size)
	return fmt.Sprintf("subgroup-%d", len(c.subgroups)), nil
}

func (c *deliveryTestClient) CloseCoherenceWindowSubgroup(_ context.Context, subgroupID string) error {
	c.closed = append(c.closed, subgroupID)
	return nil
}

type deliveryRun struct {
	dir  string
	args []string
}

type deliveryTest struct {
	provider *builtinProvider
	client   *deliveryTestClient
	runs     *[]deliveryRun
	fail     *error
}

func newDeliveryTest(t *testing.T, window string) deliveryTest {
	var mu sync.Mutex
	runs := &[]deliveryRun{}
	fail := new(error)
	client := &deliveryTestClient{
		BackendClient: &deploytest.BackendClient{
			GetStackOutputsF: func(_ context.Context, name string, _ func(error) error) (property.Map, error) {
				return property.NewMap(map[string]property.Value{
					"url":      property.New("https://" + name),
					"password": property.New("hunter2").WithSecret(true),
				}), nil
			},
		},
		window: window,
	}
	provider := &builtinProvider{
		backendClient: client,
		diag:          diagtest.LogSink(t),
		runPulumi: func(_ context.Context, dir string, args []string) ([]byte, error) {
			mu.Lock()
			defer mu.Unlock()
			*runs = append(*runs, deliveryRun{dir: dir, args: args})
			return []byte("line 1\nline 2\n"), *fail
		},
	}
	return deliveryTest{provider: provider, client: client, runs: runs, fail: fail}
}

func stackInputs(stack, directory string) resource.PropertyMap {
	return resource.PropertyMap{
		"stack":  resource.NewProperty(stack),
		"source": resource.NewProperty(resource.PropertyMap{"directory": resource.NewProperty(directory)}),
	}
}

func TestBuiltinProviderDelivery(t *testing.T) {
	t.Parallel()

	stackURN := resource.NewURN("stack", "proj", "", deliveryStackType, "app")
	groupURN := resource.NewURN("stack", "proj", "", deliveryStackGroupType, "services")
	abs := func(dir string) string {
		path, err := filepath.Abs(dir)
		require.NoError(t, err)
		return path
	}
	expectedOutputs := func(stack string) resource.PropertyMap {
		return resource.PropertyMap{
			"stack": resource.NewProperty(stack),
			"outputs": resource.NewProperty(resource.PropertyMap{
				"url":      resource.NewProperty("https://" + stack),
				"password": resource.MakeSecret(resource.NewProperty("hunter2")),
			}),
			"secretOutputNames": resource.NewProperty([]resource.PropertyValue{resource.NewProperty("password")}),
		}
	}

	t.Run("Check reports what is missing", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		resp, err := p.Check(t.Context(), plugin.CheckRequest{
			URN:       stackURN,
			NewInputs: property.NewMap(map[string]property.Value{"stack": property.New("org/proj/app")}),
		})
		require.NoError(t, err)
		require.Len(t, resp.Failures, 1)
		assert.Equal(t, `missing required property "source"`, resp.Failures[0].Reason)

		resp, err = p.Check(t.Context(), plugin.CheckRequest{
			URN: groupURN,
			NewInputs: property.NewMap(map[string]property.Value{
				"stacks": property.New([]property.Value{property.New(property.NewMap(map[string]property.Value{
					"stack":  property.New("org/proj/app"),
					"source": property.New(property.NewMap(map[string]property.Value{"directory": property.New(1.0)})),
				}))}),
				"extra": property.New("no"),
			}),
		})
		require.NoError(t, err)
		require.Len(t, resp.Failures, 2)
		assert.Equal(t, `unknown property "extra"`, resp.Failures[0].Reason)
		assert.Equal(t, `stacks[0]: property "source.directory" must be a string`, resp.Failures[1].Reason)
	})

	t.Run("Diff always has work to do", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		inputs := resource.FromResourcePropertyMap(stackInputs("org/proj/app", "../app"))
		resp, err := p.Diff(t.Context(), plugin.DiffRequest{URN: stackURN, OldInputs: inputs, NewInputs: inputs})
		require.NoError(t, err)
		assert.Equal(t, plugin.DiffSome, resp.Changes)
	})

	t.Run("a preview previews the stack in the window", func(t *testing.T) {
		t.Parallel()
		d := newDeliveryTest(t, "window-1")
		resp, err := d.provider.Create(t.Context(), plugin.CreateRequest{
			URN: stackURN, Properties: stackInputs("org/proj/app", "../app"), Preview: true,
		})
		require.NoError(t, err)
		assert.Empty(t, resp.ID)
		assert.Equal(t, expectedOutputs("org/proj/app"), resp.Properties)
		assert.Equal(t, []deliveryRun{{
			dir:  abs("../app"),
			args: []string{"preview", "--stack", "org/proj/app", "--non-interactive", "--coherence-window", "window-1"},
		}}, *d.runs)
	})

	t.Run("an update updates the stack in the window", func(t *testing.T) {
		t.Parallel()
		d := newDeliveryTest(t, "window-1")
		resp, err := d.provider.Update(t.Context(), plugin.UpdateRequest{
			URN: stackURN, NewInputs: stackInputs("org/proj/app", "../app"),
		})
		require.NoError(t, err)
		assert.Equal(t, expectedOutputs("org/proj/app"), resp.Properties)
		assert.Equal(t, []deliveryRun{{
			dir: abs("../app"),
			args: []string{
				"up", "--yes", "--skip-preview", "--stack", "org/proj/app", "--non-interactive",
				"--coherence-window", "window-1",
			},
		}}, *d.runs)
	})

	t.Run("unknown inputs run nothing", func(t *testing.T) {
		t.Parallel()
		d := newDeliveryTest(t, "window-1")
		resp, err := d.provider.Create(t.Context(), plugin.CreateRequest{
			URN: stackURN,
			Properties: resource.PropertyMap{
				"stack":  resource.MakeComputed(resource.NewProperty("")),
				"source": resource.NewProperty(resource.PropertyMap{"directory": resource.NewProperty("../app")}),
			},
			Preview: true,
		})
		require.NoError(t, err)
		assert.Empty(t, *d.runs)
		assert.True(t, resp.Properties["outputs"].IsComputed())
	})

	t.Run("a failed stack reports what it printed", func(t *testing.T) {
		t.Parallel()
		d := newDeliveryTest(t, "window-1")
		*d.fail = errors.New("exit status 255")
		_, err := d.provider.Create(t.Context(), plugin.CreateRequest{
			URN: stackURN, Properties: stackInputs("org/proj/app", "../app"),
		})
		require.ErrorContains(t, err, "updating org/proj/app: exit status 255")
		assert.ErrorContains(t, err, "line 2")
	})

	t.Run("a stack outside a window is refused", func(t *testing.T) {
		t.Parallel()
		d := newDeliveryTest(t, "")
		_, err := d.provider.Create(t.Context(), plugin.CreateRequest{
			URN: stackURN, Properties: stackInputs("org/proj/app", "../app"),
		})
		require.ErrorContains(t, err, "coherence window")
		assert.Empty(t, *d.runs)
	})

	t.Run("a group runs its stacks in a subgroup", func(t *testing.T) {
		t.Parallel()
		d := newDeliveryTest(t, "window-1")
		resp, err := d.provider.Create(t.Context(), plugin.CreateRequest{
			URN: groupURN,
			Properties: resource.PropertyMap{"stacks": resource.NewProperty([]resource.PropertyValue{
				resource.NewProperty(stackInputs("org/proj/db", "../db")),
				resource.NewProperty(stackInputs("org/proj/app", "../app")),
			})},
		})
		require.NoError(t, err)
		assert.Equal(t, []int{2}, d.client.subgroups)
		assert.Equal(t, []string{"subgroup-1"}, d.client.closed)
		require.Len(t, *d.runs, 2)
		for _, run := range *d.runs {
			assert.Equal(t, "subgroup-1", run.args[len(run.args)-1])
			assert.Equal(t, "up", run.args[0])
		}
		assert.Equal(t, resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty(expectedOutputs("org/proj/db")),
			resource.NewProperty(expectedOutputs("org/proj/app")),
		}), resp.Properties["members"])
	})

	t.Run("a group whose stack fails is closed all the same", func(t *testing.T) {
		t.Parallel()
		d := newDeliveryTest(t, "window-1")
		*d.fail = errors.New("exit status 1")
		_, err := d.provider.Create(t.Context(), plugin.CreateRequest{
			URN: groupURN,
			Properties: resource.PropertyMap{"stacks": resource.NewProperty([]resource.PropertyValue{
				resource.NewProperty(stackInputs("org/proj/db", "../db")),
			})},
		})
		require.ErrorContains(t, err, "updating org/proj/db: exit status 1")
		assert.Equal(t, []string{"subgroup-1"}, d.client.closed)
	})

	t.Run("a refresh reads the stacks", func(t *testing.T) {
		t.Parallel()
		d := newDeliveryTest(t, "window-1")
		resp, err := d.provider.Read(t.Context(), plugin.ReadRequest{
			URN: stackURN, ID: "id", Inputs: stackInputs("org/proj/app", "../app"),
		})
		require.NoError(t, err)
		assert.Equal(t, expectedOutputs("org/proj/app"), resp.Outputs)
		assert.Empty(t, *d.runs)
	})
}
