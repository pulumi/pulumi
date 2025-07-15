// Copyright 2016-2024, Pulumi Corporation.
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
	"regexp"
	"strings"
	"testing"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/display"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestPlannedUpdate(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	var ins resource.PropertyMap
	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		if expectError {
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		} else {
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Generate a plan.
	computed := interface{}(resource.Computed{Element: resource.NewStringProperty("")})
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"baz": map[string]interface{}{
			"a": 42,
			"b": computed,
		},
		"qux": []interface{}{
			computed,
			24,
		},
		"zed": computed,
	})
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NoError(t, err)

	// Attempt to run an update using the plan.
	expectError = true
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"qux": []interface{}{
			"alpha",
			24,
		},
	})
	p.Options.Plan = plan.Clone()
	validate := ExpectDiagMessage(t, regexp.QuoteMeta(
		"<{%reset%}>resource urn:pulumi:test::test::pkgA:m:typA::resA violates plan: "+
			"properties changed: +-baz[{map[a:{42} b:output<string>{}]}], +-foo[{bar}]<{%reset%}>\n"))
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate, "0")
	assert.NoError(t, err)

	// Check the resource's state.
	if !assert.Len(t, snap.Resources, 1) {
		return
	}

	// Change the provider's planned operation to a same step.
	// Remove the provider from the plan.
	plan.ResourcePlans["urn:pulumi:test::test::pulumi:providers:pkgA::default"].Ops = []display.StepOp{deploy.OpSame}

	// Attempt to run an update using the plan.
	expectError = false
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"baz": map[string]interface{}{
			"a": 42,
			"b": "alpha",
		},
		"qux": []interface{}{
			"beta",
			24,
		},
		"zed": "grr",
	})
	p.Options.Plan = plan.Clone()
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)

	// Check the resource's state.
	if !assert.Len(t, snap.Resources, 2) {
		return
	}

	expected := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"baz": map[string]interface{}{
			"a": 42,
			"b": "alpha",
		},
		"qux": []interface{}{
			"beta",
			24,
		},
		"zed": "grr",
	})
	assert.Equal(t, expected, snap.Resources[1].Outputs)
}

func TestUnplannedCreate(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	createResource := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if createResource {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs: ins,
			})
			assert.ErrorContains(t, err, "context canceled")
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Create a plan to do nothing
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NoError(t, err)

	// Now set the flag for the language runtime to create a resource, and run update with the plan
	createResource = true
	p.Options.Plan = plan.Clone()
	validate := ExpectDiagMessage(t, regexp.QuoteMeta(
		"<{%reset%}>create is not allowed by the plan: no steps were expected for this resource<{%reset%}>\n"))
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)
	assert.NoError(t, err)

	// Check nothing was was created
	assert.NotNil(t, snap)
	if !assert.Len(t, snap.Resources, 0) {
		return
	}
}

func TestUnplannedDelete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(context.Context, plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	createAllResources := true
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)

		if createAllResources {
			_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs: ins,
			})
			assert.NoError(t, err)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Create an initial snapshot that resA and resB exist
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	// Create a plan that resA and resB won't change
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, snap), p.Options, p.BackendClient, nil)
	assert.NoError(t, err)

	// Now set the flag for the language runtime to not create resB and run an update with
	// the no-op plan, this should block the delete
	createAllResources = false
	p.Options.Plan = plan.Clone()
	validate := ExpectDiagMessage(t, regexp.QuoteMeta(
		"<{%reset%}>delete is not allowed by the plan: this resource is constrained to same<{%reset%}>\n"))
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate, "1")
	assert.NotNil(t, snap)
	assert.NoError(t, err)

	// Check both resources and the provider are still listed in the snapshot
	if !assert.Len(t, snap.Resources, 3) {
		return
	}
}

