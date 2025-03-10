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

package fuzzing

import (
	"fmt"
	"slices"
	"strings"
	"time"

	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

// GenerateReproTest generates a string containing Go code for a set of lifecycle tests that reproduce the scenario
// captured by the given *Specs.
func GenerateReproTest(
	t lt.TB,
	sso StackSpecOptions,
	snapSpec *SnapshotSpec,
	progSpec *ProgramSpec,
	provSpec *ProviderSpec,
	planSpec *PlanSpec,
) string {
	var b strings.Builder

	writeHeader(&b)
	writePackageImports(&b)

	g := &generator{b: &b}
	writeSnapshotTestFunction(t, g, sso, snapSpec, progSpec, provSpec, planSpec)
	g.writeLine("")
	writeFrameworkTestFunction(t, g, sso, snapSpec, progSpec, provSpec, planSpec)

	return b.String()
}

// writeHeader writes the standard Pulumi license header to the given strings.Builder.
func writeHeader(b *strings.Builder) {
	year := time.Now().Year()

	b.WriteString(fmt.Sprintf(`// Copyright %d, Pulumi Corporation.
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

`, year))
}

// writePackageImports writes a superset of the imports that we'll need for a generated lifecycle test.
func writePackageImports(b *strings.Builder) {
	b.WriteString(`package lifecycletest

import (
	"context"
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
)
`)
}

// generator encapsulates writing indented Go code to a strings.Builder.
type generator struct {
	// The underlying strings.Builder to write to.
	b *strings.Builder
	// The current indentation string (which will typically be a possibly empty series of tabs).
	prefix string
}

// indent increases the indentation level of the generator.
func (g *generator) indent() {
	g.prefix += "\t"
}

// dedent decreases the indentation level of the generator.
func (g *generator) dedent() {
	g.prefix = g.prefix[:len(g.prefix)-1]
}

// writeLine writes a newline-prefixed line of Go code to the generator's strings.Builder, prefixed by the current
// indentation level.
func (g *generator) writeLine(s string) {
	g.b.WriteString("\n" + g.prefix + s)
}

// writeLinef writes a formatted newline-prefixed line of Go code to the generator's strings.Builder, prefixed by the
// current indentation level.
func (g *generator) writeLinef(format string, args ...interface{}) {
	g.writeLine(fmt.Sprintf(format, args...))
}

// writeBlock writes a newline-prefixed block of Go code to the generator's strings.Builder. The block's prefix and
// suffix will be indented at the current level, while the block's contents will be indented one level deeper. For
// example, the call:
//
//	writeBlock(
//	  "func foo() {",
//	  func(g *generator) {
//	    g.writeLine("bar := 42")
//	  },
//	  "}",
//	)
//
// will yield:
//
//	func foo() {
//	  bar := 42
//	}
func (g *generator) writeBlock(
	prefix string,
	block func(g *generator),
	suffix string,
) {
	g.writeLine(prefix)
	g.indent()
	block(g)
	g.dedent()
	g.writeLine(suffix)
}

// writeSnapshotTestFunction writes a Go test function that reproduces the scenario captured by the given *Specs using
// a hard-coded initial snapshot. This is useful for reducing a fuzzed test case to a minimal reproduction, which can
// then be mocked up as a test case using only methods from the lifecycle test framework.
func writeSnapshotTestFunction(
	t require.TestingT,
	g *generator,
	sso StackSpecOptions,
	snapSpec *SnapshotSpec,
	progSpec *ProgramSpec,
	provSpec *ProviderSpec,
	planSpec *PlanSpec,
) {
	g.writeLine("// TestReproSnapshot reproduces a failing fuzz test using a hard-coded starting snapshot.")
	g.writeBlock(
		"func TestReproSnapshot(t *testing.T) {",
		func(g *generator) {
			g.writeLine("t.Parallel()")

			g.writeLine("")
			g.writeBlock(
				"p := &lt.TestPlan{",
				func(g *generator) {
					g.writeLinef(`Project: "%s",`, sso.Project)
					g.writeLinef(`Stack: "%s",`, sso.Stack)
				},
				"}",
			)
			g.writeLine("project := p.GetProject()")

			g.writeLine("")
			g.writeLine("// Set up the initial snapshot.")
			g.writeBlock(
				"setupSnap := func() *deploy.Snapshot {",
				writeSnapshotStatements(t, snapSpec),
				"}()",
			)
			g.writeLine("require.NoError(t, setupSnap.VerifyIntegrity(), \"initial snapshot is not valid\")")

			g.writeLine("")
			g.writeLine("// Set up the reproduction providers and program.")
			g.writeBlock(
				"createF := func(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {",
				writeCreateFStatements(provSpec),
				"}",
			)
			g.writeBlock(
				"deleteF := func(ctx context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {",
				writeDeleteFStatements(provSpec),
				"}",
			)
			g.writeBlock(
				"diffF := func(ctx context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {",
				writeDiffFStatements(provSpec),
				"}",
			)
			g.writeBlock(
				"readF := func(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {",
				writeReadFStatements(provSpec),
				"}",
			)
			g.writeBlock(
				"updateF := func(ctx context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {",
				writeUpdateFStatements(provSpec),
				"}",
			)

			g.writeLine("")
			g.writeBlock(
				"reproLoaders := []*deploytest.ProviderLoader{",
				writeReproLoaderElements(provSpec),
				"}",
			)

			g.writeLine("")
			g.writeBlock(
				"reproProgramF := deploytest.NewLanguageRuntimeF("+
					"func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {",
				writeResourceRegistrationStatements(t, progSpec.ResourceRegistrations),
				"})",
			)

			g.writeLine("")
			g.writeLine("reproHostF := deploytest.NewPluginHostF(nil, nil, reproProgramF, reproLoaders...)")
			g.writeBlock(
				"reproOpts := lt.TestUpdateOptions{",
				func(g *generator) {
					g.writeLine("T: t,")
					g.writeLine("HostF: reproHostF,")
					g.writeBlock(
						"UpdateOptions: engine.UpdateOptions{",
						func(g *generator) {
							if len(planSpec.TargetURNs) > 0 {
								g.writeBlock(
									"Targets: deploy.NewUrnTargets([]string{",
									func(g *generator) {
										for _, urn := range planSpec.TargetURNs {
											g.writeLinef(`"%s",`, urn)
										}
									},
									"}),",
								)
							}
						},
						"},",
					)
				},
				"}",
			)

			var operation string
			switch planSpec.Operation {
			case PlanOperationUpdate:
				operation = "engine.Update"
			case PlanOperationRefresh:
				operation = "engine.Refresh"
			case PlanOperationDestroy:
				operation = "engine.Destroy"
			case PlanOperationDestroyV2:
				operation = "engine.DestroyV2"
			}

			g.writeLine("")
			g.writeLine("// Trigger the reproduction.")
			g.writeLinef(
				"reproSnap, err := "+
					"lt.TestOp(%s).RunStep(project, p.GetTarget(t, setupSnap), reproOpts, false, p.BackendClient, nil, \"1\")",
				operation,
			)
			g.writeLine("require.NoError(t, err)")
		},
		"}",
	)
}

// writeFrameworkTestFunction writes a Go test function that reproduces the scenario captured by the given *Specs using
// only methods from the lifecycle test framework. That is, it uses a test resource monitor to register resources to
// build an initial snapshot, before triggering a deployment.
func writeFrameworkTestFunction(
	t require.TestingT,
	g *generator,
	sso StackSpecOptions,
	snapSpec *SnapshotSpec,
	progSpec *ProgramSpec,
	provSpec *ProviderSpec,
	planSpec *PlanSpec,
) {
	g.writeLine("// TestReproFramework reproduces a failing fuzz test using only the lifecycle test framework.")
	g.writeBlock(
		"func TestReproFramework(t *testing.T) {",
		func(g *generator) {
			g.writeLine("t.Parallel()")

			g.writeLine("")
			g.writeBlock(
				"p := &lt.TestPlan{",
				func(g *generator) {
					g.writeLinef(`Project: "%s",`, sso.Project)
					g.writeLinef(`Stack: "%s",`, sso.Stack)
				},
				"}",
			)
			g.writeLine("project := p.GetProject()")

			g.writeLine("")
			g.writeLine("// Set up the initial snapshot.")
			g.writeBlock(
				"setupLoaders := []*deploytest.ProviderLoader{",
				writeSetupLoaderElements(provSpec),
				"}",
			)

			g.writeLine("")
			g.writeBlock(
				"setupProgramF := deploytest.NewLanguageRuntimeF("+
					"func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {",
				writeResourceRegistrationStatements(t, snapSpec.Resources),
				"})",
			)

			g.writeLine("")
			g.writeLine("setupHostF := deploytest.NewPluginHostF(nil, nil, setupProgramF, setupLoaders...)")
			g.writeBlock(
				"setupOpts := lt.TestUpdateOptions{",
				func(g *generator) {
					g.writeLine("T: t,")
					g.writeLine("HostF: setupHostF,")
				},
				"}",
			)

			g.writeLine("setupSnap, err := lt.TestOp(engine.Update)." +
				"RunStep(project, p.GetTarget(t, nil), setupOpts, false, p.BackendClient, nil, \"0\")")
			g.writeLine("require.NoError(t, err)")

			g.writeLine("")
			g.writeLine("// Set up the reproduction providers and program.")
			g.writeBlock(
				"createF := func(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {",
				writeCreateFStatements(provSpec),
				"}",
			)
			g.writeBlock(
				"deleteF := func(ctx context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {",
				writeDeleteFStatements(provSpec),
				"}",
			)
			g.writeBlock(
				"diffF := func(ctx context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {",
				writeDiffFStatements(provSpec),
				"}",
			)
			g.writeBlock(
				"readF := func(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {",
				writeReadFStatements(provSpec),
				"}",
			)
			g.writeBlock(
				"updateF := func(ctx context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {",
				writeUpdateFStatements(provSpec),
				"}",
			)

			g.writeLine("")
			g.writeBlock(
				"reproLoaders := []*deploytest.ProviderLoader{",
				writeReproLoaderElements(provSpec),
				"}",
			)

			g.writeLine("")
			g.writeBlock(
				"reproProgramF := deploytest.NewLanguageRuntimeF("+
					"func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {",
				writeResourceRegistrationStatements(t, progSpec.ResourceRegistrations),
				"})",
			)

			g.writeLine("")
			g.writeLine("reproHostF := deploytest.NewPluginHostF(nil, nil, reproProgramF, reproLoaders...)")
			g.writeBlock(
				"reproOpts := lt.TestUpdateOptions{",
				func(g *generator) {
					g.writeLine("T: t,")
					g.writeLine("HostF: reproHostF,")
					g.writeBlock(
						"UpdateOptions: engine.UpdateOptions{",
						func(g *generator) {
							if len(planSpec.TargetURNs) > 0 {
								g.writeBlock(
									"Targets: deploy.NewUrnTargets([]string{",
									func(g *generator) {
										for _, urn := range planSpec.TargetURNs {
											g.writeLinef(`"%s",`, urn)
										}
									},
									"}),",
								)
							}
						},
						"},",
					)
				},
				"}",
			)

			var operation string
			switch planSpec.Operation {
			case PlanOperationUpdate:
				operation = "engine.Update"
			case PlanOperationRefresh:
				operation = "engine.Refresh"
			case PlanOperationDestroy:
				operation = "engine.Destroy"
			case PlanOperationDestroyV2:
				operation = "engine.DestroyV2"
			}

			g.writeLine("")
			g.writeLine("// Trigger the reproduction.")
			g.writeLinef(
				"reproSnap, err := "+
					"lt.TestOp(%s).RunStep(project, p.GetTarget(t, setupSnap), reproOpts, false, p.BackendClient, nil, \"1\")",
				operation,
			)
			g.writeLine("require.NoError(t, err)")
		},
		"}",
	)
}

// writeSnapshotStatements writes a series of Go statements that build a deploy.Snapshot according to the given
// SnapshotSpec.
//
//	s := &deploy.Snapshot{}
//
//	res0 := &resource.State{
//	  Type: ...,
//	  URN: ...,
//	  ...
//	}
//	s.Resources = append(s.Resources, res0)
//
//	...
//
//	return s
func writeSnapshotStatements(t require.TestingT, snapSpec *SnapshotSpec) func(g *generator) {
	return func(g *generator) {
		indicesByURN := map[resource.URN]int{}
		varFor := func(urn resource.URN) string {
			if urn == "" {
				return ""
			}

			if i, has := indicesByURN[urn]; has {
				if providers.IsProviderType(urn.Type()) {
					return fmt.Sprintf("prov%d", i)
				}

				return fmt.Sprintf("res%d", i)
			}

			return "unknownVar"
		}

		provRefVarsByRef := map[string]string{}
		provRefVarFor := func(refStr string) string {
			if refStr == "" {
				return ""
			}

			if refVar, has := provRefVarsByRef[refStr]; has {
				return refVar
			}

			ref, err := providers.ParseReference(refStr)
			require.NoError(t, err)

			if i, has := indicesByURN[ref.URN()]; has {
				refVar := fmt.Sprintf("provRef%d", i)
				provRefVarsByRef[refStr] = refVar

				g.writeLine("")
				g.writeLinef(
					"%s, err := providers.NewReference(%s.URN, %s.ID)",
					refVar, varFor(ref.URN()), varFor(ref.URN()),
				)
				g.writeLine("require.NoError(t, err)")

				return refVar
			}

			return "unknownProvRef"
		}

		g.writeLine("s := &deploy.Snapshot{}")

		for i, r := range snapSpec.Resources {
			indicesByURN[r.URN()] = i

			provRefVar := provRefVarFor(r.Provider)

			g.writeLine("")
			g.writeBlock(
				varFor(r.URN())+" := &resource.State{",
				func(g *generator) {
					g.writeLinef("Type:               \"%s\",", r.Type)
					g.writeLinef("URN:                \"%s\",", r.URN())
					g.writeLinef("Custom:             %v,", r.Custom)

					if r.Delete {
						g.writeLine("Delete:             true,")
					}

					g.writeLinef("ID:                 \"%s\",", r.ID)

					if r.Protect {
						g.writeLine("Protect:            true,")
					}

					if r.PendingReplacement {
						g.writeLine("PendingReplacement: true,")
					}

					if r.RetainOnDelete {
						g.writeLine("RetainOnDelete:     true,")
					}

					if r.Provider != "" {
						g.writeLinef("Provider: %s.String(),", provRefVar)
					}

					if r.Parent != "" {
						g.writeLinef("Parent: %s.URN,", varFor(r.Parent))
					}

					if len(r.Dependencies) > 0 {
						g.writeBlock(
							"Dependencies: []resource.URN{",
							func(g *generator) {
								for _, dep := range r.Dependencies {
									g.writeLine(varFor(dep) + ".URN,")
								}
							},
							"},",
						)
					}

					if len(r.PropertyDependencies) > 0 {
						g.writeBlock(
							"PropertyDeps: map[resource.PropertyKey][]resource.URN{",
							func(g *generator) {
								for k, deps := range r.PropertyDependencies {
									g.writeBlock(
										fmt.Sprintf("\"%s\": {", k),
										func(g *generator) {
											for _, dep := range deps {
												g.writeLine(varFor(dep) + ".URN,")
											}
										},
										"},",
									)
								}
							},
							"},",
						)
					}

					if r.DeletedWith != "" {
						g.writeLinef("DeletedWith: %s.URN,", varFor(r.DeletedWith))
					}

					if len(r.Aliases) > 0 {
						g.writeBlock(
							"Aliases: []resource.URN{",
							func(g *generator) {
								for _, alias := range r.Aliases {
									g.writeLinef("\"%s\",", alias)
								}
							},
							"},",
						)
					}

					if !providers.IsProviderType(r.Type) {
						g.writeBlock(
							"Inputs: resource.PropertyMap{",
							func(g *generator) {
								g.writeLinef("\"__id\": resource.NewStringProperty(\"%s\"),", r.ID.String())
							},
							"},",
						)
					}
				},
				"}",
			)

			g.writeLinef("s.Resources = append(s.Resources, %s)", varFor(r.URN()))
		}

		g.writeLine("")
		g.writeLine("return s")
	}
}

// writeSetupLoaderElements writes a series of Go expressions that set up a series of deploytest.ProviderLoaders
// according to the given ProviderSpec. These expressions are designed to be used in an array constructor.
//
//	deploytest.NewProviderLoader("pkg0", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
//	  return &deploytest.Provider{...}, nil
//	}),
//	...
func writeSetupLoaderElements(provSpec *ProviderSpec) func(g *generator) {
	return func(g *generator) {
		pkgs := maps.Keys(provSpec.Packages)
		slices.Sort(pkgs)

		for _, pkg := range pkgs {
			g.writeBlock(
				fmt.Sprintf(
					"deploytest.NewProviderLoader(\"%s\", semver.MustParse(\"1.0.0\"), func() (plugin.Provider, error) {",
					pkg,
				),
				func(g *generator) {
					g.writeLine("return &deploytest.Provider{}, nil")
				},
				"}),",
			)
		}
	}
}

// writeResourceRegistrationStatements writes a series of Go statements that register a series of resources with a
// deploytest.ResourceMonitor according to the given ProgramSpec.
//
//	res0, err := monitor.RegisterResource("pkg0:typ0", "res0", false, deploytest.ResourceOptions{...})
//	require.NoError(t, err)
//
//	...
func writeResourceRegistrationStatements(t require.TestingT, rs []*ResourceSpec) func(g *generator) {
	return func(g *generator) {
		indicesByURN := map[resource.URN]int{}
		varFor := func(urn resource.URN) string {
			if urn == "" {
				return ""
			}

			if i, has := indicesByURN[urn]; has {
				if providers.IsProviderType(urn.Type()) {
					return fmt.Sprintf("prov%d", i)
				}

				return fmt.Sprintf("res%d", i)
			}

			return "unknownVar"
		}

		provRefVarsByRef := map[string]string{}
		provRefVarFor := func(refStr string) string {
			if refStr == "" {
				return ""
			}

			if refVar, has := provRefVarsByRef[refStr]; has {
				return refVar
			}

			ref, err := providers.ParseReference(refStr)
			require.NoError(t, err)

			if i, has := indicesByURN[ref.URN()]; has {
				refVar := fmt.Sprintf("provRef%d", i)
				provRefVarsByRef[refStr] = refVar

				g.writeLine(fmt.Sprintf(
					"%s, err := providers.NewReference(%s.URN, %s.ID)",
					refVar, varFor(ref.URN()), varFor(ref.URN()),
				))
				g.writeLine("require.NoError(t, err)")
				g.writeLine("")

				return refVar
			}

			return "unknownProvRef"
		}

		for i, r := range rs {
			indicesByURN[r.URN()] = i

			provRefVar := provRefVarFor(r.Provider)

			g.writeBlock(
				fmt.Sprintf(
					"%s, err := monitor.RegisterResource(\"%s\", \"%s\", %v, deploytest.ResourceOptions{",
					varFor(r.URN()), r.Type, r.Name, r.Custom,
				),
				func(g *generator) {
					if r.Delete {
						g.writeLine("// You'll need to set up a means for Delete: true to be set on this resource")
						g.writeLine("// Delete: true,")
					}
					if r.PendingReplacement {
						g.writeLine("// You'll need to set up a means for PendingReplacement: true to be set on this resource")
						g.writeLine("// PendingReplacement: true,")
					}

					if r.Protect {
						g.writeLine("Protect: true,")
					}
					if r.RetainOnDelete {
						g.writeLine("RetainOnDelete: true,")
					}

					if r.Provider != "" {
						g.writeLinef("Provider: %s.String(),", provRefVar)
					}

					if r.Parent != "" {
						g.writeLinef("Parent: %s.URN,", varFor(r.Parent))
					}

					if len(r.Dependencies) > 0 {
						g.writeBlock(
							"Dependencies: []resource.URN{",
							func(g *generator) {
								for _, dep := range r.Dependencies {
									g.writeLine(varFor(dep) + ".URN,")
								}
							},
							"},",
						)
					}

					if len(r.PropertyDependencies) > 0 {
						g.writeBlock(
							"PropertyDeps: map[resource.PropertyKey][]resource.URN{",
							func(g *generator) {
								for k, deps := range r.PropertyDependencies {
									g.writeBlock(
										fmt.Sprintf("\"%s\": {", k),
										func(g *generator) {
											for _, dep := range deps {
												g.writeLine(varFor(dep) + ".URN,")
											}
										},
										"},",
									)
								}
							},
							"},",
						)
					}

					if r.DeletedWith != "" {
						g.writeLinef("DeletedWith: %s.URN,", varFor(r.DeletedWith))
					}

					if len(r.Aliases) > 0 {
						g.writeBlock(
							"AliasURNs: []resource.URN{",
							func(g *generator) {
								for _, alias := range r.Aliases {
									g.writeLinef("\"%s\",", alias)
								}
							},
							"},",
						)
					}
				},
				"})",
			)
			g.writeLine("require.NoError(t, err)")
			g.writeLine("")
		}

		g.writeLine("return nil")
	}
}

func writeCreateFStatements(provSpec *ProviderSpec) func(g *generator) {
	return func(g *generator) {
		if len(provSpec.Create) > 0 {
			g.writeLine("switch req.URN {")
			for urn := range provSpec.Create {
				g.writeBlock(
					fmt.Sprintf("case \"%s\":", urn),
					func(g *generator) {
						g.writeBlock(
							"return plugin.CreateResponse{",
							func(g *generator) {
								g.writeLine("Status: resource.StatusUnknown,")
							},
							"}, fmt.Errorf(\"create failure for %s\", req.URN)",
						)
					},
					"",
				)
			}
			g.writeLine("}")
		}

		g.writeBlock(
			"return plugin.CreateResponse{",
			func(g *generator) {
				g.writeLine("Properties: req.Properties,")
				g.writeLine("Status: resource.StatusOK,")
			},
			"}, nil",
		)
	}
}

func writeDeleteFStatements(provSpec *ProviderSpec) func(g *generator) {
	return func(g *generator) {
		if len(provSpec.Delete) > 0 {
			g.writeLine("switch req.URN {")
			for urn := range provSpec.Delete {
				g.writeBlock(
					fmt.Sprintf("case \"%s\":", urn),
					func(g *generator) {
						g.writeBlock(
							"return plugin.DeleteResponse{",
							func(g *generator) {
								g.writeLine("Status: resource.StatusUnknown,")
							},
							"}, fmt.Errorf(\"delete failure for %s\", req.URN)",
						)
					},
					"",
				)
			}
			g.writeLine("}")
		}

		g.writeBlock(
			"return plugin.DeleteResponse{",
			func(g *generator) {
				g.writeLine("Status: resource.StatusOK,")
			},
			"}, nil",
		)
	}
}

func writeDiffFStatements(provSpec *ProviderSpec) func(g *generator) {
	return func(g *generator) {
		if len(provSpec.Diff) > 0 {
			g.writeLine("switch req.URN {")
			for urn, action := range provSpec.Diff {
				g.writeBlock(
					fmt.Sprintf("case \"%s\":", urn),
					func(g *generator) {
						switch action {
						case ProviderDiffDeleteBeforeReplace:
							g.writeBlock(
								"return plugin.DiffResponse{",
								func(g *generator) {
									g.writeLine("Changes: plugin.DiffSome,")
									g.writeLine("ReplaceKeys: []resource.PropertyKey{\"__replace\"},")
									g.writeLine("DeleteBeforeReplace: true,")
								},
								"}, nil",
							)
						case ProviderDiffDeleteAfterReplace:
							g.writeBlock(
								"return plugin.DiffResponse{",
								func(g *generator) {
									g.writeLine("Changes: plugin.DiffSome,")
									g.writeLine("ReplaceKeys: []resource.PropertyKey{\"__replace\"},")
									g.writeLine("DeleteBeforeReplace: false,")
								},
								"}, nil",
							)
						case ProviderDiffChange:
							g.writeLine("return plugin.DiffResponse{Changes: plugin.DiffSome}, nil")
						case ProviderDiffFailure:
							g.writeLine("return plugin.DiffResponse{}, fmt.Errorf(\"diff failure for %s\", req.URN)")
						}
					},
					"",
				)
			}
			g.writeLine("}")
		}

		g.writeLine("return plugin.DiffResponse{}, nil")
	}
}

func writeReadFStatements(provSpec *ProviderSpec) func(g *generator) {
	return func(g *generator) {
		if len(provSpec.Read) > 0 {
			g.writeLine("switch req.URN {")
			for urn, action := range provSpec.Read {
				g.writeBlock(
					fmt.Sprintf("case \"%s\":", urn),
					func(g *generator) {
						switch action {
						case ProviderReadDeleted:
							g.writeLine("return plugin.ReadResponse{}, nil")
						case ProviderReadFailure:
							g.writeBlock(
								"return plugin.ReadResponse{",
								func(g *generator) {
									g.writeLine("Status: resource.StatusUnknown,")
								},
								"}, fmt.Errorf(\"read failure for %s\", req.URN)",
							)
						}
					},
					"",
				)
			}
			g.writeLine("}")
		}

		g.writeBlock(
			"return plugin.ReadResponse{",
			func(g *generator) {
				g.writeLine("ReadResult: plugin.ReadResult{Outputs: resource.PropertyMap{}},")
				g.writeLine("Status: resource.StatusOK,")
			},
			"}, nil",
		)
	}
}

func writeUpdateFStatements(provSpec *ProviderSpec) func(g *generator) {
	return func(g *generator) {
		if len(provSpec.Update) > 0 {
			g.writeLine("switch req.URN {")
			for urn := range provSpec.Update {
				g.writeBlock(
					fmt.Sprintf("case \"%s\":", urn),
					func(g *generator) {
						g.writeBlock(
							"return plugin.UpdateResponse{",
							func(g *generator) {
								g.writeLine("Status: resource.StatusUnknown,")
							},
							"}, fmt.Errorf(\"update failure for %s\", req.URN)",
						)
					},
					"",
				)
			}
			g.writeLine("}")
		}

		g.writeBlock(
			"return plugin.UpdateResponse{",
			func(g *generator) {
				g.writeLine("Properties: req.NewInputs,")
				g.writeLine("Status: resource.StatusOK,")
			},
			"}, nil",
		)
	}
}

func writeReproLoaderElements(provSpec *ProviderSpec) func(g *generator) {
	return func(g *generator) {
		pkgs := maps.Keys(provSpec.Packages)
		slices.Sort(pkgs)

		for _, pkg := range pkgs {
			g.writeBlock(
				fmt.Sprintf(
					"deploytest.NewProviderLoader(\"%s\", semver.MustParse(\"1.0.0\"), func() (plugin.Provider, error) {",
					pkg,
				),
				func(g *generator) {
					g.writeBlock(
						"return &deploytest.Provider{",
						func(g *generator) {
							g.writeLine("CreateF: createF,")
							g.writeLine("DeleteF: deleteF,")
							g.writeLine("DiffF: diffF,")
							g.writeLine("ReadF: readF,")
							g.writeLine("UpdateF: updateF,")
						},
						"}, nil",
					)
				},
				"}),",
			)
		}
	}
}
