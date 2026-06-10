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

package lifecycletest

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/blang/semver"
	"github.com/gofrs/uuid"

	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// pclSnippetTestProvider returns a loader for a single-resource "pkgA" provider whose schema is
// passed in by the caller. The provider counts create/update/delete calls into the supplied slices.
func pclSnippetTestProvider(
	schema string, created, updated, deleted *[]resource.URN,
) []*deploytest.ProviderLoader {
	return []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(ctx context.Context, _ plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					return plugin.GetSchemaResponse{Schema: []byte(schema)}, nil
				},
				CreateF: func(ctx context.Context, cr plugin.CreateRequest) (plugin.CreateResponse, error) {
					if created != nil {
						*created = append(*created, cr.URN)
					}
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}
					id := uuid.String()
					if cr.Preview {
						id = ""
					}
					return plugin.CreateResponse{ID: resource.ID(id), Properties: cr.Properties}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldInputs.DeepEquals(req.NewInputs) {
						return plugin.DiffResult{Changes: plugin.DiffSome}, nil
					}
					return plugin.DiffResult{}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					if updated != nil {
						*updated = append(*updated, req.URN)
					}
					return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if deleted != nil {
						*deleted = append(*deleted, req.URN)
					}
					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}
}

const pclSnippetSchemaPropA = `{
  "version": "0.0.1",
  "name": "pkgA",
  "resources": {
    "pkgA:index:res": {
      "inputProperties": {
        "propA": { "type": "boolean" }
      },
      "requiredInputs": ["propA"]
    }
  }
}`

// TestPclSnippet checks that we can run a PCL snippet via an engine update.
func TestPclSnippet(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(ctx context.Context, gsr plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					return plugin.GetSchemaResponse{Schema: []byte(`{
  "version": "0.0.1",
  "name": "pkgA",
  "resources": {
    "pkgA:index:res": {
      "inputProperties": {
        "propA": { "type": "boolean" }
      },
      "requiredInputs": ["propA"]
    }
  }
}`)}, nil
				},
				CreateF: func(ctx context.Context, cr plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}
					id := uuid.String()
					if cr.Preview {
						id = ""
					}
					return plugin.CreateResponse{
						ID:         resource.ID(id),
						Properties: cr.Properties,
					}, nil
				},
			}, nil
		}),
	}

	snippets := []resource.Snippet{
		{
			Name: "test-resource", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			Code:       `propA = true`,
		},
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	// Make an empty snapshot.
	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Add a snippet to the snapshot and rerun the update to execute it.
	snap.Snippets = snippets
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	// Check that the snippet is still present in the snapshot.
	require.Len(t, snap.Snippets, 1)
	require.Equal(t, `test-resource`, snap.Snippets[0].Name)
	require.Equal(t, `pkgA:index:res`, snap.Snippets[0].Type)
	require.Equal(t, `propA = true`, snap.Snippets[0].Code)

	// Check the resource was created.
	require.Len(t, snap.Resources, 2)
	require.Equal(t, tokens.Type("pulumi:providers:pkgA"), snap.Resources[0].Type)
	require.Equal(t, tokens.Type("pkgA:index:res"), snap.Resources[1].Type)
	require.Equal(t, resource.PropertyMap{"propA": resource.NewProperty(true)}, snap.Resources[1].Inputs)
}

// TestPclInvalidSnippet checks that an invalid snippet (i.e does not type check) returns an error.
func TestPclInvalidSnippet(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(ctx context.Context, gsr plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					return plugin.GetSchemaResponse{Schema: []byte(`{
  "version": "0.0.1",
  "name": "pkgA",
  "resources": {
    "pkgA:index:res": {
      "inputProperties": {
        "propA": { "type": "boolean" },
        "propB": { "type": "string" }
      },
      "requiredInputs": ["propA", "propB"]
    }
  }
}`)}, nil
				},
				CreateF: func(ctx context.Context, cr plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}
					id := uuid.String()
					if cr.Preview {
						id = ""
					}
					return plugin.CreateResponse{
						ID:         resource.ID(id),
						Properties: cr.Properties,
					}, nil
				},
			}, nil
		}),
	}

	snippets := []resource.Snippet{
		// only set one property
		{
			Name: "test-resource", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			Code:       `propA = true`,
		},
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	// Make an empty snapshot.
	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Add a snippet to the snapshot and rerun the update to execute it.
	snap.Snippets = snippets
	_, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	// The binder now type-checks the snippet body against the loaded schema before any
	// RegisterResource call goes out, so the missing required input is caught at bind time
	// rather than by the resource monitor.
	require.ErrorContains(t, err, "missing required attribute 'propB'")
}