func TestExpectedDelete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(context.Context, plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	createAllResources := true
	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)

		if createAllResources {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs: ins,
			})
			if expectError {
				assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
			} else {
				assert.NoError(t, err)
			}
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Create an initial snapshot that resA and resB exist
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NotNil(t, snap)
	assert.NoError(t, err)

	// Create a plan that resA is same and resB is deleted
	createAllResources = false
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, snap), p.Options, p.BackendClient, nil)
	assert.NotNil(t, plan)
	assert.NoError(t, err)

	// Now run but set the runtime to return resA and resB, given we expected resB to be deleted
	// this should be an error
	createAllResources = true
	expectError = true
	p.Options.Plan = plan.Clone()
	validate := ExpectDiagMessage(t, regexp.QuoteMeta(
		"<{%reset%}>resource urn:pulumi:test::test::pkgA:m:typA::resB violates plan: "+
			"resource unexpectedly not deleted<{%reset%}>\n"))
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate, "1")
	assert.NotNil(t, snap)
	assert.NoError(t, err)

	// Check both resources and the provider are still listed in the snapshot
	if !assert.Len(t, snap.Resources, 3) {
		return
	}
}

func TestExpectedCreate(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	createAllResources := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)

		if createAllResources {
			_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs: ins,
			})
			assert.NoError(t, err)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Create an initial snapshot that resA exists
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NotNil(t, snap)
	assert.NoError(t, err)

	// Create a plan that resA is same and resB is created
	createAllResources = true
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, snap), p.Options, p.BackendClient, nil)
	assert.NotNil(t, plan)
	assert.NoError(t, err)

	// Now run but set the runtime to return resA, given we expected resB to be created
	// this should be an error
	createAllResources = false
	p.Options.Plan = plan.Clone()
	validate := ExpectDiagMessage(t, regexp.QuoteMeta(
		"<{%reset%}>expected resource operations for "+
			"urn:pulumi:test::test::pkgA:m:typA::resB but none were seen<{%reset%}>\n"))
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate, "1")
	assert.NotNil(t, snap)
	assert.NoError(t, err)

	// Check resA and the provider are still listed in the snapshot
	if !assert.Len(t, snap.Resources, 2) {
		return
	}
}

func TestPropertySetChange(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo":  "bar",
		"frob": "baz",
	})
	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		if expectError {
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		} else {
			assert.NoError(t, err)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Create an initial plan to create resA
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NotNil(t, plan)
	assert.NoError(t, err)

	// Now change the runtime to not return property "frob", this should error
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	expectError = true
	p.Options.Plan = plan.Clone()
	validate := ExpectDiagMessage(t, regexp.QuoteMeta(
		"<{%reset%}>resource urn:pulumi:test::test::pkgA:m:typA::resA violates plan: "+
			"properties changed: +-frob[{baz}]<{%reset%}>\n"))
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)
	assert.NotNil(t, snap)
	assert.NoError(t, err)
}

func TestExpectedUnneededCreate(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Create a plan that resA needs creating
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NotNil(t, plan)
	assert.NoError(t, err)

	// Create an a snapshot that resA exists
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NotNil(t, snap)
	assert.NoError(t, err)

	// Now run again with the plan set but the snapshot that resA already exists
	p.Options.Plan = plan.Clone()
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NotNil(t, snap)
	assert.NoError(t, err)

	// Check resA and the provider are still listed in the snapshot
	if !assert.Len(t, snap.Resources, 2) {
		return
	}
}

func TestExpectedUnneededDelete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(context.Context, plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	createResource := true
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if createResource {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs: ins,
			})
			assert.NoError(t, err)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Create an initial snapshot that resA exists
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	// Create a plan that resA is deleted
	createResource = false
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, snap), p.Options, p.BackendClient, nil)
	assert.NoError(t, err)

	// Now run to delete resA
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NotNil(t, snap)
	assert.NoError(t, err)

	// Now run again with the plan set but the snapshot that resA is already deleted
	p.Options.Plan = plan.Clone()
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	assert.NotNil(t, snap)
	assert.NoError(t, err)

	// Check the resources are still gone
	if !assert.Len(t, snap.Resources, 0) {
		return
	}
}

