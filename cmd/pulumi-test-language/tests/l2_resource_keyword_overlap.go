// Copyright 2024, Pulumi Corporation.
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
	"slices"

	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
)

func init() {
	LanguageTests["l2-resource-keyword-overlap"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")

					resourceNames := []string{}
					for _, res := range snap.Resources {
						resourceNames = append(resourceNames, res.URN.Name())
					}

					assert.Equal(l, "simple:index:Resource", snap.Resources[2].Type.String())
					assert.True(l, slices.Contains(resourceNames, "class"))

					assert.Equal(l, "simple:index:Resource", snap.Resources[3].Type.String())
					assert.True(l, slices.Contains(resourceNames, "export"))

					assert.Equal(l, "simple:index:Resource", snap.Resources[4].Type.String())
					assert.True(l, slices.Contains(resourceNames, "mod"))

					assert.Equal(l, "simple:index:Resource", snap.Resources[5].Type.String())
					assert.True(l, slices.Contains(resourceNames, "import"))

					assert.Equal(l, "simple:index:Resource", snap.Resources[6].Type.String())
					assert.True(l, slices.Contains(resourceNames, "object"))

					assert.Equal(l, "simple:index:Resource", snap.Resources[7].Type.String())
					assert.True(l, slices.Contains(resourceNames, "self"))

					assert.Equal(l, "simple:index:Resource", snap.Resources[8].Type.String())
					assert.True(l, slices.Contains(resourceNames, "this"))
				},
			},
		},
	}
}