// TestPclSnippetUpdate checks that mutating the code on a snippet between updates causes the
// underlying resource to be updated rather than recreated.
func TestPclSnippetUpdate(t *testing.T) {
	t.Parallel()

	var created, updated, deleted []resource.URN
	loaders := pclSnippetTestProvider(pclSnippetSchemaPropA, &created, &updated, &deleted)

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	snap.Snippets = []resource.Snippet{
		{
			Name: "test-resource", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			Code:       `propA = true`,
		},
	}
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.Len(t, created, 1)
	require.Empty(t, updated)
	require.Equal(t, resource.PropertyMap{"propA": resource.NewProperty(true)}, snap.Resources[1].Inputs)

	// Change the snippet code and rerun: we expect an Update, not a Create or Delete.
	snap.Snippets[0].Code = `propA = false`
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	require.NoError(t, err)
	require.Len(t, created, 1, "resource should not be recreated")
	require.Len(t, updated, 1, "resource should be updated once")
	require.Empty(t, deleted)
	require.Equal(t, resource.PropertyMap{"propA": resource.NewProperty(false)}, snap.Resources[1].Inputs)
}

// TestPclSnippetDelete checks that removing a snippet from the snapshot causes the engine to
// delete the resource it had previously synthesized.
func TestPclSnippetDelete(t *testing.T) {
	t.Parallel()

	var created, updated, deleted []resource.URN
	loaders := pclSnippetTestProvider(pclSnippetSchemaPropA, &created, &updated, &deleted)

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	snap.Snippets = []resource.Snippet{
		{
			Name: "test-resource", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			Code:       `propA = true`,
		},
	}
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.Len(t, snap.Resources, 2)
	require.Len(t, created, 1)

	// Remove the snippet and rerun. The synthesized resource should be deleted; the default
	// provider for pkgA should also disappear since nothing references it.
	snap.Snippets = nil
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	require.NoError(t, err)
	require.Empty(t, snap.Snippets)
	require.Empty(t, updated)
	require.Len(t, deleted, 1)
	for _, r := range snap.Resources {
		require.NotEqual(t, tokens.Type("pkgA:index:res"), r.Type, "snippet resource should be gone")
	}
}

// TestPclMultipleSnippets checks that several snippets in a snapshot each produce their own
// resource and that all of them survive a round trip.
func TestPclMultipleSnippets(t *testing.T) {
	t.Parallel()

	loaders := pclSnippetTestProvider(pclSnippetSchemaPropA, nil, nil, nil)
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	snap.Snippets = []resource.Snippet{
		{
			Name: "r1", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			Code:       `propA = true`,
		},
		{
			Name: "r2", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			Code:       `propA = false`,
		},
	}
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.Len(t, snap.Snippets, 2)

	// Index resources by URN name so the assertions don't depend on engine ordering.
	resourcesByName := map[string]*resource.State{}
	for _, r := range snap.Resources {
		if r.Type == "pkgA:index:res" {
			resourcesByName[r.URN.Name()] = r
		}
	}
	require.Len(t, resourcesByName, 2)
	require.Equal(t,
		resource.PropertyMap{"propA": resource.NewProperty(true)},
		resourcesByName["r1"].Inputs)
	require.Equal(t,
		resource.PropertyMap{"propA": resource.NewProperty(false)},
		resourcesByName["r2"].Inputs)
}

// TestPclSnippetBuiltins exercises the PCL evaluator wired up for a snippet, verifying that it has
// access to context (project, stack, organization) and trivial builtins (max, length).
func TestPclSnippetBuiltins(t *testing.T) {
	t.Parallel()

	schemaJSON := `{
  "version": "0.0.1",
  "name": "pkgA",
  "resources": {
    "pkgA:index:res": {
      "inputProperties": {
        "projectName":  { "type": "string" },
        "stackName":    { "type": "string" },
        "orgName":      { "type": "string" },
        "maxValue":     { "type": "integer" },
        "lengthValue":  { "type": "integer" }
      },
      "requiredInputs": [
        "projectName", "stackName", "orgName",
        "maxValue", "lengthValue"
      ]
    }
  },
  "functions": {
    "pkgA:index:echo": {
      "inputs": {
        "properties": {
          "input": { "type": "string" }
        }
      },
      "outputs": {
        "properties": {
          "result": { "type": "string" }
        },
        "required": ["result"]
      }
    }
  }
}`

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(_ context.Context, _ plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					return plugin.GetSchemaResponse{Schema: []byte(schemaJSON)}, nil
				},
				CreateF: func(_ context.Context, cr plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}
					id := uuid.String()
					if cr.Preview {
						id = ""
					}
					return plugin.CreateResponse{ID: resource.ID(id), Properties: cr.Properties}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})

	p := &lt.TestPlan{
		Project: "my-project",
		Stack:   "my-stack",
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	snap.Snippets = []resource.Snippet{
		{
			Name: "test-resource", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			Code: `projectName = project()
stackName = stack()
orgName = organization()
maxValue = max(1, 7, 3)
lengthValue = length(["a", "b", "c", "d"])`,
		},
	}

	target := p.GetTarget(t, snap)
	target.Organization = "my-org"
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), target, p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	// Find the synthesized resource.
	var res *resource.State
	for _, r := range snap.Resources {
		if r.Type == "pkgA:index:res" {
			res = r
			break
		}
	}
	require.NotNil(t, res, "snippet resource should have been created")
	require.Equal(t, resource.PropertyMap{
		"projectName": resource.NewProperty("my-project"),
		"stackName":   resource.NewProperty("my-stack"),
		"orgName":     resource.NewProperty("my-org"),
		"maxValue":    resource.NewProperty(7.0),
		"lengthValue": resource.NewProperty(4.0),
	}, res.Inputs)
}

