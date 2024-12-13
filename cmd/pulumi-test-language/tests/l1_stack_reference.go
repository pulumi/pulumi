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
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-stack-reference"] = LanguageTest{
		StackReferences: map[string]resource.PropertyMap{
			"organization/other/dev": {
				"plain":  resource.NewStringProperty("plain"),
				"secret": resource.MakeSecret(resource.NewStringProperty("secret")),
			},
		},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 3, "expected at least 3 resources")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")
					prov := snap.Resources[1]
					require.Equal(l, "pulumi:providers:pulumi", prov.Type.String(), "expected a default pulumi provider resource")
					ref := snap.Resources[2]
					require.Equal(l, "pulumi:pulumi:StackReference", ref.Type.String(), "expected a stack reference resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 2, "expected 2 outputs")
					AssertPropertyMapMember(l, outputs, "plain", resource.NewStringProperty("plain"))
					AssertPropertyMapMember(l, outputs, "secret", resource.MakeSecret(resource.NewStringProperty("secret")))
				},
			},
		},
	}
}
