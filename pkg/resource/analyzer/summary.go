// Copyright 2016-2025, Pulumi Corporation.
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

package analyzer

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/maputil"
)

// NewAnalyzePolicySummary creates a new summary from the Analyze response and policies in the analyzer.
func NewAnalyzePolicySummary(
	urn resource.URN, response plugin.AnalyzeResponse, info plugin.AnalyzerInfo,
) plugin.PolicySummary {
	return newPolicySummary(plugin.AnalyzerPolicyTypeResource, urn, response, info)
}

// NewRemediatePolicySummary creates a new summary from the Remediate response and policies in the analyzer.
func NewRemediatePolicySummary(
	urn resource.URN, response plugin.RemediateResponse, info plugin.AnalyzerInfo,
) plugin.PolicySummary {
	// Convert the RemediateResponse into an AnalyzerResponse, to reuse the same summary calculation below.
	// Any remediations are treated as failures.
	diagnostics := slice.Map(response.Remediations, func(r plugin.Remediation) plugin.AnalyzeDiagnostic {
		return plugin.AnalyzeDiagnostic{
			URN:        r.URN,
			PolicyName: r.PolicyName,
		}
	})
	analyzerResponse := plugin.AnalyzeResponse{
		Diagnostics:   diagnostics,
		NotApplicable: response.NotApplicable,
	}
	return newPolicySummary(plugin.AnalyzerPolicyTypeResource, urn, analyzerResponse, info)
}

// NewAnalyzeStackPolicySummary creates a new summary from the Analyze response and policies in the analyzer.
func NewAnalyzeStackPolicySummary(
	response plugin.AnalyzeResponse, info plugin.AnalyzerInfo,
) plugin.PolicySummary {
	return newPolicySummary(plugin.AnalyzerPolicyTypeStack, "", response, info)
}

// newPolicySummary creates a new summary from the Analyze response and policies in the analyzer.
func newPolicySummary(
	typ plugin.AnalyzerPolicyType,
	urn resource.URN,
	response plugin.AnalyzeResponse,
	info plugin.AnalyzerInfo,
) plugin.PolicySummary {
	failed := map[string]struct{}{}
	for _, d := range response.Diagnostics {
		failed[d.PolicyName] = struct{}{}
	}

	notApplicable := map[string]string{}
	for _, na := range response.NotApplicable {
		notApplicable[na.PolicyName] = na.Reason
	}

	disabled := map[string]struct{}{}
	// Passed = All (of a given type) minus Disabled minus Not Applicable minus Failed.
	passed := map[string]struct{}{}
	for _, p := range info.Policies {
		// An older policy SDK that doesn't support reporting the policy type will have
		// an unknown type. In that case, we'll include the unknown typed policy among
		// the policies we're summarizing.
		if p.Type != typ && p.Type != plugin.AnalyzerPolicyTypeUnknown {
			continue
		}

		if p.EnforcementLevel == apitype.Disabled {
			disabled[p.Name] = struct{}{}
			continue
		}

		_, isFailed := failed[p.Name]
		_, isNotApplicable := notApplicable[p.Name]
		if !isFailed && !isNotApplicable {
			passed[p.Name] = struct{}{}
		}
	}

	notApplicableResult := make([]plugin.PolicyNotApplicable, len(notApplicable))
	for i, name := range maputil.SortedKeys(notApplicable) {
		notApplicableResult[i] = plugin.PolicyNotApplicable{
			PolicyName: name,
			Reason:     notApplicable[name],
		}
	}

	return plugin.PolicySummary{
		URN:               urn,
		PolicyPackName:    info.Name,
		PolicyPackVersion: info.Version,
		Disabled:          maputil.SortedKeys(disabled),
		NotApplicable:     notApplicableResult,
		Passed:            maputil.SortedKeys(passed),
		Failed:            maputil.SortedKeys(failed),
	}
}