// TestPclSnippetDirectories checks that the cwd() and rootDirectory() builtins are wired through to
// the snippet evaluator. In the lifecycle test framework the project root is "/", so both should
// resolve to that.
func TestPclSnippetDirectories(t *testing.T) {
	t.Parallel()

	schemaJSON := `{
  "version": "0.0.1",
  "name": "pkgA",
  "resources": {
    "pkgA:index:res": {
      "inputProperties": {
        "workingDir": { "type": "string" },
        "rootDir":    { "type": "string" }
      },
      "requiredInputs": ["workingDir", "rootDir"]
    }
  }
}`

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(_ context.Context, _ plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					return plugin.GetSchemaResponse{Schema: []byte(schemaJSON)}, nil
				},
				CreateF: func(_ context.Context, cr plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}
					id := uuid.String()
					if cr.Preview {
						id = ""
					}
					return plugin.CreateResponse{ID: resource.ID(id), Properties: cr.Properties}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	snap.Snippets = []resource.Snippet{
		{
			Name: "test-resource", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			Code: `workingDir = cwd()
rootDir = rootDirectory()`,
		},
	}

	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	var res *resource.State
	for _, r := range snap.Resources {
		if r.Type == "pkgA:index:res" {
			res = r
			break
		}
	}
	require.NotNil(t, res, "snippet resource should have been created")
	require.Equal(t, resource.PropertyMap{
		"workingDir": resource.NewProperty("/"),
		"rootDir":    resource.NewProperty("/"),
	}, res.Inputs)
}

// TestPclSnippetInvoke checks that a snippet can call an invoke against the resource monitor and use
// the returned value as a resource input.
func TestPclSnippetInvoke(t *testing.T) {
	t.Parallel()

	schemaJSON := `{
  "version": "0.0.1",
  "name": "pkgA",
  "resources": {
    "pkgA:index:res": {
      "inputProperties": {
        "message": { "type": "string" }
      },
      "requiredInputs": ["message"]
    }
  },
  "functions": {
    "pkgA:index:echo": {
      "inputs": {
        "properties": {
          "input": { "type": "string" }
        },
        "required": ["input"]
      },
      "outputs": {
        "properties": {
          "result": { "type": "string" }
        },
        "required": ["result"]
      }
    }
  }
}`

	var invoked []string
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(_ context.Context, _ plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					return plugin.GetSchemaResponse{Schema: []byte(schemaJSON)}, nil
				},
				CreateF: func(_ context.Context, cr plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}
					id := uuid.String()
					if cr.Preview {
						id = ""
					}
					return plugin.CreateResponse{ID: resource.ID(id), Properties: cr.Properties}, nil
				},
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					input := req.Args["input"].StringValue()
					invoked = append(invoked, input)
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"result": resource.NewProperty("echoed: " + input),
						},
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	snap.Snippets = []resource.Snippet{
		{
			Name: "test-resource", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			Code:       `message = invoke("pkgA:index:echo", { input = "hi" }).result`,
		},
	}

	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	require.Equal(t, []string{"hi"}, invoked, "invoke should be called exactly once with input \"hi\"")

	var res *resource.State
	for _, r := range snap.Resources {
		if r.Type == "pkgA:index:res" {
			res = r
			break
		}
	}
	require.NotNil(t, res, "snippet resource should have been created")
	require.Equal(t, resource.PropertyMap{
		"message": resource.NewProperty("echoed: hi"),
	}, res.Inputs)
}

