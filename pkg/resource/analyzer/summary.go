package analyzer

import analyzer "github.com/pulumi/pulumi/sdk/v3/pkg/resource/analyzer"

// NewAnalyzePolicySummary creates a new summary from the Analyze response and policies in the analyzer.
func NewAnalyzePolicySummary(urn resource.URN, response plugin.AnalyzeResponse, info plugin.AnalyzerInfo) plugin.PolicySummary {
	return analyzer.NewAnalyzePolicySummary(urn, response, info)
}

// NewRemediatePolicySummary creates a new summary from the Remediate response and policies in the analyzer.
func NewRemediatePolicySummary(urn resource.URN, response plugin.RemediateResponse, info plugin.AnalyzerInfo) plugin.PolicySummary {
	return analyzer.NewRemediatePolicySummary(urn, response, info)
}

// NewAnalyzeStackPolicySummary creates a new summary from the Analyze response and policies in the analyzer.
func NewAnalyzeStackPolicySummary(response plugin.AnalyzeResponse, info plugin.AnalyzerInfo) plugin.PolicySummary {
	return analyzer.NewAnalyzeStackPolicySummary(response, info)
}

