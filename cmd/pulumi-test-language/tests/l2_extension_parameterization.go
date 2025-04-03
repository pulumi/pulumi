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
	"context"
	"encoding/json"

	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-extension-parameterization"] = LanguageTest{
		Providers: []plugin.Provider{providers.NewExtensionProvider()},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					_ string,
					_ error,
					snap *deploy.Snapshot,
					changes display.ResourceChanges,
				) {
					d, err := stack.SerializeDeployment(context.TODO(), snap, false /*showSecrets*/)
					require.NoError(l, err, "failed to serialize deployment")

					jsonBytes, err := json.Marshal(d)
					require.NoError(l, err, "failed to marshal deployment")

					l.Logf("%s", string(jsonBytes))
				},
			},
		},
	}
}