// TestPclSnippetResourceReference checks that a snippet can reference a resource registered by the
// main program by its logical name (declared in Snippet.References) and read one of its output
// properties as an input to the snippet's own resource.
// TestPclSnippetResourceReference covers a snippet whose References point at a custom resource the
// main program has already registered. Because the snippet integrity check requires every
// referenced URN to be present in the snapshot, this test sets the scene with a first update that
// only registers the program-owned resource (no snippet yet), and only on the second update —
// after that resource is persisted — does the snippet appear. The snippet then evaluates against
// an already-known URN and the integrity check passes throughout. The create case (the snippet's
// first appearance with broker-mediated references not yet in the snapshot) is deferred to a
// follow-up alongside the engine options that add/remove snippets atomically.
func TestPclSnippetResourceReference(t *testing.T) {
	t.Parallel()

	schemaJSON := `{
  "version": "0.0.1",
  "name": "pkgA",
  "resources": {
    "pkgA:index:res": {
      "inputProperties": {
        "message": { "type": "string" }
      },
      "requiredInputs": ["message"]
    },
    "pkgA:index:producer": {
      "inputProperties": {
        "seed": { "type": "string" }
      },
      "requiredInputs": ["seed"],
      "properties": {
        "value": { "type": "string" }
      },
      "required": ["value"]
    }
  }
}`

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(_ context.Context, _ plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					return plugin.GetSchemaResponse{Schema: []byte(schemaJSON)}, nil
				},
				CreateF: func(_ context.Context, cr plugin.CreateRequest) (plugin.CreateResponse, error) {
					out := resource.PropertyMap{}
					for k, v := range cr.Properties {
						out[k] = v
					}
					// Producer synthesises `value` from the seed input so the consumer snippet has
					// something deterministic to observe via the broker.
					if seed, ok := cr.Properties["seed"]; ok {
						out["value"] = resource.NewProperty("value-of-" + seed.StringValue())
					}
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}
					id := uuid.String()
					if cr.Preview {
						id = ""
					}
					return plugin.CreateResponse{ID: resource.ID(id), Properties: out}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:index:producer", "producer", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{"seed": resource.NewProperty("hello")},
		})
		return err
	})

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	// Step 0: register the producer via the program. The resulting snapshot has the producer in
	// its Resources, so the snippet we add next can legally reference its URN.
	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	const producerURN = "urn:pulumi:test::test::pkgA:index:producer::producer"
	snap.Snippets = []resource.Snippet{
		{
			Name: "test-resource", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			References: map[string]string{
				"producer": producerURN,
			},
			Code: `message = producer.value`,
		},
	}

	// Step 1: the snippet enters the snapshot alongside an already-known producer URN. This is
	// the "update" path the snippet integrity check is happy with.
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	var res *resource.State
	for _, r := range snap.Resources {
		if r.Type == "pkgA:index:res" {
			res = r
			break
		}
	}
	require.NotNil(t, res, "snippet resource should have been created")
	require.Equal(t, resource.PropertyMap{
		"message": resource.NewProperty("value-of-hello"),
	}, res.Inputs)
}

// TestPclSnippetReferenceLocalComponent covers a snippet whose References point at a local
// component (Remote=false) whose outputs arrive via RegisterResourceOutputs rather than via the
// initial RegisterResource call. The broker must defer publication until RegisterResourceOutputs
// fires, otherwise the snippet would either hang waiting for the URN or read an empty outputs
// snapshot that pre-dates the program's `RegisterResourceOutputs` call.
func TestPclSnippetReferenceLocalComponent(t *testing.T) {
	t.Parallel()

	schemaJSON := `{
  "version": "0.0.1",
  "name": "pkgA",
  "resources": {
    "pkgA:index:res": {
      "inputProperties": {
        "message": { "type": "string" }
      },
      "requiredInputs": ["message"]
    },
    "pkgA:index:Comp": {
      "isComponent": true,
      "inputProperties": {
        "seed": { "type": "string" }
      },
      "requiredInputs": ["seed"],
      "properties": {
        "value": { "type": "string" }
      },
      "required": ["value"]
    }
  }
}`

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(_ context.Context, _ plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					return plugin.GetSchemaResponse{Schema: []byte(schemaJSON)}, nil
				},
			}, nil
		}),
	}

	// Program: register the component locally (Remote=false), then publish its outputs via
	// RegisterResourceOutputs. The broker must wait until that second call before resolving any
	// pending references to comp's URN.
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:index:Comp", "comp", false, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{"seed": resource.NewProperty("hello")},
		})
		if err != nil {
			return err
		}
		return monitor.RegisterResourceOutputs(resp.URN, resource.PropertyMap{
			"value": resource.NewProperty("value-of-hello"),
		})
	})

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	// Step 0: register the component via the program. Its URN ends up in the snapshot's resources
	// so the snippet we add next can reference it.
	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	const compURN = "urn:pulumi:test::test::pkgA:index:Comp::comp"
	snap.Snippets = []resource.Snippet{
		{
			Name: "test-resource", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			References: map[string]string{
				"comp": compURN,
			},
			Code: `message = comp.value`,
		},
	}

	// Step 1: the snippet enters the snapshot. The program still re-registers comp on each update,
	// so the broker resolves comp's URN once RegisterResourceOutputs fires for it.
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	var res *resource.State
	for _, r := range snap.Resources {
		if r.Type == "pkgA:index:res" {
			res = r
			break
		}
	}
	require.NotNil(t, res, "snippet resource should have been created")
	require.Equal(t, resource.PropertyMap{
		"message": resource.NewProperty("value-of-hello"),
	}, res.Inputs)
}

