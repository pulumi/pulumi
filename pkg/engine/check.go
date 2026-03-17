// Copyright 2016-2026, Pulumi Corporation.
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

package engine

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	resourceanalyzer "github.com/pulumi/pulumi/pkg/v3/resource/analyzer"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"golang.org/x/sync/errgroup"
)

// CheckResult contains the results of a policy check.
type CheckResult struct {
	// Passed is true if no mandatory violations were found.
	Passed bool
	// MandatoryViolations is the number of mandatory policy violations.
	MandatoryViolations int
	// AdvisoryViolations is the number of advisory policy violations.
	AdvisoryViolations int
}

// Check runs policy packs against the current stack state without evaluating the program.
// It loads the snapshot's resources and runs both per-resource Analyze() and stack-wide
// AnalyzeStack() against them.
//
// The property semantics match the normal deployment flow:
//   - Analyze() is called with input properties (what policies normally see during up/preview)
//   - AnalyzeStack() is called with output properties (what stack policies normally see)
func Check(
	ctx context.Context,
	u UpdateInfo,
	opts UpdateOptions,
	snap *deploy.Snapshot,
	events chan<- Event,
) (CheckResult, error) {
	emitter, err := makeEventEmitter(events, u)
	if err != nil {
		return CheckResult{}, err
	}
	defer emitter.Close()

	return check(ctx, u, opts, snap, &emitter)
}

func check(
	ctx context.Context,
	u UpdateInfo,
	opts UpdateOptions,
	snap *deploy.Snapshot,
	emitter *eventEmitter,
) (CheckResult, error) {
	// Create a plugin context for loading analyzers.
	diagSink := newEventSink(*emitter, false)
	statusDiag := newEventSink(*emitter, true)

	pctx, err := plugin.NewContext(ctx, diagSink, statusDiag, nil, nil, "",
		nil, true, nil, schema.NewLoaderServerFromHost)
	if err != nil {
		return CheckResult{}, fmt.Errorf("creating plugin context: %w", err)
	}
	defer contract.IgnoreClose(pctx)

	// Build deployment options with policy packs.
	deplOpts := &deploymentOptions{
		UpdateOptions: opts,
		Events:        *emitter,
		Diag:          diagSink,
		StatusDiag:    statusDiag,
		DryRun:        true, // Policy check is read-only.
	}

	// Install required policy packs.
	if err := EnsurePoliciesAreInstalled(ctx, pctx, deplOpts, opts.RequiredPolicies); err != nil {
		return CheckResult{}, fmt.Errorf("installing policy packs: %w", err)
	}

	// Build analyzer options. We set DryRun to true since we're not making changes.
	analyzerOpts := &plugin.PolicyAnalyzerOptions{
		DryRun: true,
	}
	if u.Target != nil {
		analyzerOpts.Stack = u.Target.Name.String()
		analyzerOpts.Organization = u.Target.Organization.String()
		if u.Target.Tags != nil {
			analyzerOpts.Tags = u.Target.Tags
		}
	}
	if u.Project != nil {
		analyzerOpts.Project = u.Project.Name.String()
	}

	// Load and configure policy plugins.
	if err := loadPolicyPlugins(pctx, deplOpts, analyzerOpts); err != nil {
		return CheckResult{}, fmt.Errorf("loading policy plugins: %w", err)
	}

	analyzers := pctx.Host.ListAnalyzers()
	if len(analyzers) == 0 {
		return CheckResult{Passed: true}, nil
	}

	// Handle nil or empty snapshots.
	if snap == nil || len(snap.Resources) == 0 {
		return CheckResult{Passed: true}, nil
	}

	// Build provider map from snapshot resources.
	providerMap := buildProviderMap(snap)

	// Run per-resource Analyze() and stack-wide AnalyzeStack().
	return runPolicyChecks(snap, analyzers, providerMap, emitter)
}

// buildProviderMap creates a lookup from provider URN to resource state for all provider
// resources in the snapshot.
func buildProviderMap(snap *deploy.Snapshot) map[resource.URN]*resource.State {
	m := make(map[resource.URN]*resource.State)
	for _, res := range snap.Resources {
		if providers.IsProviderType(res.Type) {
			m[res.URN] = res
		}
	}
	return m
}

