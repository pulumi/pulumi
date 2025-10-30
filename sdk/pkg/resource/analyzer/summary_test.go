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
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
)

func TestNewAnalyzePolicySummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name		string
		urn		resource.URN
		response	plugin.AnalyzeResponse
		info		plugin.AnalyzerInfo
		expected	plugin.PolicySummary
	}{
		{
			name:	"empty response with no policies",
			urn:	"urn:pulumi:test::project::Type::resource",
			response: plugin.AnalyzeResponse{
				Diagnostics:	[]plugin.AnalyzeDiagnostic{},
				NotApplicable:	[]plugin.PolicyNotApplicable{},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies:	[]plugin.AnalyzerPolicyInfo{},
			},
			expected: plugin.PolicySummary{
				URN:			"urn:pulumi:test::project::Type::resource",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{},
				Failed:			[]string{},
			},
		},
		{
			name:	"single passing resource policy",
			urn:	"urn:pulumi:test::project::Type::resource",
			response: plugin.AnalyzeResponse{
				Diagnostics:	[]plugin.AnalyzeDiagnostic{},
				NotApplicable:	[]plugin.PolicyNotApplicable{},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies: []plugin.AnalyzerPolicyInfo{
					{
						Name:			"resource-policy",
						Type:			plugin.AnalyzerPolicyTypeResource,
						EnforcementLevel:	apitype.Advisory,
					},
				},
			},
			expected: plugin.PolicySummary{
				URN:			"urn:pulumi:test::project::Type::resource",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{"resource-policy"},
				Failed:			[]string{},
			},
		},
		{
			name:	"single failing resource policy",
			urn:	"urn:pulumi:test::project::Type::resource",
			response: plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{
					{
						PolicyName:	"resource-policy",
						URN:		"urn:pulumi:test::project::Type::resource",
					},
				},
				NotApplicable:	[]plugin.PolicyNotApplicable{},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies: []plugin.AnalyzerPolicyInfo{
					{
						Name:			"resource-policy",
						Type:			plugin.AnalyzerPolicyTypeResource,
						EnforcementLevel:	apitype.Mandatory,
					},
				},
			},
			expected: plugin.PolicySummary{
				URN:			"urn:pulumi:test::project::Type::resource",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{},
				Failed:			[]string{"resource-policy"},
			},
		},
		{
			name:	"disabled resource policy",
			urn:	"urn:pulumi:test::project::Type::resource",
			response: plugin.AnalyzeResponse{
				Diagnostics:	[]plugin.AnalyzeDiagnostic{},
				NotApplicable:	[]plugin.PolicyNotApplicable{},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies: []plugin.AnalyzerPolicyInfo{
					{
						Name:			"resource-policy",
						Type:			plugin.AnalyzerPolicyTypeResource,
						EnforcementLevel:	apitype.Disabled,
					},
				},
			},
			expected: plugin.PolicySummary{
				URN:			"urn:pulumi:test::project::Type::resource",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{},
				Failed:			[]string{},
			},
		},
		{
			name:	"not applicable resource policy",
			urn:	"urn:pulumi:test::project::Type::resource",
			response: plugin.AnalyzeResponse{
				Diagnostics:	[]plugin.AnalyzeDiagnostic{},
				NotApplicable: []plugin.PolicyNotApplicable{
					{
						PolicyName:	"resource-policy",
						Reason:		"policy not applicable to this resource type",
					},
				},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies: []plugin.AnalyzerPolicyInfo{
					{
						Name:			"resource-policy",
						Type:			plugin.AnalyzerPolicyTypeResource,
						EnforcementLevel:	apitype.Advisory,
					},
				},
			},
			expected: plugin.PolicySummary{
				URN:			"urn:pulumi:test::project::Type::resource",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{},
				Failed:			[]string{},
			},
		},
		{
			name:	"mixed resource and stack policies - only resource policies included",
			urn:	"urn:pulumi:test::project::Type::resource",
			response: plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{
					{
						PolicyName:	"failing-resource-policy",
						URN:		"urn:pulumi:test::project::Type::resource",
					},
				},
				NotApplicable:	[]plugin.PolicyNotApplicable{},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies: []plugin.AnalyzerPolicyInfo{
					{
						Name:			"failing-resource-policy",
						Type:			plugin.AnalyzerPolicyTypeResource,
						EnforcementLevel:	apitype.Mandatory,
					},
					{
						Name:			"passing-resource-policy",
						Type:			plugin.AnalyzerPolicyTypeResource,
						EnforcementLevel:	apitype.Advisory,
					},
					{
						Name:			"stack-policy",
						Type:			plugin.AnalyzerPolicyTypeStack,
						EnforcementLevel:	apitype.Advisory,
					},
				},
			},
			expected: plugin.PolicySummary{
				URN:			"urn:pulumi:test::project::Type::resource",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{"passing-resource-policy"},
				Failed:			[]string{"failing-resource-policy"},
			},
		},
		{
			name:	"unknown type policies treated as applicable",
			urn:	"urn:pulumi:test::project::Type::resource",
			response: plugin.AnalyzeResponse{
				Diagnostics:	[]plugin.AnalyzeDiagnostic{},
				NotApplicable:	[]plugin.PolicyNotApplicable{},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies: []plugin.AnalyzerPolicyInfo{
					{
						Name:			"unknown-type-policy",
						Type:			plugin.AnalyzerPolicyTypeUnknown,
						EnforcementLevel:	apitype.Advisory,
					},
				},
			},
			expected: plugin.PolicySummary{
				URN:			"urn:pulumi:test::project::Type::resource",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{"unknown-type-policy"},
				Failed:			[]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := NewAnalyzePolicySummary(tt.urn, tt.response, tt.info)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewRemediatePolicySummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name		string
		urn		resource.URN
		response	plugin.RemediateResponse
		info		plugin.AnalyzerInfo
		expected	plugin.PolicySummary
	}{
		{
			name:	"empty response with no policies",
			urn:	"urn:pulumi:test::project::Type::resource",
			response: plugin.RemediateResponse{
				Remediations:	[]plugin.Remediation{},
				NotApplicable:	[]plugin.PolicyNotApplicable{},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies:	[]plugin.AnalyzerPolicyInfo{},
			},
			expected: plugin.PolicySummary{
				URN:			"urn:pulumi:test::project::Type::resource",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{},
				Failed:			[]string{},
			},
		},
		{
			name:	"single remediation treated as failure",
			urn:	"urn:pulumi:test::project::Type::resource",
			response: plugin.RemediateResponse{
				Remediations: []plugin.Remediation{
					{
						PolicyName:	"resource-policy",
						URN:		"urn:pulumi:test::project::Type::resource",
					},
				},
				NotApplicable:	[]plugin.PolicyNotApplicable{},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies: []plugin.AnalyzerPolicyInfo{
					{
						Name:			"resource-policy",
						Type:			plugin.AnalyzerPolicyTypeResource,
						EnforcementLevel:	apitype.Advisory,
					},
				},
			},
			expected: plugin.PolicySummary{
				URN:			"urn:pulumi:test::project::Type::resource",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{},
				Failed:			[]string{"resource-policy"},
			},
		},
		{
			name:	"multiple remediations with not applicable",
			urn:	"urn:pulumi:test::project::Type::resource",
			response: plugin.RemediateResponse{
				Remediations: []plugin.Remediation{
					{
						PolicyName:	"policy-a",
						URN:		"urn:pulumi:test::project::Type::resource",
					},
					{
						PolicyName:	"policy-b",
						URN:		"urn:pulumi:test::project::Type::resource",
					},
				},
				NotApplicable: []plugin.PolicyNotApplicable{
					{
						PolicyName:	"policy-c",
						Reason:		"not applicable",
					},
				},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"2.0.0",
				Policies: []plugin.AnalyzerPolicyInfo{
					{
						Name:			"policy-a",
						Type:			plugin.AnalyzerPolicyTypeResource,
						EnforcementLevel:	apitype.Mandatory,
					},
					{
						Name:			"policy-b",
						Type:			plugin.AnalyzerPolicyTypeResource,
						EnforcementLevel:	apitype.Advisory,
					},
					{
						Name:			"policy-c",
						Type:			plugin.AnalyzerPolicyTypeResource,
						EnforcementLevel:	apitype.Advisory,
					},
					{
						Name:			"policy-d",
						Type:			plugin.AnalyzerPolicyTypeResource,
						EnforcementLevel:	apitype.Advisory,
					},
				},
			},
			expected: plugin.PolicySummary{
				URN:			"urn:pulumi:test::project::Type::resource",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"2.0.0",
				Passed:			[]string{"policy-d"},
				Failed:			[]string{"policy-a", "policy-b"},
			},
		},
		{
			name:	"disabled policies excluded from remediation summary",
			urn:	"urn:pulumi:test::project::Type::resource",
			response: plugin.RemediateResponse{
				Remediations: []plugin.Remediation{
					{
						PolicyName:	"active-policy",
						URN:		"urn:pulumi:test::project::Type::resource",
					},
				},
				NotApplicable:	[]plugin.PolicyNotApplicable{},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies: []plugin.AnalyzerPolicyInfo{
					{
						Name:			"active-policy",
						Type:			plugin.AnalyzerPolicyTypeResource,
						EnforcementLevel:	apitype.Advisory,
					},
					{
						Name:			"disabled-policy",
						Type:			plugin.AnalyzerPolicyTypeResource,
						EnforcementLevel:	apitype.Disabled,
					},
				},
			},
			expected: plugin.PolicySummary{
				URN:			"urn:pulumi:test::project::Type::resource",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{},
				Failed:			[]string{"active-policy"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := NewRemediatePolicySummary(tt.urn, tt.response, tt.info)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewAnalyzeStackPolicySummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name		string
		response	plugin.AnalyzeResponse
		info		plugin.AnalyzerInfo
		expected	plugin.PolicySummary
	}{
		{
			name:	"empty response with no policies",
			response: plugin.AnalyzeResponse{
				Diagnostics:	[]plugin.AnalyzeDiagnostic{},
				NotApplicable:	[]plugin.PolicyNotApplicable{},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies:	[]plugin.AnalyzerPolicyInfo{},
			},
			expected: plugin.PolicySummary{
				URN:			"",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{},
				Failed:			[]string{},
			},
		},
		{
			name:	"single passing stack policy",
			response: plugin.AnalyzeResponse{
				Diagnostics:	[]plugin.AnalyzeDiagnostic{},
				NotApplicable:	[]plugin.PolicyNotApplicable{},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies: []plugin.AnalyzerPolicyInfo{
					{
						Name:			"stack-policy",
						Type:			plugin.AnalyzerPolicyTypeStack,
						EnforcementLevel:	apitype.Advisory,
					},
				},
			},
			expected: plugin.PolicySummary{
				URN:			"",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{"stack-policy"},
				Failed:			[]string{},
			},
		},
		{
			name:	"single failing stack policy",
			response: plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{
					{
						PolicyName: "stack-policy",
					},
				},
				NotApplicable:	[]plugin.PolicyNotApplicable{},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies: []plugin.AnalyzerPolicyInfo{
					{
						Name:			"stack-policy",
						Type:			plugin.AnalyzerPolicyTypeStack,
						EnforcementLevel:	apitype.Mandatory,
					},
				},
			},
			expected: plugin.PolicySummary{
				URN:			"",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{},
				Failed:			[]string{"stack-policy"},
			},
		},
		{
			name:	"mixed resource and stack policies - only stack policies included",
			response: plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{
					{
						PolicyName: "failing-stack-policy",
					},
				},
				NotApplicable: []plugin.PolicyNotApplicable{
					{
						PolicyName:	"not-applicable-stack-policy",
						Reason:		"stack condition not met",
					},
				},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies: []plugin.AnalyzerPolicyInfo{
					{
						Name:			"failing-stack-policy",
						Type:			plugin.AnalyzerPolicyTypeStack,
						EnforcementLevel:	apitype.Mandatory,
					},
					{
						Name:			"passing-stack-policy",
						Type:			plugin.AnalyzerPolicyTypeStack,
						EnforcementLevel:	apitype.Advisory,
					},
					{
						Name:			"not-applicable-stack-policy",
						Type:			plugin.AnalyzerPolicyTypeStack,
						EnforcementLevel:	apitype.Advisory,
					},
					{
						Name:			"resource-policy",
						Type:			plugin.AnalyzerPolicyTypeResource,
						EnforcementLevel:	apitype.Advisory,
					},
				},
			},
			expected: plugin.PolicySummary{
				URN:			"",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{"passing-stack-policy"},
				Failed:			[]string{"failing-stack-policy"},
			},
		},
		{
			name:	"disabled stack policy",
			response: plugin.AnalyzeResponse{
				Diagnostics:	[]plugin.AnalyzeDiagnostic{},
				NotApplicable:	[]plugin.PolicyNotApplicable{},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies: []plugin.AnalyzerPolicyInfo{
					{
						Name:			"disabled-stack-policy",
						Type:			plugin.AnalyzerPolicyTypeStack,
						EnforcementLevel:	apitype.Disabled,
					},
					{
						Name:			"active-stack-policy",
						Type:			plugin.AnalyzerPolicyTypeStack,
						EnforcementLevel:	apitype.Advisory,
					},
				},
			},
			expected: plugin.PolicySummary{
				URN:			"",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{"active-stack-policy"},
				Failed:			[]string{},
			},
		},
		{
			name:	"unknown type policies treated as applicable for stack",
			response: plugin.AnalyzeResponse{
				Diagnostics:	[]plugin.AnalyzeDiagnostic{},
				NotApplicable:	[]plugin.PolicyNotApplicable{},
			},
			info: plugin.AnalyzerInfo{
				Name:		"test-pack",
				Version:	"1.0.0",
				Policies: []plugin.AnalyzerPolicyInfo{
					{
						Name:			"unknown-type-policy",
						Type:			plugin.AnalyzerPolicyTypeUnknown,
						EnforcementLevel:	apitype.Advisory,
					},
				},
			},
			expected: plugin.PolicySummary{
				URN:			"",
				PolicyPackName:		"test-pack",
				PolicyPackVersion:	"1.0.0",
				Passed:			[]string{"unknown-type-policy"},
				Failed:			[]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := NewAnalyzeStackPolicySummary(tt.response, tt.info)
			assert.Equal(t, tt.expected, result)
		})
	}
}
