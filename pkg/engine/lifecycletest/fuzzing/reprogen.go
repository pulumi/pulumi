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

	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

func FixtureCode(
	t lt.TB,
	sso StackSpecOptions,
	snapSpec *SnapshotSpec,
	progSpec *ProgramSpec,
	provSpec *ProviderSpec,
	planSpec *PlanSpec,
) string {
	var b strings.Builder

	// Preamble

	b.WriteString(fmt.Sprintf(`package lifecycletest

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func TestRepro(t *testing.T) {
	p := &TestPlan{
		Project: "%s",
		Stack:   "%s",
	}
	project := p.GetProject()`,
		sso.Project,
		sso.Stack,
	))

	// Setup loaders
	b.WriteString("\n\n\tsetupLoaders := []*deploytest.ProviderLoader{")

	pkgs := maps.Keys(provSpec.Packages)
	slices.Sort(pkgs)

	for _, pkg := range pkgs {
		b.WriteString(fmt.Sprintf(`
		deploytest.NewProviderLoader("%s", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),`,
			pkg,
		))
	}

	b.WriteString("\n\t}")

	// Setup program (imperfect)
	b.WriteString(
		"\n\n\tsetupProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {",
	)

	indicesByURN := map[resource.URN]int{}
	varFor := func(urn resource.URN) string {
		if i, has := indicesByURN[urn]; has {
			return fmt.Sprintf("res%d", i)
		}

		return "resUnknown"
	}

	for i, r := range snapSpec.Resources {
		indicesByURN[r.URN()] = i

		if r.Provider != "" {
			ref, err := providers.ParseReference(r.Provider)
			require.NoError(t, err)

			b.WriteString(fmt.Sprintf("\n\t\tres%dProvRef, err := providers.NewReference(%[2]s.URN, %[2]s.ID)", i, varFor(ref.URN())))
			b.WriteString("\n\t\trequire.NoError(t, err)")
		}

		b.WriteString(fmt.Sprintf(
			"\n\t\tres%d, err := monitor.RegisterResource(\"%s\", \"%s\", %v, deploytest.ResourceOptions{",
			i, r.Type, r.Name, r.Custom,
		))

		if r.Delete {
			b.WriteString("\n\t\t\t// You'll need to set-up a means for Delete: true to be set on this resource")
			b.WriteString("\n\t\t\t// Delete: true,")
		}
		if r.PendingReplacement {
			b.WriteString("\n\t\t\t// You'll need to set-up a means for PendingReplacement: true to be set on this resource")
			b.WriteString("\n\t\t\t// PendingReplacement: true,")
		}

		if r.Protect {
			b.WriteString("\n\t\t\tProtect: true,")
		}
		if r.RetainOnDelete {
			b.WriteString("\n\t\t\tRetainOnDelete: true,")
		}

		if r.Provider != "" {
			b.WriteString(fmt.Sprintf("\n\t\t\tProvider: res%dProvRef.String(),", i))
		}

		if len(r.Dependencies) > 0 {
			b.WriteString("\n\t\t\tDependencies: []resource.URN{")
			for _, dep := range r.Dependencies {
				b.WriteString(fmt.Sprintf("\n\t\t\t\t%s.URN,", varFor(dep)))
			}
			b.WriteString("\n\t\t\t},")
		}

		if len(r.PropertyDependencies) > 0 {
			b.WriteString("\n\t\t\tPropertyDeps: map[resource.PropertyKey][]resource.URN{")
			for k, deps := range r.PropertyDependencies {
				b.WriteString(fmt.Sprintf("\n\t\t\t\t\"%s\": {", k))
				for _, dep := range deps {
					b.WriteString(fmt.Sprintf("\n\t\t\t\t\t%s.URN,", varFor(dep)))
				}
				b.WriteString("\n\t\t\t\t},")
			}
			b.WriteString("\n\t\t\t},")
		}

		if r.DeletedWith != "" {
			b.WriteString(fmt.Sprintf("\n\t\t\tDeletedWith: %s.URN,", varFor(r.DeletedWith)))
		}

		b.WriteString("\n\t\t})")
		b.WriteString("\n\t\trequire.NoError(t, err)\n")
	}

	b.WriteString("\n\t\treturn nil")
	b.WriteString("\n\t})")

	// Setup execution
	b.WriteString("\n\n\tsetupHostF := deploytest.NewPluginHostF(nil, nil, setupProgramF, setupLoaders...)")
	b.WriteString("\n\tsetupOpts := TestUpdateOptions{")
	b.WriteString("\n\t\tT: t,")
	b.WriteString("\n\t\tHostF: setupHostF,")
	b.WriteString("\n\t}")

	b.WriteString(
		"\n\n\tsetupSnap, err := " +
			"TestOp(engine.Update).RunStep(project, p.GetTarget(t, nil), setupOpts, false, p.BackendClient, nil, \"0\")",
	)
	b.WriteString("\n\trequire.NoError(t, err)")

	// Reproduction loaders
	b.WriteString("\n\n\tcreateF := func(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {")
	if len(provSpec.Create) > 0 {
		b.WriteString("\n\t\tswitch req.URN {")
		for urn := range provSpec.Create {
			b.WriteString(fmt.Sprintf("\n\t\tcase \"%s\":", urn))
			b.WriteString("\n\t\t\treturn plugin.CreateResponse{Status: resource.StatusUnknown}, fmt.Errorf(\"create failure for %s\", req.URN)")
		}
		b.WriteString("\n\t\t}")
	}

	b.WriteString("\n\t\treturn plugin.CreateResponse{Properties: req.Properties, Status: resource.StatusOK}, nil")
	b.WriteString("\n\t}")

	b.WriteString("\n\n\tdeleteF := func(ctx context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {")
	if len(provSpec.Delete) > 0 {
		b.WriteString("\n\t\tswitch req.URN {")
		for urn := range provSpec.Delete {
			b.WriteString(fmt.Sprintf("\n\t\tcase \"%s\":", urn))
			b.WriteString("\n\t\t\treturn plugin.DeleteResponse{Status: resource.StatusUnknown}, fmt.Errorf(\"delete failure for %s\", req.URN)")
		}
		b.WriteString("\n\t\t}")
	}

	b.WriteString("\n\t\treturn plugin.DeleteResponse{Status: resource.StatusOK}, nil")
	b.WriteString("\n\t}")

	b.WriteString("\n\n\tdiffF := func(ctx context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {")
	if len(provSpec.Diff) > 0 {
		b.WriteString("\n\t\tswitch req.URN {")
		for urn, action := range provSpec.Diff {
			b.WriteString(fmt.Sprintf("\n\t\tcase \"%s\":", urn))
			switch action {
			case ProviderDiffDeleteBeforeReplace:
				b.WriteString("\n\t\t\treturn plugin.DiffResponse{")
				b.WriteString("\n\t\t\t\tChanges:             plugin.DiffSome,")
				b.WriteString("\n\t\t\t\tReplaceKeys:         []resource.PropertyKey{\"__replace\"},")
				b.WriteString("\n\t\t\t\tDeleteBeforeReplace: true,")
				b.WriteString("\n\t\t\t}, nil")
			case ProviderDiffDeleteAfterReplace:
				b.WriteString("\n\t\t\treturn plugin.DiffResponse{")
				b.WriteString("\n\t\t\t\tChanges:             plugin.DiffSome,")
				b.WriteString("\n\t\t\t\tReplaceKeys:         []resource.PropertyKey{\"__replace\"},")
				b.WriteString("\n\t\t\t\tDeleteBeforeReplace: false,")
				b.WriteString("\n\t\t\t}, nil")
			case ProviderDiffChange:
				b.WriteString("\n\t\t\treturn plugin.DiffResponse{Changes: plugin.DiffSome}, nil")
			case ProviderDiffFailure:
				b.WriteString("\n\t\t\treturn plugin.DiffResponse{}, fmt.Errorf(\"diff failure for %s\", req.URN)")
			}
		}
		b.WriteString("\n\t\t}")
	}

	b.WriteString("\n\t\treturn plugin.DiffResponse{}, nil")
	b.WriteString("\n\t}")

	b.WriteString("\n\n\treadF := func(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {")
	if len(provSpec.Read) > 0 {
		b.WriteString("\n\t\tswitch req.URN {")
		for urn, action := range provSpec.Read {
			b.WriteString(fmt.Sprintf("\n\t\tcase \"%s\":", urn))
			switch action {
			case ProviderReadDeleted:
				b.WriteString("\n\t\t\treturn plugin.ReadResponse{}, nil")
			case ProviderReadFailure:
				b.WriteString("\n\t\t\treturn plugin.ReadResponse{Status: resource.StatusPartialFailure}, fmt.Errorf(\"read failure for %s\", req.URN)")
			}
		}
		b.WriteString("\n\t\t}")
	}

	b.WriteString("\n\t\treturn plugin.ReadResponse{")
	b.WriteString("\n\t\t\tReadResult: plugin.ReadResult{Outputs: resource.PropertyMap{}},")
	b.WriteString("\n\t\t\tStatus:     resource.StatusOK,")
	b.WriteString("\n\t\t}, nil")
	b.WriteString("\n\t}")

	b.WriteString("\n\n\tupdateF := func(ctx context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {")
	if len(provSpec.Update) > 0 {
		b.WriteString("\n\t\tswitch req.URN {")
		for urn := range provSpec.Update {
			b.WriteString(fmt.Sprintf("\n\t\tcase \"%s\":", urn))
			b.WriteString("\n\t\t\treturn plugin.UpdateResponse{Status: resource.StatusUnknown}, fmt.Errorf(\"update failure for %s\", req.URN)")
		}
		b.WriteString("\n\t\t}")
	}

	b.WriteString("\n\t\treturn plugin.UpdateResponse{")
	b.WriteString("\n\t\t\tProperties: req.NewInputs,")
	b.WriteString("\n\t\t\tStatus:     resource.StatusOK,")
	b.WriteString("\n\t\t}, nil")
	b.WriteString("\n\t}")

	b.WriteString("\n\n\treproLoaders := []*deploytest.ProviderLoader{")

	for _, pkg := range pkgs {
		b.WriteString(fmt.Sprintf(`
		deploytest.NewProviderLoader("%s", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{CreateF: createF, DeleteF: deleteF, DiffF: diffF, ReadF: readF, UpdateF: updateF}, nil
		}),`,
			pkg,
		))
	}

	b.WriteString("\n\t}")

	// Reproduction program (imperfect)
	b.WriteString(
		"\n\n\treproProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {",
	)

	indicesByURN = map[resource.URN]int{}

	for i, r := range progSpec.ResourceRegistrations {
		indicesByURN[r.URN()] = i

		if r.Provider != "" {
			ref, err := providers.ParseReference(r.Provider)
			require.NoError(t, err)

			b.WriteString(fmt.Sprintf("\n\t\tres%dProvRef, err := providers.NewReference(%[2]s.URN, %[2]s.ID)", i, varFor(ref.URN())))
			b.WriteString("\n\t\trequire.NoError(t, err)")
		}

		b.WriteString(fmt.Sprintf(
			"\n\t\tres%d, err := monitor.RegisterResource(\"%s\", \"%s\", %v, deploytest.ResourceOptions{",
			i, r.Type, r.Name, r.Custom,
		))

		if r.Delete {
			b.WriteString("\n\t\t\t// You'll need to set-up a means for Delete: true to be set on this resource")
			b.WriteString("\n\t\t\t// Delete: true,")
		}
		if r.PendingReplacement {
			b.WriteString("\n\t\t\t// You'll need to set-up a means for PendingReplacement: true to be set on this resource")
			b.WriteString("\n\t\t\t// PendingReplacement: true,")
		}

		if r.Protect {
			b.WriteString("\n\t\t\tProtect: true,")
		}
		if r.RetainOnDelete {
			b.WriteString("\n\t\t\tRetainOnDelete: true,")
		}

		if r.Provider != "" {
			b.WriteString(fmt.Sprintf("\n\t\t\tProvider: res%dProvRef.String(),", i))
		}

		if len(r.Dependencies) > 0 {
			b.WriteString("\n\t\t\tDependencies: []resource.URN{")
			for _, dep := range r.Dependencies {
				b.WriteString(fmt.Sprintf("\n\t\t\t\t%s.URN,", varFor(dep)))
			}
			b.WriteString("\n\t\t\t},")
		}

		if len(r.PropertyDependencies) > 0 {
			b.WriteString("\n\t\t\tPropertyDeps: map[resource.PropertyKey][]resource.URN{")
			for k, deps := range r.PropertyDependencies {
				b.WriteString(fmt.Sprintf("\n\t\t\t\t\"%s\": {", k))
				for _, dep := range deps {
					b.WriteString(fmt.Sprintf("\n\t\t\t\t\t%s.URN,", varFor(dep)))
				}
				b.WriteString("\n\t\t\t\t},")
			}
			b.WriteString("\n\t\t\t},")
		}

		if r.DeletedWith != "" {
			b.WriteString(fmt.Sprintf("\n\t\t\tDeletedWith: %s.URN,", varFor(r.DeletedWith)))
		}

		b.WriteString("\n\t\t})")
		b.WriteString("\n\t\trequire.NoError(t, err)\n")
	}

	b.WriteString("\n\t\treturn nil")
	b.WriteString("\n\t})")

	// Reproduction execution
	b.WriteString("\n\n\treproHostF := deploytest.NewPluginHostF(nil, nil, reproProgramF, reproLoaders...)")
	b.WriteString("\n\treproOpts := TestUpdateOptions{")
	b.WriteString("\n\t\tT: t,")
	b.WriteString("\n\t\tHostF: reproHostF,")
	b.WriteString("\n\t\tUpdateOptions: engine.UpdateOptions{")
	if len(planSpec.TargetURNs) > 0 {
		b.WriteString("\n\t\t\tTargets: deploy.NewUrnTargets([]string{")
		for _, urn := range planSpec.TargetURNs {
			b.WriteString(fmt.Sprintf("\n\t\t\t\t\"%s\",", urn))
		}
		b.WriteString("\n\t\t\t}),")
	}
	b.WriteString("\n\t\t},")
	b.WriteString("\n\t}")

	var operation string
	switch planSpec.Operation {
	case PlanOperationUpdate:
		operation = "engine.Update"
	case PlanOperationRefresh:
		operation = "engine.Refresh"
	case PlanOperationDestroy:
		operation = "engine.Destroy"
	}

	b.WriteString(fmt.Sprintf(
		"\n\n\treproSnap, err := "+
			"TestOp(%s).RunStep(project, p.GetTarget(t, setupSnap), reproOpts, false, p.BackendClient, nil, \"1\")",
		operation,
	))
	b.WriteString("\n\trequire.NoError(t, err)")

	b.WriteString("\n}")

	return b.String()
}