func TestResoucesWithSames(t *testing.T) {
	t.Parallel()

	// This test checks that if between generating a constraint and running the update that if new resources have been
	// added to the stack that the update doesn't change those resources in any way that they don't cause constraint
	// errors.

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	var ins resource.PropertyMap
	createA := false
	createB := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if createA {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs: ins,
			})
			assert.NoError(t, err)
		}

		if createB {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
					"X": "Y",
				}),
			})
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Generate a plan to create A
	createA = true
	createB = false
	computed := interface{}(resource.Computed{Element: resource.NewStringProperty("")})
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"zed": computed,
	})
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NoError(t, err)

	// Run an update that creates B
	createA = false
	createB = true
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	// Check the resource's state.
	if !assert.Len(t, snap.Resources, 2) {
		return
	}

	expected := resource.NewPropertyMapFromMap(map[string]interface{}{
		"X": "Y",
	})
	assert.Equal(t, expected, snap.Resources[1].Outputs)

	// Attempt to run an update with the plan on the stack that creates A and sames B
	createA = true
	createB = true
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"zed": 24,
	})
	p.Options.Plan = plan.Clone()
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)

	// Check the resource's state.
	if !assert.Len(t, snap.Resources, 3) {
		return
	}

	expected = resource.NewPropertyMapFromMap(map[string]interface{}{
		"X": "Y",
	})
	assert.Equal(t, expected, snap.Resources[2].Outputs)

	expected = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"zed": 24,
	})
	assert.Equal(t, expected, snap.Resources[1].Outputs)
}

func TestPlannedPreviews(t *testing.T) {
	t.Parallel()

	// This checks that plans work in previews, this is very similar to TestPlannedUpdate except we only do previews

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	var ins resource.PropertyMap
	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		if expectError {
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		} else {
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Generate a plan.
	computed := interface{}(resource.Computed{Element: resource.NewStringProperty("")})
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"baz": map[string]interface{}{
			"a": 42,
			"b": computed,
		},
		"qux": []interface{}{
			computed,
			24,
		},
		"zed": computed,
	})
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NoError(t, err)

	// Attempt to run a new preview using the plan, given we've changed the property set this should fail
	expectError = true
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"qux": []interface{}{
			"alpha",
			24,
		},
	})
	p.Options.Plan = plan.Clone()
	validate := ExpectDiagMessage(t, regexp.QuoteMeta(
		"<{%reset%}>resource urn:pulumi:test::test::pkgA:m:typA::resA violates plan: properties changed: "+
			"+-baz[{map[a:{42} b:output<string>{}]}], +-foo[{bar}]<{%reset%}>\n"))
	_, err = lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, validate)
	assert.NoError(t, err)

	// Attempt to run an preview using the plan, such that the property set is now valid
	expectError = false
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"baz": map[string]interface{}{
			"a": 42,
			"b": computed,
		},
		"qux": []interface{}{
			"beta",
			24,
		},
		"zed": "grr",
	})
	p.Options.Plan = plan.Clone()
	_, err = lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NoError(t, err)
}

func TestPlannedUpdateChangedStack(t *testing.T) {
	t.Parallel()

	// This tests the case that we run a planned update against a stack that has changed between preview and update

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	var ins resource.PropertyMap
	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		if expectError {
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		} else {
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Set initial data for foo and zed
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"zed": 24,
	})
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	// Generate a plan that we want to change foo
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
		"zed": 24,
	})
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, snap), p.Options, p.BackendClient, nil)
	assert.NoError(t, err)

	// Change zed in the stack
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"zed": 26,
	})
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)

	// Attempt to run an update using the plan but where we haven't updated our program for the change of zed
	expectError = true
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
		"zed": 24,
	})
	p.Options.Plan = plan.Clone()
	validate := ExpectDiagMessage(t, regexp.QuoteMeta(
		"<{%reset%}>resource urn:pulumi:test::test::pkgA:m:typA::resA violates plan: "+
			"properties changed: =~zed[{24}]<{%reset%}>\n"))
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate, "2")
	assert.NoError(t, err)

	// Check the resource's state we shouldn't of changed anything because the update failed
	if !assert.Len(t, snap.Resources, 2) {
		return
	}

	expected := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"zed": 26,
	})
	assert.Equal(t, expected, snap.Resources[1].Outputs)
}

