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

package plugin

import (
	"context"
	"io"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// AnalyzerPolicyType indicates the type of a policy.
type AnalyzerPolicyType int

const (
	// Unknown policy type.
	AnalyzerPolicyTypeUnknown AnalyzerPolicyType = iota
	// A policy that validates a resource.
	AnalyzerPolicyTypeResource
	// A policy that validates a stack.
	AnalyzerPolicyTypeStack
)

// Analyzer provides a pluggable interface for performing arbitrary analysis of entire projects/stacks/snapshots, and/or
// individual resources, for arbitrary issues.  These might be style, policy, correctness, security, or performance
// related.  This interface hides the messiness of the underlying machinery, since providers are behind an RPC boundary.
type Analyzer interface {
	// Closer closes any underlying OS resources associated with this provider (like processes, RPC channels, etc).
	io.Closer
	// Name fetches an analyzer's qualified name.
	Name() tokens.QName
	// Analyze analyzes a single resource object, and returns any errors that it finds.
	// Is called before the resource is modified.
	Analyze(r AnalyzerResource) (AnalyzeResponse, error)
	// AnalyzeStack analyzes all resources after a successful preview or update.
	// Is called after all resources have been processed, and all changes applied.
	AnalyzeStack(resources []AnalyzerStackResource) (AnalyzeResponse, error)
	// Remediate is given the opportunity to optionally transform a single resource's properties.
	Remediate(r AnalyzerResource) (RemediateResponse, error)
	// GetAnalyzerInfo returns metadata about the analyzer (e.g., list of policies contained).
	GetAnalyzerInfo() (AnalyzerInfo, error)
	// GetPluginInfo returns this plugin's information.
	GetPluginInfo() (PluginInfo, error)
	// Configure configures the analyzer, passing configuration properties for each policy.
	Configure(policyConfig map[string]AnalyzerPolicyConfig) error
	// Cancel signals the analyzer to gracefully shut down and abort any ongoing analysis operations.
	// Operations aborted in this way will return an error. Since Cancel is advisory and non-blocking,
	// it is up to the host to decide how long to wait after Cancel is called before (e.g.)
	// hard-closing any gRPC connection.
	Cancel(ctx context.Context) error
}

// AnalyzerResource mirrors a resource that is passed to `Analyze`.
type AnalyzerResource struct {
	URN        resource.URN
	Type       tokens.Type
	Name       string
	Properties resource.PropertyMap
	Options    AnalyzerResourceOptions
	Provider   *AnalyzerProviderResource
}

// AnalyzerStackResource mirrors a resource that is passed to `AnalyzeStack`.
type AnalyzerStackResource struct {
	AnalyzerResource
	Parent               resource.URN                            // an optional parent URN for this resource.
	Dependencies         []resource.URN                          // dependencies of this resource object.
	PropertyDependencies map[resource.PropertyKey][]resource.URN // the set of dependencies that affect each property.
}

// AnalyzerResourceOptions mirrors resource options sent to the analyzer.
type AnalyzerResourceOptions struct {
	Protect                 bool                    // true to protect this resource from deletion.
	IgnoreChanges           []string                // a list of property names to ignore during changes.
	DeleteBeforeReplace     *bool                   // true if this resource should be deleted prior to replacement.
	AdditionalSecretOutputs []resource.PropertyKey  // outputs that should always be treated as secrets.
	AliasURNs               []resource.URN          // additional URNs that should be aliased to this resource.
	Aliases                 []resource.Alias        // additional URNs that should be aliased to this resource.
	CustomTimeouts          resource.CustomTimeouts // an optional config object for resource options
	Parent                  resource.URN            // an optional parent URN for this resource.
}

// AnalyzerProviderResource mirrors a resource's provider sent to the analyzer.
type AnalyzerProviderResource struct {
	URN        resource.URN
	Type       tokens.Type
	Name       string
	Properties resource.PropertyMap
}

// AnalyzeDiagnostic indicates that resource analysis failed; it contains the property and reason
// for the failure.
type AnalyzeDiagnostic struct {
	PolicyName        string
	PolicyPackName    string
	PolicyPackVersion string
	Description       string
	Message           string
	EnforcementLevel  apitype.EnforcementLevel
	URN               resource.URN
	Severity          apitype.PolicySeverity
}

// AnalyzeResponse is the response from the Analyze method, containing violations.
type AnalyzeResponse struct {
	// Information about policy violations.
	Diagnostics []AnalyzeDiagnostic
	// Information about policies that were not applicable.
	NotApplicable []PolicyNotApplicable
}

// Remediation indicates that a resource remediation took place, and contains the resulting
// transformed properties and associated metadata.
type Remediation struct {
	PolicyName        string
	Description       string
	PolicyPackName    string
	PolicyPackVersion string
	URN               resource.URN
	Properties        resource.PropertyMap
	Diagnostic        string
}

// RemediateResponse is the response from the Remediate method, containing a sequence of remediations applied, in order.
type RemediateResponse struct {
	// The remediations that were applied.
	Remediations []Remediation
	// Information about policies that were not applicable.
	NotApplicable []PolicyNotApplicable
}

// PolicyNotApplicable describes a policy that was not applicable, including an optional reason why.
type PolicyNotApplicable struct {
	// The name of the policy that was not applicable.
	PolicyName string
	// An optional reason why the policy was not applicable.
	Reason string
}

type PolicySummary struct {
	// The URN of the resource. This will be empty for AnalyzeStack.
	URN resource.URN
	// The name of the policy pack.
	PolicyPackName string
	// The version of the policy pack.
	PolicyPackVersion string
	// The names of policies that passed (i.e. did not produce any violations).
	Passed []string
	// The names of policies that failed (i.e. produced violations).
	Failed []string
}

// AnalyzerInfo provides metadata about a PolicyPack inside an analyzer.
type AnalyzerInfo struct {
	// The name of the policy pack.
	Name string
	// An optional pretty name for the policy pack.
	DisplayName string
	// The version of the policy pack.
	Version string
	// Whether the policy pack supports configuration.
	SupportsConfig bool
	// The policies defined in the policy pack.
	Policies []AnalyzerPolicyInfo
	// The initial configuration for the policy pack.
	InitialConfig map[string]AnalyzerPolicyConfig
	// A brief description of the policy pack. This will override the description in PulumiPolicy.yaml.
	Description string
	// README text about the policy pack.
	Readme string
	// The cloud provider/platform this policy pack is associated with, e.g. AWS, Azure, etc.
	Provider string
	// Tags for this policy pack.
	Tags []string
	// A URL to the repository where the policy pack is defined.
	Repository string
}

// AnalyzerPolicyInfo defines the metadata for an individual Policy within a Policy Pack.
type AnalyzerPolicyInfo struct {
	// Unique URL-safe name for the policy.  This is unique to a specific version
	// of a Policy Pack.
	Name string
	// An optional pretty name for the policy.
	DisplayName string

	// Description is used to provide more context about the purpose of the policy.
	Description      string
	EnforcementLevel apitype.EnforcementLevel

	// Message is the message that will be displayed to end users when they violate
	// this policy.
	Message string

	// ConfigSchema is optional config schema for the policy.
	ConfigSchema *AnalyzerPolicyConfigSchema

	// Type of the policy.
	Type AnalyzerPolicyType
	// Severity is the severity level of the policy.
	Severity apitype.PolicySeverity
	// The compliance framework that this policy belongs to.
	Framework *AnalyzerPolicyComplianceFramework
	// Tags associated with the policy
	Tags []string
	// A description of the steps to take to remediate a policy violation.
	RemediationSteps string
	// An optional URL to more information about the policy.
	URL string
}

// JSONSchema represents a JSON schema.
type JSONSchema map[string]any

// AnalyzerPolicyConfigSchema provides metadata about a policy's configuration.
type AnalyzerPolicyConfigSchema struct {
	// Map of config property names to JSON schema.
	Properties map[string]JSONSchema

	// Required config properties
	Required []string
}

// AnalyzerPolicyConfig is the configuration for a policy.
type AnalyzerPolicyConfig struct {
	// Configured enforcement level for the policy.
	EnforcementLevel apitype.EnforcementLevel
	// Configured properties of the policy.
	Properties map[string]any
}

// AnalyzerPolicyComplianceFramework represents a compliance framework that a policy belongs to.
type AnalyzerPolicyComplianceFramework struct {
	// The compliance framework name.
	Name string
	// The compliance framework version.
	Version string
	// The compliance framework reference.
	Reference string
	// The compliance framework specification.
	Specification string
}