// TestPclSnippetMissingProgramReference covers a snippet whose reference URN was registered by the
// program on one update but not on a subsequent update. The second update must fail with a
// "no source registered" error from the broker watchdog rather than hanging.
//
// As with TestPclSnippetResourceReference, the snippet is introduced on its second update — after
// the producer has already been registered and persisted — so the strict integrity check is happy
// with the steady state. The create-case (snippet introduced alongside its references) is deferred
// to a follow-up alongside the engine options that add/remove snippets atomically.
func TestPclSnippetMissingProgramReference(t *testing.T) {
	t.Parallel()

	schemaJSON := `{
  "version": "0.0.1",
  "name": "pkgA",
  "resources": {
    "pkgA:index:res": {
      "inputProperties": {
        "message": { "type": "string" }
      },
      "requiredInputs": ["message"]
    },
    "pkgA:index:producer": {
      "inputProperties": {
        "seed": { "type": "string" }
      },
      "requiredInputs": ["seed"],
      "properties": {
        "value": { "type": "string" }
      },
      "required": ["value"]
    }
  }
}`

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(_ context.Context, _ plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					return plugin.GetSchemaResponse{Schema: []byte(schemaJSON)}, nil
				},
				CreateF: func(_ context.Context, cr plugin.CreateRequest) (plugin.CreateResponse, error) {
					out := resource.PropertyMap{}
					for k, v := range cr.Properties {
						out[k] = v
					}
					if seed, ok := cr.Properties["seed"]; ok {
						out["value"] = resource.NewProperty("value-of-" + seed.StringValue())
					}
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}
					id := uuid.String()
					if cr.Preview {
						id = ""
					}
					return plugin.CreateResponse{ID: resource.ID(id), Properties: out}, nil
				},
			}, nil
		}),
	}

	// programRegistersProducer lets the test toggle whether the language host registers the
	// custom resource the snippet depends on. Atomic so a hypothetical concurrent program
	// execution still observes a coherent value.
	var programRegistersProducer atomic.Bool
	programRegistersProducer.Store(true)
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if !programRegistersProducer.Load() {
			return nil
		}
		_, err := monitor.RegisterResource("pkgA:index:producer", "producer", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{"seed": resource.NewProperty("hello")},
		})
		return err
	})

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			SkipDisplayTests: true,
			T:                t,
			HostF:            deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	// Step 0: register producer via the program. No snippet yet.
	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	const producerURN = "urn:pulumi:test::test::pkgA:index:producer::producer"
	snap.Snippets = []resource.Snippet{
		{
			Name: "test-resource", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			References: map[string]string{
				"producer": producerURN,
			},
			Code: `message = producer.value`,
		},
	}

	// Step 1: snippet enters the snapshot alongside an already-known producer URN. Update succeeds.
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	var res *resource.State
	for _, r := range snap.Resources {
		if r.Type == "pkgA:index:res" {
			res = r
			break
		}
	}
	require.NotNil(t, res, "snippet resource should have been created on the first run")
	require.Equal(t, resource.PropertyMap{"message": resource.NewProperty("value-of-hello")}, res.Inputs)

	// Step 2: program no longer registers producer. The snippet's reference URN won't be
	// resolved; the broker watchdog reaps it and the update fails with a clear error rather than
	// hanging. The integrity check doesn't fire here because snippet evaluation bails out before
	// the engine has a chance to mutate the snapshot.
	programRegistersProducer.Store(false)
	_, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	require.ErrorContains(t, err, "no source registered this URN")
}

