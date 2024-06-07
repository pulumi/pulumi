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
	"fmt"
	"os"
	"testing"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// TestSubstack asserts that the engine can correctly start up multiple language plugins to execute a second program as
// a component.
//
//nolint:paralleltest // Test depends on the filesystem
func TestSubstack(t *testing.T) {
	tmp := t.TempDir()

	// Switch to a temporary directory to mock filesystem paths
	cwd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		err = os.Chdir(cwd)
		require.NoError(t, err)
	})
	err = os.Chdir(tmp)
	require.NoError(t, err)

	err = os.Mkdir("source", 0o700)
	require.NoError(t, err)

	err = os.WriteFile("source/Pulumi.yaml", []byte("name: substack\nruntime: subtest\n"), 0o600)
	require.NoError(t, err)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	runtimeF := func(rt string, info plugin.ProgramInfo) (plugin.LanguageRuntime, error) {
		if rt == "test" {
			return deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource(resource.RootStackType, "test", false)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pulumi:pulumi:Stack", "substack", false, deploytest.ResourceOptions{
					Inputs: resource.PropertyMap{
						"source": resource.NewStringProperty("source"),
					},
					Remote: true,
				})
				assert.NoError(t, err)

				return nil
			}), nil
		} else if rt == "subtest" {
			return deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pkgA:index:MyResource", "test", true)
				assert.NoError(t, err)
				return nil
			}), nil
		}
		return nil, fmt.Errorf("unsupported runtime %s", rt)
	}

	hostF := deploytest.NewPluginHostF(nil, nil, runtimeF, loaders...)
	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	// Run the update to create the substack and its inner resource
	snap, err := TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)

	assert.Len(t, snap.Resources, 5)
	// We should have the root stack, the component stack, and the custom resource nested inside that component stack.
	assert.Equal(t, tokens.Type("pulumi:pulumi:Stack"), snap.Resources[0].URN.QualifiedType())
	assert.Equal(t, "test", snap.Resources[0].URN.Name())
	assert.Equal(t, tokens.Type("pulumi:pulumi:Stack"), snap.Resources[2].URN.QualifiedType())
	assert.Equal(t, "substack", snap.Resources[2].URN.Name())
	// N.B. The presence of $test$ in the type URN. Not really a type, but ensures URNs are unique.
	assert.Equal(t, tokens.Type("pulumi:pulumi:Stack$test$pkgA:index:MyResource"), snap.Resources[4].URN.QualifiedType())
	assert.Equal(t, "test", snap.Resources[4].URN.Name())
}

// TestSubstackNameValidation asserts that the engine validates substacks only have legal names. Substack names have to
// fit into the qualified type part of the URN and so we don't want them to contain :: or $ characters.
func TestSubstackNameValidation(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	runtimeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource(resource.RootStackType, "test", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pulumi:pulumi:Stack", "a bad \n stack", false, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"source": resource.NewStringProperty("source"),
			},
			Remote: true,
		})
		assert.ErrorContains(t, err,
			"a stack name may only contain alphanumeric, hyphens, underscores, or periods:"+
				" invalid character ' ' at position 1")

		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, runtimeF, loaders...)
	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	// Run the update to create the substack and its inner resource
	_, err := TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.ErrorContains(t, err, "BAIL")
}
