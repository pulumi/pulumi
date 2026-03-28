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

package deploy

import (
	"context"
	"fmt"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	resourceanalyzer "github.com/pulumi/pulumi/pkg/v3/resource/analyzer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// analyzeResource runs all analyzers against a single resource in parallel, emitting
// policy violation and summary events as results arrive. It respects the
// PULUMI_PARALLEL_ANALYZE environment variable to limit concurrency.
//
// Returns (invalid, sawError) where invalid is true if any mandatory violation was found
// (unless dryRun is true), and sawError is true if any mandatory violation was found.
func analyzeResource(
	analyzers []plugin.Analyzer,
	r plugin.AnalyzerResource,
	events PolicyEvents,
	dryRun bool,
) (invalid bool, sawError bool, err error) {
	if len(analyzers) == 0 {
		return false, false, nil
	}

	var invalidAtomic atomic.Bool
	var sawErrorAtomic atomic.Bool

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
					// If we ran a remediation but are still triggering a violation,
					// "downgrade" the level from remediate to mandatory.
					d.EnforcementLevel = apitype.Mandatory
				}

				if d.EnforcementLevel == apitype.Mandatory {
					if !dryRun {
						invalidAtomic.Store(true)
					}
					sawErrorAtomic.Store(true)
				}
				events.OnPolicyViolation(r.URN, d)
			}

			summary := resourceanalyzer.NewAnalyzePolicySummary(r.URN, response, info)
			events.OnPolicyAnalyzeSummary(summary)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return false, false, err
	}

	return invalidAtomic.Load(), sawErrorAtomic.Load(), nil
}

// buildSnapshotProviderMap builds a map of provider URN -> provider state from a snapshot.
func buildSnapshotProviderMap(snap *Snapshot) map[resource.URN]*resource.State {
	providers := make(map[resource.URN]*resource.State)
	for _, res := range snap.Resources {
		if sdkproviders.IsProviderType(res.Type) {
			providers[res.URN] = res
		}
	}
	return providers
}

// stateToAnalyzerResource converts a resource.State to a plugin.AnalyzerResource.
// properties must be supplied explicitly: use res.Inputs for per-resource Analyze calls
// (matching what Analyze receives during a deployment) and res.Outputs for AnalyzeStack
// calls (to verify the final deployed state).
func stateToAnalyzerResource(
	res *resource.State,
	properties resource.PropertyMap,
	providers map[resource.URN]*resource.State,
) plugin.AnalyzerResource {
	r := plugin.AnalyzerResource{
		URN:        res.URN,
		Type:       res.Type,
		Name:       res.URN.Name(),
		Properties: properties,
		Options: plugin.AnalyzerResourceOptions{
			Protect:                 res.Protect,
			IgnoreChanges:           res.IgnoreChanges,
			AdditionalSecretOutputs: res.AdditionalSecretOutputs,
			Aliases:                 res.GetAliases(),
			CustomTimeouts:          res.CustomTimeouts,
			Parent:                  res.Parent,
		},
	}
	if res.Provider != "" {
		if ref, err := sdkproviders.ParseReference(res.Provider); err == nil {
			if provRes, ok := providers[ref.URN()]; ok {
				r.Provider = &plugin.AnalyzerProviderResource{
					URN:        provRes.URN,
					Type:       provRes.Type,
					Name:       provRes.URN.Name(),
					Properties: provRes.Inputs,
				}
			}
		}
	}
	return r
}

// AnalyzeSnapshot runs policy analysis against an existing stack snapshot without
// executing a Pulumi program or making provider calls. It:
//   - Runs per-resource remediations and reports what would be changed (without applying)
//   - Runs per-resource analysis policies
//   - Runs stack-level analysis policies
//
// Returns true if any mandatory policy violations were found.
func AnalyzeSnapshot(
	ctx context.Context,
	snap *Snapshot,
	analyzers []plugin.Analyzer,
	events PolicyEvents,
) (bool, error) {
	if snap == nil || len(analyzers) == 0 {
		return false, nil
	}

	providers := buildSnapshotProviderMap(snap)

	var hasMandatoryViolation bool

	// Per-resource analysis: remediations then analysis.
	for _, res := range snap.Resources {
		if res.Delete {
			continue
		}

		analyzerRes := stateToAnalyzerResource(res, res.Inputs, providers)

		// First pass: report remediations for this resource. Unlike during a deployment,
		// we report what would be changed but do not apply the changes.
		for _, analyzer := range analyzers {
			info, err := analyzer.GetAnalyzerInfo()
			if err != nil {
				return false, fmt.Errorf("failed to get analyzer info: %w", err)
			}

			response, err := analyzer.Remediate(analyzerRes)
			if err != nil {
				return false, fmt.Errorf("failed to run remediation: %w", err)
			}

			for _, tresult := range response.Remediations {
				if tresult.Diagnostic != "" {
					events.OnPolicyViolation(res.URN, plugin.AnalyzeDiagnostic{
						PolicyName:        tresult.PolicyName,
						PolicyPackName:    tresult.PolicyPackName,
						PolicyPackVersion: tresult.PolicyPackVersion,
						Description:       tresult.Description,
						Message:           tresult.Diagnostic,
						EnforcementLevel:  apitype.Advisory,
						URN:               res.URN,
					})
				} else if tresult.Properties != nil {
					// Report what would be remediated without applying it.
					events.OnPolicyRemediation(res.URN, tresult, analyzerRes.Properties, tresult.Properties)
				}
				// A remediation might not set diagnostic or properties, we just ignore them.
			}
			summary := resourceanalyzer.NewRemediatePolicySummary(res.URN, response, info)
			events.OnPolicyRemediateSummary(summary)
		}

		// Second pass: run analysis in parallel. We use dryRun=true because we're not
		// executing a deployment; violations are reported but do not fail the resource.
		_, sawError, err := analyzeResource(analyzers, analyzerRes, events, true /*dryRun*/)
		if err != nil {
			return false, err
		}
		if sawError {
			hasMandatoryViolation = true
		}
	}

	// Stack-level analysis: collect all non-deleted resources and run AnalyzeStack.
	// AnalyzeStack receives Outputs (final deployed state) rather than Inputs.
	stackResources := make([]plugin.AnalyzerStackResource, 0, len(snap.Resources))
	for _, res := range snap.Resources {
		if res.Delete {
			continue
		}
		analyzerRes := stateToAnalyzerResource(res, res.Outputs, providers)
		stackResources = append(stackResources, plugin.AnalyzerStackResource{
			AnalyzerResource:     analyzerRes,
			Parent:               res.Parent,
			Dependencies:         res.Dependencies,
			PropertyDependencies: res.PropertyDependencies,
		})
	}

	var sawStackError atomic.Bool
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

			response, err := analyzer.AnalyzeStack(stackResources)
			if err != nil {
				return fmt.Errorf("failed to run stack policy: %w", err)
			}

			for _, d := range response.Diagnostics {
				if d.EnforcementLevel == apitype.Remediate {
					// Stack policies cannot be remediated, so treat the level as mandatory.
					d.EnforcementLevel = apitype.Mandatory
				}
				if d.EnforcementLevel == apitype.Mandatory {
					sawStackError.Store(true)
				}
				events.OnPolicyViolation(d.URN, d)
			}

			summary := resourceanalyzer.NewAnalyzeStackPolicySummary(response, info)
			events.OnPolicyAnalyzeStackSummary(summary)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return false, err
	}

	return hasMandatoryViolation || sawStackError.Load(), nil
}
