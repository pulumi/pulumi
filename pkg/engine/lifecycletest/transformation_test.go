// Copyright 2016-2022, Pulumi Corporation.
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
	"testing"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TransformationFunction(f func(name, typ string, props resource.PropertyMap) (resource.PropertyMap, error),
) func([]resource.PropertyValue) ([]resource.PropertyValue, error) {
	return func(args []resource.PropertyValue) ([]resource.PropertyValue, error) {
		name := args[0].StringValue()
		typ := args[1].StringValue()
		props := args[2].ObjectValue()
		ret, err := f(name, typ, props)
		return []resource.PropertyValue{
			resource.NewObjectProperty(ret),
		}, err
	}
}

// Test that the engine invokes all transformation functions in the correct order.
func TestRemoteTransformations(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		callback1, err := callbacks.Allocate(
			TransformationFunction(func(name, typ string, props resource.PropertyMap) (resource.PropertyMap, error) {
				props["foo"] = resource.NewNumberProperty(props["foo"].NumberValue() + 1)
				// callback 2 should run before this one so "bar" should exist at this point
				props["bar"] = resource.NewStringProperty(props["bar"].StringValue() + "baz")

				return props, nil
			}))
		require.NoError(t, err)

		callback2, err := callbacks.Allocate(
			TransformationFunction(func(name, typ string, props resource.PropertyMap) (resource.PropertyMap, error) {
				props["foo"] = resource.NewNumberProperty(props["foo"].NumberValue() + 1)
				props["bar"] = resource.NewStringProperty("bar")
				// if this is for resB then callback 3 will have run before this one
				if prop, has := props["frob"]; has {
					props["frob"] = resource.MakeSecret(prop)
				} else {
					props["frob"] = resource.NewStringProperty("nofrob")
				}

				return props, nil
			}))
		require.NoError(t, err)

		callback3, err := callbacks.Allocate(
			TransformationFunction(func(name, typ string, props resource.PropertyMap) (resource.PropertyMap, error) {
				props["foo"] = resource.NewNumberProperty(props["foo"].NumberValue() + 1)
				props["frob"] = resource.NewStringProperty("frob")
				return props, nil
			}))
		require.NoError(t, err)

		err = monitor.RegisterStackTransformation(callback1)
		require.NoError(t, err)

		aURN, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewNumberProperty(1),
			},
			Transformations: []*pulumirpc.Callback{
				callback2,
			},
		})
		require.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewNumberProperty(10),
			},
			Transformations: []*pulumirpc.Callback{
				callback3,
			},
			Parent: aURN,
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)

	assert.Len(t, snap.Resources, 3)
	// Check Resources[1] is the resA resource
	res := snap.Resources[1]
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), res.URN)
	// Check it's final input properties match what we expected from the transformations
	assert.Equal(t, resource.PropertyMap{
		"foo":  resource.NewNumberProperty(3),
		"bar":  resource.NewStringProperty("barbaz"),
		"frob": resource.NewStringProperty("nofrob"),
	}, res.Inputs)

	// Check Resources[2] is the resB resource
	res = snap.Resources[2]
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resB"), res.URN)
	// Check it's final input properties match what we expected from the transformations
	assert.Equal(t, resource.PropertyMap{
		"foo":  resource.NewNumberProperty(13),
		"bar":  resource.NewStringProperty("barbaz"),
		"frob": resource.MakeSecret(resource.NewStringProperty("frob")),
	}, res.Inputs)
}
