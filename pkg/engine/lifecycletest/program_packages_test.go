// Copyright 2025, Pulumi Corporation.
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
	"bytes"
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
)

func TestRefreshGatherPackagesFromProgramFailure(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(test plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		return nil
	}, deploytest.LanguageRuntimeOptions{
		GetRequiredPackagesFailure: true,
	})

	var output bytes.Buffer
	sink := diag.DefaultSink(&output, &output, diag.FormatOptions{Color: colors.Never, Debug: true})

	hostF := deploytest.NewPluginHostF(sink, sink, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	assert.Len(t, snap.Resources, 2)

	p.Steps = []lt.TestStep{{Op: Refresh}}
	snap = p.Run(t, snap)

	assert.Len(t, snap.Resources, 2)
	// TODO: Is this how we should be getting the message back from the engine?
	// assert.Contains(t, output.String(), "failed to gather packages from program")
}

func TestRefreshGatherPackagesFromProgramSuccess(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(test plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		return nil
	}, deploytest.LanguageRuntimeOptions{
		GetRequiredPackagesFailure: false,
	})

	var output bytes.Buffer
	sink := diag.DefaultSink(&output, &output, diag.FormatOptions{Color: colors.Never, Debug: true})

	hostF := deploytest.NewPluginHostF(sink, sink, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	assert.Len(t, snap.Resources, 2)

	p.Steps = []lt.TestStep{{Op: Refresh}}
	snap = p.Run(t, snap)

	assert.Len(t, snap.Resources, 2)
	// TODO: Is this how we should be getting the message back from the engine?
	// assert.Contains(t, output.String(), "failed to gather packages from program")
}

func TestDestroyGatherPackagesFromProgramFailure(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(test plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		return nil
	}, deploytest.LanguageRuntimeOptions{
		GetRequiredPackagesFailure: true,
	})

	var output bytes.Buffer
	sink := diag.DefaultSink(&output, &output, diag.FormatOptions{Color: colors.Never, Debug: true})

	hostF := deploytest.NewPluginHostF(sink, sink, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	assert.Len(t, snap.Resources, 2)

	p.Steps = []lt.TestStep{{Op: Destroy}}
	snap = p.Run(t, snap)

	assert.Len(t, snap.Resources, 0)
	// TODO: Is this how we should be getting the message back from the engine?
	// assert.Contains(t, output.String(), "failed to gather packages from program")
}

func TestDestroyGatherPackagesFromProgramSuccess(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(test plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		return nil
	}, deploytest.LanguageRuntimeOptions{
		GetRequiredPackagesFailure: false,
	})

	var output bytes.Buffer
	sink := diag.DefaultSink(&output, &output, diag.FormatOptions{Color: colors.Never, Debug: true})

	hostF := deploytest.NewPluginHostF(sink, sink, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	assert.Len(t, snap.Resources, 2)

	p.Steps = []lt.TestStep{{Op: Destroy}}
	snap = p.Run(t, snap)

	assert.Len(t, snap.Resources, 0)
	// TODO: Is this how we should be getting the message back from the engine?
	// assert.Contains(t, output.String(), "failed to gather packages from program")
}
