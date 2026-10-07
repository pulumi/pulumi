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
	"sync"
	"sync/atomic"
	"testing"

	"github.com/blang/semver"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers/sandbox"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func localTarget(t *testing.T, p *lt.TestPlan, snap *deploy.Snapshot) deploy.Target {
	target := p.GetTarget(t, snap)
	target.Tags = map[string]string{sandbox.StackTag: "true"}
	return target
}

func newSandbox(t *testing.T) *sandbox.Mode {
	mode, err := sandbox.New(sandbox.Options{})
	require.NoError(t, err)
	return mode
}

// singleProviderProgram registers an explicit provider for pkg with the given config and one resource that uses it.
func singleProviderProgram(
	t *testing.T, pkg tokens.Package, config resource.PropertyMap,
) deploytest.LanguageRuntimeFactory {
	return deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource(providers.MakeProviderType(pkg), "prov", true, deploytest.ResourceOptions{
			Inputs: config,
		})
		if err != nil {
			return err
		}
		ref, err := providers.NewReference(resp.URN, resp.ID)
		require.NoError(t, err)
		_, err = monitor.RegisterResource(tokens.Type(string(pkg)+":index:Res"), "res", true, deploytest.ResourceOptions{
			Provider: ref.String(),
		})
		return err
	})
}

// Sandbox mode rewrites a provider's configuration when it is configured, but the provider's inputs in state stay
// exactly as the program declared them.
func TestSandboxRewritesConfigOnlyAtConfigure(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var configured []property.Map
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("aws", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConfigureF: func(_ context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
					mu.Lock()
					defer mu.Unlock()
					configured = append(configured, req.Inputs)
					return plugin.ConfigureResponse{}, nil
				},
			}, nil
		}),
	}

	programF := singleProviderProgram(t, "aws", resource.PropertyMap{
		"region":  resource.NewProperty("eu-west-1"),
		"profile": resource.NewProperty("production"),
	})
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			HostF:            deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...),
			SkipDisplayTests: true,
		},
	}
	p.Options.Sandbox = newSandbox(t)

	snap, err := lt.TestOp(Update).Run(p.GetProject(), localTarget(t, p, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)

	require.NotEmpty(t, configured)
	for _, inputs := range configured {
		assert.Equal(t, "eu-west-1", inputs.Get("region").AsString(), "an explicit region is kept")
		assert.Equal(t, "test", inputs.Get("accessKey").AsString())
		assert.True(t, inputs.Get("skipCredentialsValidation").AsBool())
		_, hasProfile := inputs.GetOk("profile")
		assert.False(t, hasProfile, "the profile must not reach the provider")
	}

	var prov *pkgresource.State
	for _, r := range snap.Resources {
		if providers.IsProviderType(r.Type) {
			prov = r
		}
	}
	require.NotNil(t, prov)
	assert.Equal(t, "production", prov.Inputs["profile"].StringValue())
	assert.NotContains(t, prov.Inputs, resource.PropertyKey("accessKey"))
	assert.NotContains(t, prov.Inputs, resource.PropertyKey("skipCredentialsValidation"))
}

func TestSandboxRejectsUnsupportedPackages(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("kubernetes", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}
	programF := singleProviderProgram(t, "kubernetes", resource.PropertyMap{})
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			HostF:            deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...),
			SkipDisplayTests: true,
		},
	}
	p.Options.Sandbox = newSandbox(t)

	_, err := lt.TestOp(Update).Run(p.GetProject(), localTarget(t, p, nil), p.Options, false, p.BackendClient, nil)
	require.ErrorContains(t, err, `package "kubernetes" is not supported for sandbox stacks`)
}

func TestSandboxMustMatchStack(t *testing.T) {
	t.Parallel()

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})

	t.Run("sandbox stack without sandbox mode", func(t *testing.T) {
		t.Parallel()

		p := &lt.TestPlan{
			Options: lt.TestUpdateOptions{
				T: t, HostF: deploytest.NewPluginHostF(nil, nil, programF, nil, nil), SkipDisplayTests: true,
			},
		}
		_, err := lt.TestOp(Update).Run(p.GetProject(), localTarget(t, p, nil), p.Options, false, p.BackendClient, nil)
		require.ErrorContains(t, err, "is a sandbox stack")
	})

	t.Run("sandbox mode on a non-sandbox stack", func(t *testing.T) {
		t.Parallel()

		p := &lt.TestPlan{
			Options: lt.TestUpdateOptions{
				T: t, HostF: deploytest.NewPluginHostF(nil, nil, programF, nil, nil), SkipDisplayTests: true,
			},
		}
		p.Options.Sandbox = newSandbox(t)
		_, err := lt.TestOp(Update).Run(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
		require.ErrorContains(t, err, "is not a sandbox stack")
	})
}