// TestPclSnippetMissingSnippetReference covers a snippet whose reference URN is registered by another
// snippet on one update but not on a subsequent update. The first run succeeds; the second run, with the
// producing snippet removed from the snapshot, must error out with a "no source registered" message.
func TestPclSnippetMissingSnippetReference(t *testing.T) {
	t.Parallel()

	schemaJSON := `{
  "version": "0.0.1",
  "name": "pkgA",
  "resources": {
    "pkgA:index:res": {
      "inputProperties": {
        "message": { "type": "string" }
      },
      "requiredInputs": ["message"]
    },
    "pkgA:index:producer": {
      "inputProperties": {
        "seed": { "type": "string" }
      },
      "requiredInputs": ["seed"],
      "properties": {
        "value": { "type": "string" }
      },
      "required": ["value"]
    }
  }
}`

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(_ context.Context, _ plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					return plugin.GetSchemaResponse{Schema: []byte(schemaJSON)}, nil
				},
				CreateF: func(_ context.Context, cr plugin.CreateRequest) (plugin.CreateResponse, error) {
					out := resource.PropertyMap{}
					for k, v := range cr.Properties {
						out[k] = v
					}
					// Producer: synthesize a "value" output derived from the seed input so the consumer can
					// observe it in its broker-resolved outputs.
					if seed, ok := cr.Properties["seed"]; ok {
						out["value"] = resource.NewProperty("value-of-" + seed.StringValue())
					}
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}
					id := uuid.String()
					if cr.Preview {
						id = ""
					}
					return plugin.CreateResponse{ID: resource.ID(id), Properties: out}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			SkipDisplayTests: true,
			T:                t,
			HostF:            deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	// Step 0: empty update.
	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	const producerURN = "urn:pulumi:test::test::pkgA:index:producer::producer"
	producerSnippet := resource.Snippet{
		Name: "producer", Type: "pkgA:index:producer",
		Descriptor: resource.PackageDescriptor{Name: "pkgA"},
		Code:       `seed = "abc"`,
	}
	consumerSnippet := resource.Snippet{
		Name: "consumer", Type: "pkgA:index:res",
		Descriptor: resource.PackageDescriptor{Name: "pkgA"},
		References: map[string]string{"producer": producerURN},
		Code:       `message = producer.value`,
	}

	// Step 1: introduce only the producer snippet — no consumer yet. The producer resource is
	// persisted on this update, so once the consumer snippet appears in step 2 its reference URN
	// is already in the snapshot.
	snap.Snippets = []resource.Snippet{producerSnippet}
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	// Step 2: now add the consumer snippet too. Both snippets evaluate; the consumer reads
	// producer.value via the broker.
	snap.Snippets = []resource.Snippet{producerSnippet, consumerSnippet}
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	require.NoError(t, err)

	var consumer *resource.State
	for _, r := range snap.Resources {
		if r.URN.Name() == "consumer" {
			consumer = r
			break
		}
	}
	require.NotNil(t, consumer, "consumer snippet resource should have been created")
	require.Equal(t, resource.PropertyMap{"message": resource.NewProperty("value-of-abc")}, consumer.Inputs)

	// Step 3: drop the producer snippet, keep the consumer. The producer resource is still in the
	// snapshot at the start of the update so integrity is happy, but with nothing registering its
	// URN the broker watchdog reaps the consumer's pending reference and the update fails with a
	// clear error.
	snap.Snippets = []resource.Snippet{consumerSnippet}
	_, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "3")
	require.ErrorContains(t, err, "no source registered this URN")
}