func TestPlannedOutputChanges(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	outs := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo":  "bar",
		"frob": "baz",
	})
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		_ = monitor.RegisterResourceOutputs(resp.URN, outs)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Create an initial plan to create resA and the outputs
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NotNil(t, plan)
	assert.NoError(t, err)

	// Now change the runtime to not return property "frob", this should error
	outs = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	p.Options.Plan = plan.Clone()
	validate := ExpectDiagMessage(t, regexp.QuoteMeta(
		"<{%reset%}>resource violates plan: properties changed: +-frob[{baz}]<{%reset%}>\n"))
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)
	assert.NotNil(t, snap)
	assert.NoError(t, err)
}

func TestPlannedInputOutputDifferences(t *testing.T) {
	t.Parallel()

	// This tests that plans are working on the program inputs, not the provider outputs

	createOutputs := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo":  "bar",
		"frob": "baz",
		"baz":  24,
	})
	updateOutputs := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo":  "bar",
		"frob": "newBazzer",
		"baz":  24,
	})

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id-" + req.URN.Name()),
						Properties: createOutputs,
						Status:     resource.StatusOK,
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					return plugin.UpdateResponse{
						Properties: updateOutputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	inputs := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo":  "bar",
		"frob": "baz",
	})
	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		if expectError {
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		} else {
			assert.NoError(t, err)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Create an initial plan to create resA
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NotNil(t, plan)
	assert.NoError(t, err)

	// Check we can create resA even though its outputs are different to the planned inputs
	p.Options.Plan = plan.Clone()
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NotNil(t, snap)
	assert.NoError(t, err)

	// Make a plan to change resA
	inputs = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo":  "bar",
		"frob": "newBazzer",
	})
	p.Options.Plan = nil
	plan, err = lt.TestOp(Update).Plan(project, p.GetTarget(t, snap), p.Options, p.BackendClient, nil)
	assert.NotNil(t, plan)
	assert.NoError(t, err)

	// Test the plan fails if we don't pass newBazzer
	expectError = true
	inputs = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo":  "bar",
		"frob": "differentBazzer",
	})
	p.Options.Plan = plan.Clone()
	validate := ExpectDiagMessage(t, regexp.QuoteMeta(
		"<{%reset%}>resource urn:pulumi:test::test::pkgA:m:typA::resA violates plan: "+
			"properties changed: ~~frob[{newBazzer}!={differentBazzer}]<{%reset%}>\n"))
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate, "1")
	assert.NotNil(t, snap)
	assert.NoError(t, err)

	// Check the plan succeeds if we do pass newBazzer
	expectError = false
	inputs = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo":  "bar",
		"frob": "newBazzer",
	})
	p.Options.Plan = plan.Clone()
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	assert.NotNil(t, snap)
	assert.NoError(t, err)
}

func TestAliasWithPlans(t *testing.T) {
	t.Parallel()

	// This tests that if a resource has an alias the plan for it is still used

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	resourceName := "resA"
	var aliases []resource.URN
	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo":  "bar",
		"frob": "baz",
	})
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", resourceName, true, deploytest.ResourceOptions{
			Inputs:    ins,
			AliasURNs: aliases,
		})
		assert.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Create an initial ResA
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NotNil(t, snap)
	assert.NoError(t, err)

	// Update the name and alias and make a plan for resA
	resourceName = "newResA"
	aliases = make([]resource.URN, 1)
	aliases[0] = resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA")
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NotNil(t, plan)
	assert.NoError(t, err)

	// Now try and run with the plan
	p.Options.Plan = plan.Clone()
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NotNil(t, snap)
	assert.NoError(t, err)
}

