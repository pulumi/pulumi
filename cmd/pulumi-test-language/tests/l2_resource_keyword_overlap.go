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

					RequireSingleResource(l, snap.Resources, "pulumi:providers:simple")

					classRes := RequireSingleNamedResource(l, snap.Resources, "class")
					assert.Equal(l, "simple:index:Resource", classRes.Type.String())

					exportRes := RequireSingleNamedResource(l, snap.Resources, "export")
					assert.Equal(l, "simple:index:Resource", exportRes.Type.String())

					modRes := RequireSingleNamedResource(l, snap.Resources, "mod")
					assert.Equal(l, "simple:index:Resource", modRes.Type.String())

					importRes := RequireSingleNamedResource(l, snap.Resources, "import")
					assert.Equal(l, "simple:index:Resource", importRes.Type.String())

					objectRes := RequireSingleNamedResource(l, snap.Resources, "object")
					assert.Equal(l, "simple:index:Resource", objectRes.Type.String())

					selfRes := RequireSingleNamedResource(l, snap.Resources, "self")
					assert.Equal(l, "simple:index:Resource", selfRes.Type.String())

					thisRes := RequireSingleNamedResource(l, snap.Resources, "this")
					assert.Equal(l, "simple:index:Resource", thisRes.Type.String())
				},
			},
		},
	}
}
