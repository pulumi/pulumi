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

package plugin

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

type analyzerServer struct {
	pulumirpc.UnsafeAnalyzerServer // opt out of forward compat

	analyzer Analyzer
}

func NewAnalyzerServer(analyzer Analyzer) pulumirpc.AnalyzerServer {
	return &analyzerServer{analyzer: analyzer}
}

func (a *analyzerServer) Analyze(
	ctx context.Context, req *pulumirpc.AnalyzeRequest,
) (*pulumirpc.AnalyzeResponse, error) {
	props, err := UnmarshalProperties(req.GetProperties(), MarshalOptions{
		KeepUnknowns:     true,
		KeepSecrets:      true,
		SkipInternalKeys: true,
	})
	if err != nil {
		return nil, err
	}

	provider, err := convertProvider(req.GetProvider())
	if err != nil {
		return nil, err
	}

	res, err := a.analyzer.Analyze(AnalyzerResource{
		URN:        resource.URN(req.GetUrn()),
		Type:       tokens.Type(req.GetType()),
		Name:       req.GetName(),
		Properties: props,
		Options:    convertResourceOptions(req.GetOptions()),
		Provider:   provider,
	})
	if err != nil {
		return nil, err
	}

	return &pulumirpc.AnalyzeResponse{
		Diagnostics:   marshalAnalyzeDiagnostics(res.Diagnostics),
		NotApplicable: marshalPolicyNotApplicables(res.NotApplicable),
	}, nil
}

func (a *analyzerServer) AnalyzeStack(
	ctx context.Context, req *pulumirpc.AnalyzeStackRequest,
) (*pulumirpc.AnalyzeResponse, error) {
	resources, err := slice.MapError(req.GetResources(),
		func(r *pulumirpc.AnalyzerResource) (AnalyzerStackResource, error) {
			props, err := UnmarshalProperties(r.GetProperties(), MarshalOptions{
				KeepUnknowns:     true,
				KeepSecrets:      true,
				SkipInternalKeys: true,
			})
			if err != nil {
				return AnalyzerStackResource{}, err
			}

			provider, err := convertProvider(r.GetProvider())
			if err != nil {
				return AnalyzerStackResource{}, err
			}

			propertyDeps := make(map[resource.PropertyKey][]resource.URN)
			for k, v := range r.GetPropertyDependencies() {
				deps := slice.Map(v.GetUrns(), func(urn string) resource.URN {
					return resource.URN(urn)
				})
				if len(deps) > 0 {
					propertyDeps[resource.PropertyKey(k)] = deps
				}
			}

			return AnalyzerStackResource{
				AnalyzerResource: AnalyzerResource{
					URN:        resource.URN(r.GetUrn()),
					Type:       tokens.Type(r.GetType()),
					Name:       r.GetName(),
					Properties: props,
					Options:    convertResourceOptions(r.GetOptions()),
					Provider:   provider,
				},
				Parent: resource.URN(r.GetParent()),
				Dependencies: slice.Map(r.GetDependencies(), func(d string) resource.URN {
					return resource.URN(d)
				}),
				PropertyDependencies: propertyDeps,
			}, nil
		})
	if err != nil {
		return nil, err
	}

	res, err := a.analyzer.AnalyzeStack(resources)
	if err != nil {
		return nil, err
	}

	return &pulumirpc.AnalyzeResponse{
		Diagnostics:   marshalAnalyzeDiagnostics(res.Diagnostics),
		NotApplicable: marshalPolicyNotApplicables(res.NotApplicable),
	}, nil
}