func TestComputedCanBeDropped(t *testing.T) {
	t.Parallel()

	// This tests that values that show as <computed> in the plan can be dropped in the update (because they may of
	// resolved to undefined). We're testing both RegisterResource and RegisterResourceOutputs here.

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	var resourceInputs resource.PropertyMap
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs: resourceInputs,
		})
		assert.NoError(t, err)

		// We're using the same property set on purpose, this is not a test bug
		err = monitor.RegisterResourceOutputs(resp.URN, resourceInputs)
		assert.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// The three property sets we'll use in this test
	computed := interface{}(resource.Computed{Element: resource.NewStringProperty("")})
	computedPropertySet := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"baz": map[string]interface{}{
			"a": 42,
			"b": computed,
		},
		"qux": []interface{}{
			computed,
			24,
		},
		"zed": computed,
	})
	fullPropertySet := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"baz": map[string]interface{}{
			"a": 42,
			"b": "alpha",
		},
		"qux": []interface{}{
			"beta",
			24,
		},
		"zed": "grr",
	})
	partialPropertySet := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"baz": map[string]interface{}{
			"a": 42,
		},
		"qux": []interface{}{
			nil, // computed values that resolve to undef don't get dropped from arrays, they just become null
			24,
		},
	})

	// Generate a plan.
	resourceInputs = computedPropertySet
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NoError(t, err)

	// Attempt to run an update using the plan with all computed values removed
	resourceInputs = partialPropertySet
	p.Options.Plan = plan.Clone()
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	// Check the resource's state.
	if !assert.Len(t, snap.Resources, 3) {
		return
	}

	assert.Equal(t, partialPropertySet, snap.Resources[1].Outputs)
	assert.Equal(t, partialPropertySet, snap.Resources[2].Outputs)

	// Now run an update to set the values of the computed properties...
	resourceInputs = fullPropertySet
	p.Options.Plan = nil
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)

	// Check the resource's state.
	if !assert.Len(t, snap.Resources, 3) {
		return
	}

	assert.Equal(t, fullPropertySet, snap.Resources[1].Outputs)
	assert.Equal(t, fullPropertySet, snap.Resources[2].Outputs)

	// ...and then build a new plan where they're computed updates (vs above where its computed creates)
	resourceInputs = computedPropertySet
	plan, err = lt.TestOp(Update).Plan(project, p.GetTarget(t, snap), p.Options, p.BackendClient, nil)
	assert.NoError(t, err)

	// Now run the an update with the plan and check the update is allowed to remove these properties
	resourceInputs = partialPropertySet
	p.Options.Plan = plan.Clone()
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	assert.NoError(t, err)

	// Check the resource's state.
	if !assert.Len(t, snap.Resources, 3) {
		return
	}

	assert.Equal(t, partialPropertySet, snap.Resources[1].Outputs)
	assert.Equal(t, partialPropertySet, snap.Resources[2].Outputs)
}

func TestPlannedUpdateWithNondeterministicCheck(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				CheckF: func(
					_ context.Context,
					req plugin.CheckRequest,
				) (plugin.CheckResponse, error) {
					// If we have name use it, else use olds name, else make one up
					if _, has := req.News["name"]; has {
						return plugin.CheckResponse{Properties: req.News}, nil
					}
					if _, has := req.Olds["name"]; has {
						result := req.News.Copy()
						result["name"] = req.Olds["name"]
						return plugin.CheckResponse{Properties: result}, nil
					}

					name, err := resource.NewUniqueHex(req.URN.Name(), 8, 512)
					assert.NoError(t, err)

					result := req.News.Copy()
					result["name"] = resource.NewStringProperty(name)
					return plugin.CheckResponse{Properties: result}, nil
				},
			}, nil
		}),
	}

	var ins resource.PropertyMap
	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})

		if err == nil {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
					"other": resp.Outputs["name"].StringValue(),
				}),
			})
			if expectError {
				assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
			} else {
				assert.NoError(t, err)
			}
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Generate a plan.
	computed := interface{}(resource.Computed{Element: resource.NewStringProperty("")})
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"zed": computed,
	})
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NoError(t, err)

	// Attempt to run an update using the plan.
	// This should fail because of the non-determinism
	expectError = true
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"zed": "baz",
	})
	p.Options.Plan = plan.Clone()

	validate := ExpectDiagMessage(t,
		"<{%reset%}>resource urn:pulumi:test::test::pkgA:m:typA::resA violates plan: "+
			"properties changed: \\+\\+name\\[{res[\\d\\w]{9}}!={res[\\d\\w]{9}}\\]<{%reset%}>\\n")
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate, "4")
	assert.NoError(t, err)

	// Check the resource's state.
	if !assert.Len(t, snap.Resources, 1) {
		return
	}
}

