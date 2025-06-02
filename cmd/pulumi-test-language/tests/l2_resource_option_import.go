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
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-import"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)

					// * The stack
					// * The default provider
					// * The import resource, which has an import ID and will be read
					// * The notImport resource, which has no import ID and will be created
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					importRes := RequireSingleNamedResource(l, snap.Resources, "import")
					notImportRes := RequireSingleNamedResource(l, snap.Resources, "notImport")

					require.Equal(l, importRes.ImportID.String(), "fakeID123", "expected import resource to have import ID")
					require.Equal(l, notImportRes.ImportID.String(), "", "expected import resource to not have import ID")
				},
			},
		},
	}
}