// resolveProvider looks up the provider resource for a given resource state.
func resolveProvider(
	state *resource.State,
	providerMap map[resource.URN]*resource.State,
) *plugin.AnalyzerProviderResource {
	if state.Provider == "" {
		return nil
	}
	ref, err := providers.ParseReference(state.Provider)
	if err != nil {
		// If we can't parse the reference, skip setting the provider. This can happen
		// with malformed state, but we should still attempt to check the resource.
		return nil
	}
	providerState, ok := providerMap[ref.URN()]
	if !ok {
		return nil
	}
	return &plugin.AnalyzerProviderResource{
		URN:        providerState.URN,
		Type:       providerState.Type,
		Name:       providerState.URN.Name(),
		Properties: providerState.Inputs,
	}
}

// buildAnalyzerResource converts a resource state into an AnalyzerResource for Analyze() calls.
// Uses Inputs to match the normal deployment behavior where Analyze sees input properties.
func buildAnalyzerResource(
	state *resource.State,
	providerMap map[resource.URN]*resource.State,
) plugin.AnalyzerResource {
	return plugin.AnalyzerResource{
		URN:        state.URN,
		Type:       state.Type,
		Name:       state.URN.Name(),
		Properties: state.Inputs,
		Options: plugin.AnalyzerResourceOptions{
			Protect:                 state.Protect,
			IgnoreChanges:           state.IgnoreChanges,
			AdditionalSecretOutputs: state.AdditionalSecretOutputs,
			Aliases:                 state.GetAliases(),
			CustomTimeouts:          state.CustomTimeouts,
			Parent:                  state.Parent,
		},
		Provider: resolveProvider(state, providerMap),
	}
}

// buildAnalyzerStackResources converts all non-provider, non-pending-delete resources in a snapshot
// into AnalyzerStackResources for AnalyzeStack() calls. Uses Outputs to match the normal deployment
// behavior where AnalyzeStack sees final output properties.
func buildAnalyzerStackResources(
	snap *deploy.Snapshot,
	providerMap map[resource.URN]*resource.State,
) []plugin.AnalyzerStackResource {
	var resources []plugin.AnalyzerStackResource
	for _, state := range snap.Resources {
		if providers.IsProviderType(state.Type) || state.Delete {
			continue
		}
		res := plugin.AnalyzerStackResource{
			AnalyzerResource: plugin.AnalyzerResource{
				URN:  state.URN,
				Type: state.Type,
				Name: state.URN.Name(),
				// AnalyzeStack uses Outputs (final state), matching the normal deployment behavior.
				Properties: state.Outputs,
				Options: plugin.AnalyzerResourceOptions{
					Protect:                 state.Protect,
					IgnoreChanges:           state.IgnoreChanges,
					AdditionalSecretOutputs: state.AdditionalSecretOutputs,
					Aliases:                 state.GetAliases(),
					CustomTimeouts:          state.CustomTimeouts,
					Parent:                  state.Parent,
				},
				Provider: resolveProvider(state, providerMap),
			},
			Parent:               state.Parent,
			Dependencies:         state.Dependencies,
			PropertyDependencies: state.PropertyDependencies,
		}
		resources = append(resources, res)
	}
	return resources
}

// runPolicyChecks runs both per-resource Analyze() and stack-wide AnalyzeStack() against the snapshot.
func runPolicyChecks(
	snap *deploy.Snapshot,
	analyzers []plugin.Analyzer,
	providerMap map[resource.URN]*resource.State,
	emitter *eventEmitter,
) (CheckResult, error) {
	var mandatoryViolations atomic.Int32
	var advisoryViolations atomic.Int32

	// Phase 1: Per-resource Analyze() calls.
	if err := runPerResourceAnalysis(snap, analyzers, providerMap, emitter,
		&mandatoryViolations, &advisoryViolations); err != nil {
		return CheckResult{}, err
	}

	// Phase 2: Stack-wide AnalyzeStack() calls.
	if err := runStackAnalysis(snap, analyzers, providerMap, emitter,
		&mandatoryViolations, &advisoryViolations); err != nil {
		return CheckResult{}, err
	}

	mandatory := int(mandatoryViolations.Load())
	advisory := int(advisoryViolations.Load())
	return CheckResult{
		Passed:              mandatory == 0,
		MandatoryViolations: mandatory,
		AdvisoryViolations:  advisory,
	}, nil
}