func TestPlannedUpdateWithCheckFailure(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi/issues/9247

	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				CheckF: func(
					_ context.Context,
					req plugin.CheckRequest,
				) (plugin.CheckResponse, error) {
					if req.News["foo"].StringValue() == "bad" {
						return plugin.CheckResponse{
							Failures: []plugin.CheckFailure{{
								Property: resource.PropertyKey("foo"),
								Reason:   "Bad foo",
							}},
						}, nil
					}
					return plugin.CheckResponse{Properties: req.News}, nil
				},
			}, nil
		}),
	}

	var ins resource.PropertyMap
	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		if expectError {
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		} else {
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Generate a plan with bad inputs
	expectError = true
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bad",
	})
	validate := ExpectDiagMessage(t, regexp.QuoteMeta(
		"<{%reset%}>pkgA:m:typA resource 'resA': property foo value {bad} has a problem: Bad foo<{%reset%}>\n"))
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, validate)
	assert.Nil(t, plan)
	assert.NoError(t, err)

	// Generate a plan with good inputs
	expectError = false
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "good",
	})
	plan, err = lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NotNil(t, plan)
	assert.Contains(t, plan.ResourcePlans, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
	assert.NoError(t, err)

	// Try and run against the plan with inputs that will fail Check
	expectError = true
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bad",
	})
	p.Options.Plan = plan.Clone()
	validate = ExpectDiagMessage(t, regexp.QuoteMeta(
		"<{%reset%}>pkgA:m:typA resource 'resA': property foo value {bad} has a problem: Bad foo<{%reset%}>\n"))
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)
	assert.NoError(t, err)
	assert.NotNil(t, snap)

	// Check the resource's state.
	if !assert.Len(t, snap.Resources, 1) {
		return
	}
}

func TestPluginsAreDownloaded(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	semver10 := semver.MustParse("1.0.0")

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)
		return nil
	},
		workspace.PackageDescriptor{PluginSpec: workspace.PluginSpec{Name: "pkgA"}},
		workspace.PackageDescriptor{PluginSpec: workspace.PluginSpec{Name: "pkgB", Version: &semver10}})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NotNil(t, plan)
	assert.Contains(t, plan.ResourcePlans, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
	assert.NoError(t, err)
}

func TestProviderDeterministicPreview(t *testing.T) {
	t.Parallel()

	var generatedName resource.PropertyValue

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(
					_ context.Context,
					req plugin.CheckRequest,
				) (plugin.CheckResponse, error) {
					// make a deterministic autoname
					if _, has := req.News["name"]; !has {
						if name, has := req.Olds["name"]; has {
							req.News["name"] = name
						} else {
							name, err := resource.NewUniqueName(req.RandomSeed, req.URN.Name(), -1, -1, nil)
							assert.NoError(t, err)
							generatedName = resource.NewStringProperty(name)
							req.News["name"] = generatedName
						}
					}

					return plugin.CheckResponse{Properties: req.News}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						// If foo changes do a replace, we use this to check we get a new name
						return plugin.DiffResult{
							Changes:     plugin.DiffSome,
							ReplaceKeys: []resource.PropertyKey{"foo"},
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{GeneratePlan: true, Experimental: true},
		},
	}

	project := p.GetProject()

	// Run a preview, this should want to create resA with a given name
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), p.Options, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.True(t, generatedName.IsString())
	assert.NotEqual(t, "", generatedName.StringValue())
	expectedName := generatedName

	// Run an update, we should get the same name as we saw in preview
	p.Options.Plan = plan
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, expectedName, snap.Resources[1].Inputs["name"])
	assert.Equal(t, expectedName, snap.Resources[1].Outputs["name"])

	// Run a new update which will cause a replace and check we get a new name
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	p.Options.Plan = nil
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.NotEqual(t, expectedName, snap.Resources[1].Inputs["name"])
	assert.NotEqual(t, expectedName, snap.Resources[1].Outputs["name"])
}