// A program using both AWS and Google Cloud gets each provider redirected at its own emulator.
func TestSandboxAWSAndGCP(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	configured := map[string]property.Map{}
	loader := func(pkg string) *deploytest.ProviderLoader {
		return deploytest.NewProviderLoader(tokens.Package(pkg), semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConfigureF: func(_ context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
					mu.Lock()
					defer mu.Unlock()
					configured[pkg] = req.Inputs
					return plugin.ConfigureResponse{}, nil
				},
			}, nil
		})
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		for _, pkg := range []tokens.Package{"aws", "gcp"} {
			_, err := monitor.RegisterResource(tokens.Type(string(pkg)+":index:Res"), "res", true)
			if err != nil {
				return err
			}
		}
		return nil
	})
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			HostF:            deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loader("aws"), loader("gcp")),
			SkipDisplayTests: true,
		},
	}
	p.Options.Sandbox = newSandbox(t)

	snap, err := lt.TestOp(Update).Run(p.GetProject(), localTarget(t, p, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)

	require.Contains(t, configured, "gcp")
	assert.Equal(t, "http://localhost:4588/storage/v1/", configured["gcp"].Get("storageCustomEndpoint").AsString())
	assert.Equal(t, "floci-local-token", configured["gcp"].Get("accessToken").AsString())
	assert.Equal(t, "floci-local", configured["gcp"].Get("project").AsString())
	require.Contains(t, configured, "aws")
	assert.Equal(t, "test", configured["aws"].Get("accessKey").AsString())

	for _, r := range snap.Resources {
		if providers.IsProviderType(r.Type) {
			assert.NotContains(t, r.Inputs, resource.PropertyKey("storageCustomEndpoint"), r.URN)
			assert.NotContains(t, r.Inputs, resource.PropertyKey("accessToken"), r.URN)
			assert.NotContains(t, r.Inputs, resource.PropertyKey("accessKey"), r.URN)
		}
	}
}

// Packages a sandbox stack can't use are refused before the program runs, so nothing is deployed.
func TestSandboxRefusesUnsupportedPackagesUpFront(t *testing.T) {
	t.Parallel()

	var created atomic.Bool
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("aws", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(context.Context, plugin.CreateRequest) (plugin.CreateResponse, error) {
					created.Store(true)
					return plugin.CreateResponse{ID: "id", Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}
	var ran atomic.Bool
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		ran.Store(true)
		_, err := monitor.RegisterResource("aws:s3/bucket:Bucket", "bucket", true)
		return err
	},
		workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
			Name: "aws", Kind: apitype.ResourcePlugin, Version: &semver.Version{Major: 1},
		}},
		workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
			Name: "kubernetes", Kind: apitype.ResourcePlugin, Version: &semver.Version{Major: 4},
		}},
	)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			HostF:            deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...),
			SkipDisplayTests: true,
		},
	}
	p.Options.Sandbox = newSandbox(t)

	_, err := lt.TestOp(Update).Run(p.GetProject(), localTarget(t, p, nil), p.Options, false, p.BackendClient, nil)
	require.ErrorContains(t, err, `sandbox stacks can't use "kubernetes"`)
	assert.False(t, ran.Load(), "the program must not run")
	assert.False(t, created.Load(), "nothing may be deployed")
}

// A resource whose service the emulator doesn't offer is still sent to the provider; the user only gets a warning.
func TestSandboxUnsupportedServiceStillCreates(t *testing.T) {
	t.Parallel()

	var created atomic.Bool
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("gcp", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(context.Context, plugin.CreateRequest) (plugin.CreateResponse, error) {
					created.Store(true)
					return plugin.CreateResponse{ID: "id", Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("gcp:compute/address:Address", "ip", true)
		return err
	})
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			HostF:            deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...),
			SkipDisplayTests: true,
		},
	}
	mode, err := sandbox.New(sandbox.Options{
		Ensure: func(context.Context, string, bool, diag.Sink) (sandbox.Emulator, error) {
			return sandbox.Emulator{
				Endpoint: "http://localhost:4588", DisplayName: "floci-gcp 0.9.0", Services: map[string]bool{"gcs": true},
			}, nil
		},
	})
	require.NoError(t, err)
	p.Options.Sandbox = mode

	_, err = lt.TestOp(Update).Run(p.GetProject(), localTarget(t, p, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)
	assert.True(t, created.Load())
}