// TestPclSnippetReferenceFollowsAlias exercises the alias-renamed-producer flow. A snippet declares a
// reference to a URN that the program registers under name "A". On a subsequent update the program
// registers the resource as "B" with `AliasURNs: ["...::A"]`, signalling that B is the renamed-A; the
// snippet should still resolve its reference (the broker should treat the alias as equivalent to the
// canonical URN) and the saved snippet's References should be rewritten to point at B.
//
// This test is forward-looking — it captures the intended behaviour. Two pieces are still missing:
//
//  1. Broker-side alias resolution: when the resmon processes a RegisterResource with aliases, it
//     should call broker.Resolve(aliasURN, outputs) for each alias so any consumer blocked on the
//     pre-rename URN wakes up.
//  2. Snippet rewriting at snapshot-write time: after the engine has resolved aliases, the saved
//     snippet's References map should be updated to use the new URN (similar in spirit to
//     NormalizeURNReferences for resource URNs).
func TestPclSnippetReferenceFollowsAlias(t *testing.T) {
	t.Parallel()

	schemaJSON := `{
  "version": "0.0.1",
  "name": "pkgA",
  "resources": {
    "pkgA:index:res": {
      "inputProperties": {
        "message": { "type": "string" }
      },
      "requiredInputs": ["message"]
    },
    "pkgA:index:Producer": {
      "inputProperties": {
        "value": { "type": "string" }
      },
      "requiredInputs": ["value"],
      "properties": {
        "value": { "type": "string" }
      },
      "required": ["value"]
    }
  }
}`

	// registerAsB toggles which name the program uses for the producer between the two updates.
	var registerAsB atomic.Bool

	const (
		aURN = "urn:pulumi:test::test::pkgA:index:Producer::A"
		bURN = "urn:pulumi:test::test::pkgA:index:Producer::B"
	)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(_ context.Context, _ plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					return plugin.GetSchemaResponse{Schema: []byte(schemaJSON)}, nil
				},
				CreateF: func(_ context.Context, cr plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}
					id := uuid.String()
					if cr.Preview {
						id = ""
					}
					return plugin.CreateResponse{ID: resource.ID(id), Properties: cr.Properties}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldInputs.DeepEquals(req.NewInputs) {
						return plugin.DiffResult{Changes: plugin.DiffSome}, nil
					}
					return plugin.DiffResult{}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		name := "A"
		var aliases []resource.URN
		if registerAsB.Load() {
			name = "B"
			aliases = []resource.URN{aURN}
		}
		_, err := monitor.RegisterResource("pkgA:index:Producer", name, true, deploytest.ResourceOptions{
			AliasURNs: aliases,
			Inputs:    resource.PropertyMap{"value": resource.NewProperty("hello")},
		})
		return err
	})

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			SkipDisplayTests: true,
			T:                t,
			HostF:            deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	snap.Snippets = []resource.Snippet{
		{
			Name: "consumer", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			References: map[string]string{"comp": aURN},
			Code:       `message = comp.value`,
		},
	}

	// First update: program registers the component as A; snippet resolves comp.value to "hello".
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	var consumer *resource.State
	for _, r := range snap.Resources {
		if r.URN.Name() == "consumer" {
			consumer = r
			break
		}
	}
	require.NotNil(t, consumer, "consumer snippet resource should have been created on the first run")
	require.Equal(t, resource.PropertyMap{"message": resource.NewProperty("hello")}, consumer.Inputs)
	require.Equal(t, aURN, snap.Snippets[0].References["comp"], "References should still point at A before the rename")

	// Second update: program renames A → B (via alias). The snippet still has References["comp"] = aURN.
	// The broker should treat the alias as equivalent so the snippet resolves; the saved snippet should be
	// rewritten to reference B.
	registerAsB.Store(true)
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	require.NoError(t, err)

	consumer = nil
	for _, r := range snap.Resources {
		if r.URN.Name() == "consumer" {
			consumer = r
			break
		}
	}
	require.NotNil(t, consumer, "consumer should survive the producer being aliased to a new name")
	require.Equal(t, resource.PropertyMap{"message": resource.NewProperty("hello")}, consumer.Inputs)

	require.Len(t, snap.Snippets, 1)
	require.Equal(t, bURN, snap.Snippets[0].References["comp"],
		"References should be rewritten from the aliased URN to the canonical (post-rename) URN")
}

// TestPclSnippetResourceOptions checks that a snippet's `options { ... }` block flows through to
// the engine: each option is parsed, evaluated, and lands on the synthesized resource's state.
func TestPclSnippetResourceOptions(t *testing.T) {
	t.Parallel()

	loaders := pclSnippetTestProvider(pclSnippetSchemaPropA, nil, nil, nil)

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	snap.Snippets = []resource.Snippet{
		{
			Name: "test-resource", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			Code: `propA = true
options {
    protect = true
    retainOnDelete = true
    ignoreChanges = [propA]
    additionalSecretOutputs = [propA]
    replaceOnChanges = [propA]
    deleteBeforeReplace = true
    customTimeouts = {
        create = "5m"
        update = "10m"
        delete = "15m"
    }
}`,
		},
	}
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	var snippetRes *resource.State
	for _, r := range snap.Resources {
		if r.Type == tokens.Type("pkgA:index:res") {
			snippetRes = r
			break
		}
	}
	require.NotNil(t, snippetRes, "snippet-synthesized resource should exist")
	require.True(t, snippetRes.Protect, "expected protect to flow through")
	require.True(t, snippetRes.RetainOnDelete, "expected retainOnDelete to flow through")
	require.Equal(t, []string{"propA"}, snippetRes.IgnoreChanges, "expected ignoreChanges to flow through")
	require.Equal(t, []resource.PropertyKey{"propA"}, snippetRes.AdditionalSecretOutputs,
		"expected additionalSecretOutputs to flow through")
	require.Equal(t, []string{"propA"}, snippetRes.ReplaceOnChanges, "expected replaceOnChanges to flow through")
	require.NotNil(t, snippetRes.CustomTimeouts, "expected customTimeouts to flow through")
	require.Equal(t, 5.0*60, snippetRes.CustomTimeouts.Create, "create timeout")
	require.Equal(t, 10.0*60, snippetRes.CustomTimeouts.Update, "update timeout")
	require.Equal(t, 15.0*60, snippetRes.CustomTimeouts.Delete, "delete timeout")
}