func TestPlannedUpdateWithDependentDelete(t *testing.T) {
	t.Parallel()

	var diffResult *plugin.DiffResult

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				CheckF: func(
					_ context.Context,
					req plugin.CheckRequest,
				) (plugin.CheckResponse, error) {
					return plugin.CheckResponse{Properties: req.News}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if strings.Contains(string(req.URN), "resA") || strings.Contains(string(req.URN), "resB") {
						assert.NotNil(t, diffResult, "Diff was called but diffResult wasn't set")
						return *diffResult, nil
					}
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	var ins resource.PropertyMap
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typB", "resB", true, deploytest.ResourceOptions{
			Inputs:       respA.Outputs,
			Dependencies: []resource.URN{respA.URN},
		})
		assert.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, UpdateOptions: UpdateOptions{GeneratePlan: true}},
	}

	project := p.GetProject()

	// Create an initial ResA and resB
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
		"zed": "baz",
	})
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NotNil(t, snap)
	assert.NoError(t, err)

	// Update the input and mark it as a replace, check that both A and B are marked as replacements
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "frob",
		"zed": "baz",
	})
	diffResult = &plugin.DiffResult{
		Changes:     plugin.DiffSome,
		ReplaceKeys: []resource.PropertyKey{"foo"},
		StableKeys:  []resource.PropertyKey{"zed"},
		DetailedDiff: map[string]plugin.PropertyDiff{
			"foo": {
				Kind:      plugin.DiffUpdateReplace,
				InputDiff: true,
			},
		},
		DeleteBeforeReplace: true,
	}
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, snap), p.Options, p.BackendClient, nil)
	assert.NotNil(t, plan)
	assert.NoError(t, err)

	assert.Equal(t, 3, len(plan.ResourcePlans["urn:pulumi:test::test::pkgA:m:typA::resA"].Ops))
	assert.Equal(t, 3, len(plan.ResourcePlans["urn:pulumi:test::test::pkgA:m:typB::resB"].Ops))

	// Now try and run with the plan
	p.Options.Plan = plan.Clone()
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NotNil(t, snap)
	assert.NoError(t, err)
}

// TestResourcesTargeted checks that a plan created with targets specified captures only those targets and
// default providers. It checks that trying to construct a new resource that was not targeted in the plan
// fails and that the update with the same --targets specified is compatible with the plan (roundtripped).
func TestResourcesTargeted(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewStringProperty("bar"),
			},
		})
		if expectError {
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		} else {
			assert.NoError(t, err)
		}

		_, _ = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewStringProperty("bar"),
			},
		})

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{}

	project := p.GetProject()

	// Create the update plan with only targeted resources.
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Experimental: true,
			GeneratePlan: true,
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::resB",
			}),
		},
	}, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	// Check that running an update with everything targeted fails due to our plan being constrained
	// to the resource.
	expectError = true
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			// Clone the plan as the plan will be mutated by the engine and useless in future runs.
			Plan:         plan.Clone(),
			Experimental: true,
		},
	}, false, p.BackendClient, nil, "0")
	assert.Error(t, err)

	// Check that running an update with the same Targets as the Plan succeeds.
	expectError = false
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			// Clone the plan as the plan will be mutated by the engine and useless in future runs.
			Plan:         plan.Clone(),
			Experimental: true,
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::resB",
			}),
		},
	}, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)
}

// This test checks that registering resource outputs does not fail for the root stack resource when it
// is not specified by the update targets.
func TestStackOutputsWithTargetedPlan(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test-test", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true)

		assert.NoError(t, err)

		err = monitor.RegisterResourceOutputs(resp.URN, resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		})

		assert.NoError(t, err)

		return nil
	})

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	project := p.GetProject()

	// Create the update plan without targeting the root stack.
	plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: p.Options.HostF,
		UpdateOptions: UpdateOptions{
			Experimental: true,
			GeneratePlan: true,
			Targets: deploy.NewUrnTargetsFromUrns([]resource.URN{
				resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"),
			}),
		},
	}, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	// Check that update succeeds despite the root stack not being targeted.
	_, err = lt.TestOp(Update).Run(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: p.Options.HostF,
		UpdateOptions: UpdateOptions{
			GeneratePlan: true,
			Experimental: true,
			Targets: deploy.NewUrnTargetsFromUrns([]resource.URN{
				resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"),
			}),
		},
	}, false, p.BackendClient, nil)
	assert.NoError(t, err)
}
