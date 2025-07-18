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
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-proxy-index"] = LanguageTest{
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l1-proxy-index", "anObject"):  config.NewObjectValue("{\"property\":\"config\"}"),
					config.MustMakeKey("l1-proxy-index", "anyObject"): config.NewObjectValue("{\"property\":\"dynamic\"}"),
				},
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					require.Len(l, outputs, 5, "expected 5 outputs")
					AssertPropertyMapMember(l, outputs, "l", resource.MakeSecret(resource.NewNumberProperty(1)))
					AssertPropertyMapMember(l, outputs, "m", resource.MakeSecret(resource.NewBoolProperty(true)))
					AssertPropertyMapMember(l, outputs, "c", resource.MakeSecret(resource.NewStringProperty("config")))
					AssertPropertyMapMember(l, outputs, "o", resource.MakeSecret(resource.NewStringProperty("value")))
					AssertPropertyMapMember(l, outputs, "a", resource.MakeSecret(resource.NewStringProperty("dynamic")))
				},
			},
		},
	}
}