// runPerResourceAnalysis runs Analyze() for each resource against each analyzer.
func runPerResourceAnalysis(
	snap *deploy.Snapshot,
	analyzers []plugin.Analyzer,
	providerMap map[resource.URN]*resource.State,
	emitter *eventEmitter,
	mandatoryViolations, advisoryViolations *atomic.Int32,
) error {
	for _, state := range snap.Resources {
		// Skip provider resources and pending deletes.
		if providers.IsProviderType(state.Type) || state.Delete {
			continue
		}

		r := buildAnalyzerResource(state, providerMap)

		var g errgroup.Group
		if parallelism := env.ParallelAnalyze.Value(); parallelism > 0 {
			g.SetLimit(parallelism)
		}

		for _, analyzer := range analyzers {
			g.Go(func() error {
				info, err := analyzer.GetAnalyzerInfo()
				if err != nil {
					return fmt.Errorf("failed to get analyzer info: %w", err)
				}

				response, err := analyzer.Analyze(r)
				if err != nil {
					return fmt.Errorf("failed to run policy: %w", err)
				}

				for _, d := range response.Diagnostics {
					if d.EnforcementLevel == apitype.Remediate {
						d.EnforcementLevel = apitype.Mandatory
					}
					if d.EnforcementLevel == apitype.Mandatory {
						mandatoryViolations.Add(1)
					}
					if d.EnforcementLevel == apitype.Advisory {
						advisoryViolations.Add(1)
					}
					emitter.policyViolationEvent(state.URN, d)
				}

				summary := resourceanalyzer.NewAnalyzePolicySummary(state.URN, response, info)
				emitter.policyAnalyzeSummaryEvent(summary)
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}
	}
	return nil
}

// runStackAnalysis runs AnalyzeStack() for the entire snapshot against each analyzer.
func runStackAnalysis(
	snap *deploy.Snapshot,
	analyzers []plugin.Analyzer,
	providerMap map[resource.URN]*resource.State,
	emitter *eventEmitter,
	mandatoryViolations, advisoryViolations *atomic.Int32,
) error {
	resources := buildAnalyzerStackResources(snap, providerMap)

	// Build a lookup set for validating URNs in diagnostics.
	urnSet := make(map[resource.URN]struct{}, len(resources))
	for _, r := range resources {
		urnSet[r.URN] = struct{}{}
	}

	var g errgroup.Group
	if parallelism := env.ParallelAnalyze.Value(); parallelism > 0 {
		g.SetLimit(parallelism)
	}

	for _, analyzer := range analyzers {
		g.Go(func() error {
			info, err := analyzer.GetAnalyzerInfo()
			if err != nil {
				return fmt.Errorf("failed to get analyzer info: %w", err)
			}

			response, err := analyzer.AnalyzeStack(resources)
			if err != nil {
				return err
			}

			for _, d := range response.Diagnostics {
				if d.EnforcementLevel == apitype.Remediate {
					// Stack policies cannot be remediated, treat as mandatory.
					d.EnforcementLevel = apitype.Mandatory
				}
				if d.EnforcementLevel == apitype.Mandatory {
					mandatoryViolations.Add(1)
				}
				if d.EnforcementLevel == apitype.Advisory {
					advisoryViolations.Add(1)
				}

				// Use the URN from the diagnostic if it refers to a resource in the stack,
				// otherwise fall back to the first resource URN or an empty URN.
				urn := d.URN
				if urn != "" {
					if _, ok := urnSet[urn]; !ok {
						urn = ""
					}
				}
				if urn == "" && len(resources) > 0 {
					urn = resources[0].URN
				}
				emitter.policyViolationEvent(urn, d)
			}

			summary := resourceanalyzer.NewAnalyzeStackPolicySummary(response, info)
			emitter.policyAnalyzeStackSummaryEvent(summary)
			return nil
		})
	}

	return g.Wait()
}
