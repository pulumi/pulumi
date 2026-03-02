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

package tests

import (
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-replace-on-changes"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ReplaceOnChangesProvider{} },
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.ComponentProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					snap := res.Snap

					RequireStackResource(l, res.Err, res.Changes)

					// Stack, 3 providers, 8 custom resources, 1 remote component + child, and 1 simple resource.
					require.Len(l, snap.Resources, 15, "expected 15 resources in snapshot")

					RequireSingleNamedResource(l, snap.Resources, "schemaReplace")
					RequireSingleNamedResource(l, snap.Resources, "optionReplace")
					RequireSingleNamedResource(l, snap.Resources, "bothReplaceValue")
					RequireSingleNamedResource(l, snap.Resources, "bothReplaceProp")
					RequireSingleNamedResource(l, snap.Resources, "regularUpdate")
					RequireSingleNamedResource(l, snap.Resources, "noChange")
					RequireSingleNamedResource(l, snap.Resources, "wrongPropChange")
					RequireSingleNamedResource(l, snap.Resources, "multiplePropReplace")
					RequireSingleNamedResource(l, snap.Resources, "remoteWithReplace")
					RequireSingleNamedResource(l, snap.Resources, "simpleResource")
				},
			},
			{
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					var schemaOp, optionOp, bothValueOp, bothPropOp, regularOp, noChangeOp,
						wrongPropOp, multipleOp display.StepOp

					for _, evt := range res.Events {
						switch e := evt.Payload().(type) {
						case engine.ResourcePreEventPayload:
							switch e.Metadata.URN.Name() {
							case "schemaReplace":
								schemaOp = e.Metadata.Op
							case "optionReplace":
								optionOp = e.Metadata.Op
							case "bothReplaceValue":
								bothValueOp = e.Metadata.Op
							case "bothReplaceProp":
								bothPropOp = e.Metadata.Op
							case "regularUpdate":
								regularOp = e.Metadata.Op
							case "noChange":
								noChangeOp = e.Metadata.Op
							case "wrongPropChange":
								wrongPropOp = e.Metadata.Op
							case "multiplePropReplace":
								multipleOp = e.Metadata.Op
							}
						}
					}

					require.Contains(l, schemaOp, deploy.OpReplace,
						"schemaReplace: replaceProp change should trigger replacement")
					require.Contains(l, optionOp, deploy.OpReplace,
						"optionReplace: value change should trigger replacement")
					require.Contains(l, bothValueOp, deploy.OpReplace,
						"bothReplaceValue: value change should trigger replacement")
					require.Contains(l, bothPropOp, deploy.OpReplace,
						"bothReplaceProp: replaceProp change should trigger replacement")
					require.Equal(l, regularOp, deploy.OpUpdate,
						"regularUpdate: value change should update, not replace")
					require.Equal(l, noChangeOp, deploy.OpSame,
						"noChange: no property change should result in no operation")
					require.Contains(l, wrongPropOp, deploy.OpReplace,
						"wrongPropChange: replaceProp has schema-based replaceOnChanges, should trigger replacement")
					require.Contains(l, multipleOp, deploy.OpReplace,
						"multiplePropReplace: change to any marked property should trigger replacement")
				},
				Assert: func(l *L, res AssertArgs) {
					require.Len(l, res.Snap.Resources, 15, "expected 15 resources in snapshot")
					RequireSingleNamedResource(l, res.Snap.Resources, "remoteWithReplace")
					RequireSingleNamedResource(l, res.Snap.Resources, "simpleResource")

					var schemaOp, optionOp, bothValueOp, bothPropOp, regularOp, noChangeOp,
						wrongPropOp, multipleOp display.StepOp

					for _, evt := range res.Events {
						switch e := evt.Payload().(type) {
						case engine.ResourcePreEventPayload:
							switch e.Metadata.URN.Name() {
							case "schemaReplace":
								schemaOp = e.Metadata.Op
							case "optionReplace":
								optionOp = e.Metadata.Op
							case "bothReplaceValue":
								bothValueOp = e.Metadata.Op
							case "bothReplaceProp":
								bothPropOp = e.Metadata.Op
							case "regularUpdate":
								regularOp = e.Metadata.Op
							case "noChange":
								noChangeOp = e.Metadata.Op
							case "wrongPropChange":
								wrongPropOp = e.Metadata.Op
							case "multiplePropReplace":
								multipleOp = e.Metadata.Op
							}
						}
					}

					require.Contains(l, schemaOp, deploy.OpReplace,
						"schemaReplace: should have been replaced")
					require.Contains(l, optionOp, deploy.OpReplace,
						"optionReplace: should have been replaced")
					require.Contains(l, bothValueOp, deploy.OpReplace,
						"bothReplaceValue: should have been replaced")
					require.Contains(l, bothPropOp, deploy.OpReplace,
						"bothReplaceProp: should have been replaced")
					require.Equal(l, regularOp, deploy.OpUpdate,
						"regularUpdate: should have been updated, not replaced")
					require.Equal(l, noChangeOp, deploy.OpSame,
						"noChange: should have no operation")
					require.Contains(l, wrongPropOp, deploy.OpReplace,
						"wrongPropChange: should have been replaced (schema-based replaceOnChanges on replaceProp)")
					require.Contains(l, multipleOp, deploy.OpReplace,
						"multiplePropReplace: should have been replaced")
				},
			},
		},
	}
}