func (a *analyzerServer) Remediate(
	ctx context.Context, req *pulumirpc.AnalyzeRequest,
) (*pulumirpc.RemediateResponse, error) {
	props, err := UnmarshalProperties(req.GetProperties(), MarshalOptions{
		KeepUnknowns:     true,
		KeepSecrets:      true,
		SkipInternalKeys: false,
	})
	if err != nil {
		return nil, err
	}

	provider, err := convertProvider(req.GetProvider())
	if err != nil {
		return nil, err
	}

	res, err := a.analyzer.Remediate(AnalyzerResource{
		URN:        resource.URN(req.GetUrn()),
		Type:       tokens.Type(req.GetType()),
		Name:       req.GetName(),
		Properties: props,
		Options:    convertResourceOptions(req.GetOptions()),
		Provider:   provider,
	})
	if err != nil {
		return nil, err
	}

	remediations, err := slice.MapError(res.Remediations, func(r Remediation) (*pulumirpc.Remediation, error) {
		mprops, err := MarshalProperties(r.Properties, MarshalOptions{
			KeepUnknowns:     true,
			KeepSecrets:      true,
			SkipInternalKeys: false,
		})
		if err != nil {
			return nil, err
		}

		return &pulumirpc.Remediation{
			PolicyName:        r.PolicyName,
			PolicyPackName:    r.PolicyPackName,
			PolicyPackVersion: r.PolicyPackVersion,
			Description:       r.Description,
			Properties:        mprops,
			Diagnostic:        r.Diagnostic,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	return &pulumirpc.RemediateResponse{
		Remediations:  remediations,
		NotApplicable: marshalPolicyNotApplicables(res.NotApplicable),
	}, nil
}

func (a *analyzerServer) GetAnalyzerInfo(context.Context, *emptypb.Empty) (*pulumirpc.AnalyzerInfo, error) {
	info, err := a.analyzer.GetAnalyzerInfo()
	if err != nil {
		return nil, err
	}

	policies := slice.Map(info.Policies, func(p AnalyzerPolicyInfo) *pulumirpc.PolicyInfo {
		return &pulumirpc.PolicyInfo{
			Name:             p.Name,
			DisplayName:      p.DisplayName,
			Description:      p.Description,
			Message:          p.Message,
			EnforcementLevel: marshalEnforcementLevel(p.EnforcementLevel),
			ConfigSchema:     marshalConfigSchema(p.ConfigSchema),
			PolicyType:       marshalPolicyType(p.Type),
			Severity:         marshalPolicySeverity(p.Severity),
			Framework:        marshalComplianceFramework(p.Framework),
			Tags:             p.Tags,
			RemediationSteps: p.RemediationSteps,
			Url:              p.URL,
		}
	})

	initialConfig := make(map[string]*pulumirpc.PolicyConfig)
	for k, v := range info.InitialConfig {
		properties, err := structpb.NewStruct(v.Properties)
		contract.AssertNoErrorf(err, "marshaling initial config properties for policy %s", k)
		initialConfig[k] = &pulumirpc.PolicyConfig{
			EnforcementLevel: marshalEnforcementLevel(v.EnforcementLevel),
			Properties:       properties,
		}
	}

	return &pulumirpc.AnalyzerInfo{
		Name:           info.Name,
		DisplayName:    info.DisplayName,
		Version:        info.Version,
		SupportsConfig: info.SupportsConfig,
		Policies:       policies,
		InitialConfig:  initialConfig,
		Description:    info.Description,
		Readme:         info.Readme,
		Provider:       info.Provider,
		Tags:           info.Tags,
		Repository:     info.Repository,
	}, nil
}

func (a *analyzerServer) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	info, err := a.analyzer.GetPluginInfo()
	if err != nil {
		return nil, err
	}
	return &pulumirpc.PluginInfo{Version: info.Version.String()}, nil
}

func (a *analyzerServer) Configure(_ context.Context, req *pulumirpc.ConfigureAnalyzerRequest) (*emptypb.Empty, error) {
	config, err := convertPolicyConfig(req.GetPolicyConfig())
	if err != nil {
		return nil, err
	}

	if err := a.analyzer.Configure(config); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (a *analyzerServer) Handshake(
	context.Context, *pulumirpc.AnalyzerHandshakeRequest,
) (*pulumirpc.AnalyzerHandshakeResponse, error) {
	return &pulumirpc.AnalyzerHandshakeResponse{}, nil
}

func (a *analyzerServer) ConfigureStack(
	context.Context, *pulumirpc.AnalyzerStackConfigureRequest,
) (*pulumirpc.AnalyzerStackConfigureResponse, error) {
	return &pulumirpc.AnalyzerStackConfigureResponse{}, nil
}

func (a *analyzerServer) Cancel(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	if err := a.analyzer.Cancel(ctx); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// marshalPolicyType converts an AnalyzerPolicyType to its pulumirpc equivalent.
func marshalPolicyType(t AnalyzerPolicyType) pulumirpc.PolicyType {
	switch t {
	case AnalyzerPolicyTypeResource:
		return pulumirpc.PolicyType_POLICY_TYPE_RESOURCE
	case AnalyzerPolicyTypeStack:
		return pulumirpc.PolicyType_POLICY_TYPE_STACK
	case AnalyzerPolicyTypeUnknown:
		fallthrough
	default:
		return pulumirpc.PolicyType_POLICY_TYPE_UNKNOWN
	}
}

// marshalPolicySeverity converts an PolicySeverity to its pulumirpc equivalent.
func marshalPolicySeverity(severity apitype.PolicySeverity) pulumirpc.PolicySeverity {
	switch severity {
	case apitype.PolicySeverityLow:
		return pulumirpc.PolicySeverity_POLICY_SEVERITY_LOW
	case apitype.PolicySeverityMedium:
		return pulumirpc.PolicySeverity_POLICY_SEVERITY_MEDIUM
	case apitype.PolicySeverityHigh:
		return pulumirpc.PolicySeverity_POLICY_SEVERITY_HIGH
	case apitype.PolicySeverityCritical:
		return pulumirpc.PolicySeverity_POLICY_SEVERITY_CRITICAL
	case apitype.PolicySeverityUnspecified:
		fallthrough
	default:
		return pulumirpc.PolicySeverity_POLICY_SEVERITY_UNSPECIFIED
	}
}

// marshalConfigSchema converts an AnalyzerPolicyConfigSchema to its pulumirpc equivalent.
func marshalConfigSchema(schema *AnalyzerPolicyConfigSchema) *pulumirpc.PolicyConfigSchema {
	if schema == nil {
		return nil
	}

	props := make(map[string]any)
	for k, v := range schema.Properties {
		props[k] = v
	}

	properties, err := structpb.NewStruct(props)
	contract.AssertNoErrorf(err, "")

	return &pulumirpc.PolicyConfigSchema{
		Properties: properties,
		Required:   schema.Required,
	}
}

// marshalComplianceFramework converts an AnalyzerPolicyComplianceFramework to its pulumirpc equivalent.
func marshalComplianceFramework(f *AnalyzerPolicyComplianceFramework) *pulumirpc.PolicyComplianceFramework {
	if f == nil {
		return nil
	}

	return &pulumirpc.PolicyComplianceFramework{
		Name:          f.Name,
		Version:       f.Version,
		Reference:     f.Reference,
		Specification: f.Specification,
	}
}

// marshalAnalyzeDiagnostics converts a slice of AnalyzeDiagnostic to its pulumirpc equivalent.
func marshalAnalyzeDiagnostics(diags []AnalyzeDiagnostic) []*pulumirpc.AnalyzeDiagnostic {
	return slice.Map(diags, func(d AnalyzeDiagnostic) *pulumirpc.AnalyzeDiagnostic {
		return &pulumirpc.AnalyzeDiagnostic{
			PolicyName:        d.PolicyName,
			PolicyPackName:    d.PolicyPackName,
			PolicyPackVersion: d.PolicyPackVersion,
			Description:       d.Description,
			Message:           d.Message,
			EnforcementLevel:  marshalEnforcementLevel(d.EnforcementLevel),
			Urn:               string(d.URN),
			Severity:          marshalPolicySeverity(d.Severity),
		}
	})
}

// marshalPolicyNotApplicables converts a slice of PolicyNotApplicable to its pulumirpc equivalent.
func marshalPolicyNotApplicables(nas []PolicyNotApplicable) []*pulumirpc.PolicyNotApplicable {
	return slice.Map(nas, func(na PolicyNotApplicable) *pulumirpc.PolicyNotApplicable {
		return &pulumirpc.PolicyNotApplicable{
			PolicyName: na.PolicyName,
			Reason:     na.Reason,
		}
	})
}

// convertProvider converts a pulumirpc.AnalyzerProviderResource to an AnalyzerProviderResource.
func convertProvider(p *pulumirpc.AnalyzerProviderResource) (*AnalyzerProviderResource, error) {
	if p == nil {
		return nil, nil
	}

	props, err := UnmarshalProperties(p.Properties, MarshalOptions{
		KeepUnknowns:     true,
		KeepSecrets:      true,
		SkipInternalKeys: true,
	})
	if err != nil {
		return nil, err
	}

	return &AnalyzerProviderResource{
		URN:        resource.URN(p.Urn),
		Type:       tokens.Type(p.Type),
		Name:       p.Name,
		Properties: props,
	}, nil
}

// convertResourceOptions converts a pulumirpc.AnalyzerResourceOptions to AnalyzerResourceOptions.
func convertResourceOptions(opts *pulumirpc.AnalyzerResourceOptions) AnalyzerResourceOptions {
	if opts == nil {
		return AnalyzerResourceOptions{}
	}

	var deleteBeforeReplace *bool
	if opts.GetDeleteBeforeReplace() {
		b := true
		deleteBeforeReplace = &b
	}

	var customTimeouts resource.CustomTimeouts
	if t := opts.GetCustomTimeouts(); t != nil {
		customTimeouts = resource.CustomTimeouts{
			Create: t.GetCreate(),
			Update: t.GetUpdate(),
			Delete: t.GetDelete(),
		}
	}

	return AnalyzerResourceOptions{
		Protect:             opts.GetProtect(),
		IgnoreChanges:       opts.GetIgnoreChanges(),
		DeleteBeforeReplace: deleteBeforeReplace,
		AdditionalSecretOutputs: slice.Map(opts.GetAdditionalSecretOutputs(), func(urn string) resource.PropertyKey {
			return resource.PropertyKey(urn)
		}),
		AliasURNs: slice.Map(opts.GetAliases(), func(urn string) resource.URN {
			return resource.URN(urn)
		}),
		CustomTimeouts: customTimeouts,
		Parent:         resource.URN(opts.GetParent()),
	}
}