// TestPclSnippetResourceOptionsResourceRefs checks the resource-typed options (dependsOn, deletedWith,
// replaceWith) — the snippet refers to another (custom) resource via the References map and uses it
// on each option; after the update the snippet resource's state should carry the referenced URN.
func TestPclSnippetResourceOptionsResourceRefs(t *testing.T) {
	t.Parallel()

	loaders := pclSnippetTestProvider(pclSnippetSchemaPropA, nil, nil, nil)

	// Register a custom resource from the main program that the snippet will reference.
	var targetURN resource.URN
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:index:res", "target", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{"propA": resource.NewProperty(true)},
		})
		if err != nil {
			return err
		}
		targetURN = resp.URN
		return nil
	})

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotEmpty(t, targetURN, "target URN should have been captured")

	snap.Snippets = []resource.Snippet{
		{
			Name: "snippet-resource", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			References: map[string]string{
				"target": string(targetURN),
			},
			Code: `propA = true
options {
    dependsOn = [target]
    deletedWith = target
    replaceWith = [target]
}`,
		},
	}

	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	var snippetRes *resource.State
	for _, r := range snap.Resources {
		if r.URN.Name() == "snippet-resource" {
			snippetRes = r
			break
		}
	}
	require.NotNil(t, snippetRes, "snippet-synthesized resource should exist")
	require.Contains(t, snippetRes.Dependencies, targetURN, "expected dependsOn to include the target URN")
	require.Equal(t, targetURN, snippetRes.DeletedWith, "expected deletedWith to point at the target")
	require.Contains(t, snippetRes.ReplaceWith, targetURN, "expected replaceWith to include the target URN")
}

// TestPclSnippetResourceOptionsAlias checks that a snippet's `options { aliases = [...] }` flows
// through to the engine: a snippet renamed between updates should refresh in place when its old
// URN is listed as an alias rather than triggering a delete+create.
func TestPclSnippetResourceOptionsAlias(t *testing.T) {
	t.Parallel()

	var created, deleted []resource.URN
	loaders := pclSnippetTestProvider(pclSnippetSchemaPropA, &created, nil, &deleted)

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// First update creates a resource at "old-name".
	snap.Snippets = []resource.Snippet{
		{
			Name: "old-name", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			Code:       `propA = true`,
		},
	}
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.Len(t, created, 1)
	require.Empty(t, deleted)

	oldURN := created[0]

	// Second update renames to "new-name" but declares the old URN as an alias — no delete should occur.
	snap.Snippets = []resource.Snippet{
		{
			Name: "new-name", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			Code: fmt.Sprintf(`propA = true
options {
    aliases = [%q]
}`, string(oldURN)),
		},
	}
	_, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	require.NoError(t, err)
	require.Len(t, created, 1, "alias should have prevented a second create")
	require.Empty(t, deleted, "alias should have prevented a delete")
}

// TestPclSnippetResourceOptionsRange exercises `options { range = N }` on a snippet — driving the
// snippet through the PCL interpreter (rather than a hand-rolled RegisterResource) means count/foreach
// expansion is now supported "for free". A snippet with range = 3 should fan out to three resources
// named "<snippet>-0", "<snippet>-1", "<snippet>-2".
func TestPclSnippetResourceOptionsRange(t *testing.T) {
	t.Parallel()

	var created []resource.URN
	loaders := pclSnippetTestProvider(pclSnippetSchemaPropA, &created, nil, nil)

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	snap.Snippets = []resource.Snippet{
		{
			Name: "fan", Type: "pkgA:index:res",
			Descriptor: resource.PackageDescriptor{Name: "pkgA"},
			Code: `propA = true
options {
    range = 3
}`,
		},
	}
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	// Three custom resources, named fan-0/-1/-2, should now exist in the snapshot.
	wantNames := map[string]bool{"fan-0": false, "fan-1": false, "fan-2": false}
	for _, r := range snap.Resources {
		if r.Type != tokens.Type("pkgA:index:res") {
			continue
		}
		name := r.URN.Name()
		_, expected := wantNames[name]
		require.True(t, expected, "unexpected snippet-synthesized resource %q", name)
		wantNames[name] = true
	}
	for name, found := range wantNames {
		require.True(t, found, "expected fan-out resource %q in snapshot", name)
	}
	require.Len(t, created, 3, "range = 3 should have produced three Create calls")
}
