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

package tests

import (
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["provider-alias-component"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.ConformanceComponentProvider{} },
		},
		LanguageProviders: []string{"conformance-component"},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 6, "expected 6 resources in snapshot")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:conformance-component")
					component := RequireSingleResource(l, snap.Resources, "conformance-component:index:Simple")

					assert.Equal(l, stack.URN, component.Parent, "expected stack to be parent of component resource")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:simple")
					simple := RequireSingleNamedResource(l, snap.Resources, "res-child")

					assert.Equal(l, component.URN, simple.Parent, "expected component to be parent of simple resource")
				},
			},
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)

					// Only expect one create, the parent resource
					require.Equal(l, 1, changes[deploy.OpCreate], "expected exactly one create operation")

					require.Len(l, snap.Resources, 7, "expected 7 resources in snapshot")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					parent := RequireSingleNamedResource(l, snap.Resources, "parent")
					assert.Equal(l, stack.URN, parent.Parent, "expected stack to be parent of parent resource")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:conformance-component")
					component := RequireSingleResource(l, snap.Resources, "conformance-component:index:Simple")
					assert.Equal(l, parent.URN, component.Parent, "expected parent to be parent of component resource")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:simple")
					simple := RequireSingleNamedResource(l, snap.Resources, "res-child")

					assert.Equal(l, component.URN, simple.Parent, "expected component to be parent of simple resource")
				},
			},
		},
	}
}
