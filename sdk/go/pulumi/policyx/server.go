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

package policyx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	pbempty "google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func convertAnalyzerProvider(provider *plugin.AnalyzerProviderResource) AnalyzerProviderResource {
	if provider == nil {
		return AnalyzerProviderResource{}
	}
	return AnalyzerProviderResource{
		Type:       string(provider.Type),
		Properties: resource.FromResourcePropertyMap(provider.Properties),
		URN:        string(provider.URN),
		Name:       provider.Name,
	}
}

func convertProtoAnalyzerProvider(
	provider *pulumirpc.AnalyzerProviderResource,
	label string,
) (*plugin.AnalyzerProviderResource, error) {
	if provider == nil {
		return nil, nil
	}

	properties, err := plugin.UnmarshalProperties(provider.GetProperties(), plugin.MarshalOptions{
		Label:            label,
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepResources:    true,
		KeepOutputValues: true,
	})
	if err != nil {
		return nil, err
	}

	return &plugin.AnalyzerProviderResource{
		Type:       tokens.Type(provider.GetType()),
		Properties: properties,
		URN:        resource.URN(provider.GetUrn()),
		Name:       provider.GetName(),
	}, nil
}

func formatTimeout(seconds float64) string {
	if seconds == 0 {
		return ""
	}
	return time.Duration(seconds * float64(time.Second)).String()
}

func convertProtoAnalyzerResourceOptions(options *pulumirpc.AnalyzerResourceOptions) plugin.AnalyzerResourceOptions {
	if options == nil {
		return plugin.AnalyzerResourceOptions{}
	}

	var deleteBeforeReplace *bool
	if options.GetDeleteBeforeReplaceDefined() {
		value := options.GetDeleteBeforeReplace()
		deleteBeforeReplace = &value
	}

	var customTimeouts resource.CustomTimeouts
	if timeouts := options.GetCustomTimeouts(); timeouts != nil {
		customTimeouts = resource.CustomTimeouts{
			Create: timeouts.GetCreate(),
			Update: timeouts.GetUpdate(),
			Delete: timeouts.GetDelete(),
			Read:   timeouts.GetRead(),
		}
	}

	additionalSecretOutputs := make([]resource.PropertyKey, len(options.GetAdditionalSecretOutputs()))
	for i, output := range options.GetAdditionalSecretOutputs() {
		additionalSecretOutputs[i] = resource.PropertyKey(output)
	}

	aliasURNs := make([]resource.URN, len(options.GetAliases()))
	for i, alias := range options.GetAliases() {
		aliasURNs[i] = resource.URN(alias)
	}

	return plugin.AnalyzerResourceOptions{
		Protect:                 options.GetProtect(),
		IgnoreChanges:           options.GetIgnoreChanges(),
		DeleteBeforeReplace:     deleteBeforeReplace,
		AdditionalSecretOutputs: additionalSecretOutputs,
		AliasURNs:               aliasURNs,
		CustomTimeouts:          customTimeouts,
		Parent:                  resource.URN(options.GetParent()),
	}
}

func convertProtoPropertyDependencies(
	dependencies map[string]*pulumirpc.AnalyzerPropertyDependencies,
) map[string][]string {
	propertyDependencies := make(map[string][]string)
	for key, deps := range dependencies {
		if len(deps.GetUrns()) > 0 {
			propertyDependencies[key] = deps.GetUrns()
		}
	}
	return propertyDependencies
}

func convertAnalyzerResourceOptions(options plugin.AnalyzerResourceOptions) pulumi.ResourceOptions {
	var customTimeouts *pulumi.CustomTimeouts
	if options.CustomTimeouts.IsNotEmpty() {
		customTimeouts = &pulumi.CustomTimeouts{
			Create: formatTimeout(options.CustomTimeouts.Create),
			Update: formatTimeout(options.CustomTimeouts.Update),
			Delete: formatTimeout(options.CustomTimeouts.Delete),
			Read:   formatTimeout(options.CustomTimeouts.Read),
		}
	}

	aliases := make([]pulumi.Alias, 0, len(options.Aliases)+len(options.AliasURNs))
	for _, alias := range options.Aliases {
		aliases = append(aliases, pulumi.Alias{URN: pulumi.URN(alias.GetURN())})
	}
	for _, alias := range options.AliasURNs {
		aliases = append(aliases, pulumi.Alias{URN: pulumi.URN(alias)})
	}

	additionalSecretOutputs := make([]string, len(options.AdditionalSecretOutputs))
	for i, output := range options.AdditionalSecretOutputs {
		additionalSecretOutputs[i] = string(output)
	}

	return pulumi.ResourceOptions{
		AdditionalSecretOutputs: additionalSecretOutputs,
		Aliases:                 aliases,
		CustomTimeouts:          customTimeouts,
		DeleteBeforeReplace:     options.DeleteBeforeReplace != nil && *options.DeleteBeforeReplace,
		IgnoreChanges:           options.IgnoreChanges,
		Protect:                 options.Protect,
	}
}

func convertAnalyzerResource(r plugin.AnalyzerResource) AnalyzerResource {
	return AnalyzerResource{
		Type:       string(r.Type),
		Properties: resource.FromResourcePropertyMap(r.Properties),
		URN:        string(r.URN),
		Name:       r.Name,
		Options:    convertAnalyzerResourceOptions(r.Options),
		Provider:   convertAnalyzerProvider(r.Provider),
		Parent:     string(r.Options.Parent),
	}
}

// Main starts the analyzer server with the provided policy pack factory function.
func Main(policyPack func(*pulumi.Context) (PolicyPack, error)) error {
	// Fire up a gRPC server, letting the kernel choose a free port for us.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			analyzer := &analyzerServer{
				policyPackFactory: policyPack,
			}
			pulumirpc.RegisterAnalyzerServer(srv, analyzer)
			return nil
		},
	})
	if err != nil {
		return fmt.Errorf("fatal: %v", err)
	}

	// The analyzer protocol requires that we now write out the port we have chosen to listen on.
	fmt.Printf("%d\n", handle.Port)

	// Finally, wait for the server to stop serving.
	if err := <-handle.Done; err != nil {
		return fmt.Errorf("fatal: %v", err)
	}

	return nil
}

type analyzerServer struct {
	pulumirpc.UnimplementedAnalyzerServer

	policyPackFactory func(*pulumi.Context) (PolicyPack, error)
	policyPack        PolicyPack

	stacktags map[string]string
	config    map[string]PolicyConfig
	handshake *pulumirpc.AnalyzerHandshakeRequest
}

func (srv *analyzerServer) Handshake(
	ctx context.Context,
	req *pulumirpc.AnalyzerHandshakeRequest,
) (*pulumirpc.AnalyzerHandshakeResponse, error) {
	srv.handshake = req
	return &pulumirpc.AnalyzerHandshakeResponse{}, nil
}

func (srv *analyzerServer) GetPluginInfo(context.Context, *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: srv.policyPack.Version().String(),
	}, nil
}

func (srv *analyzerServer) GetAnalyzerInfo(context.Context, *pbempty.Empty) (*pulumirpc.AnalyzerInfo, error) {
	policies := make([]*pulumirpc.PolicyInfo, 0, len(srv.policyPack.Policies()))
	for _, p := range srv.policyPack.Policies() {
		schema := p.ConfigSchema()
		var configSchema *pulumirpc.PolicyConfigSchema
		if schema != nil {
			// Convert the schema properties to a map[string]any for protobuf serialization.
			m := make(map[string]any, len(schema.Properties))
			for k, v := range schema.Properties {
				m[k] = v
			}
			proto, err := structpb.NewStruct(m)
			if err != nil {
				return nil, fmt.Errorf("failed to convert schema properties to protobuf: %w", err)
			}

			configSchema = &pulumirpc.PolicyConfigSchema{
				Properties: proto,
				Required:   schema.Required,
			}
		}

		policies = append(policies, &pulumirpc.PolicyInfo{
			Name:             p.Name(),
			Description:      p.Description(),
			EnforcementLevel: pulumirpc.EnforcementLevel(p.EnforcementLevel()),
			ConfigSchema:     configSchema,
		})
	}
	return &pulumirpc.AnalyzerInfo{
		Name:           srv.policyPack.Name(),
		Version:        srv.policyPack.Version().String(),
		Policies:       policies,
		SupportsConfig: true,
		InitialConfig:  nil, /* TODO */
	}, nil
}

func (srv *analyzerServer) ConfigureStack(ctx context.Context,
	req *pulumirpc.AnalyzerStackConfigureRequest) (
	*pulumirpc.AnalyzerStackConfigureResponse, error,
) {
	if srv.handshake == nil {
		return nil, errors.New("analyzer has not had handshake called")
	}

	root := ""
	if srv.handshake.RootDirectory != nil {
		root = *srv.handshake.RootDirectory
	}

	info := pulumi.RunInfo{
		Stack:        req.Stack,
		Project:      req.Project,
		Organization: req.Organization,

		RootDirectory: root,

		MonitorAddr: "",
		EngineAddr:  srv.handshake.EngineAddress,

		Config:           req.Config,
		ConfigSecretKeys: req.ConfigSecretKeys,
		DryRun:           req.DryRun,
		Parallel:         1,
	}
	pctx, err := pulumi.NewContext(context.Background(), info)
	if err != nil {
		return nil, fmt.Errorf("creating context: %w", err)
	}

	srv.stacktags = req.Tags

	srv.policyPack, err = srv.policyPackFactory(pctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy pack: %w", err)
	}

	return &pulumirpc.AnalyzerStackConfigureResponse{}, nil
}

func (srv *analyzerServer) Configure(ctx context.Context, req *pulumirpc.ConfigureAnalyzerRequest) (*pbempty.Empty,
	error,
) {
	conf := map[string]PolicyConfig{}
	for k, v := range req.PolicyConfig {
		data, err := v.GetProperties().MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal properties for policy %q: %w", k, err)
		}
		// Unmarshal the properties into an map[string]any
		var props map[string]any
		if err := json.Unmarshal(data, &props); err != nil {
			return nil, fmt.Errorf("failed to unmarshal properties for policy %q: %w", k, err)
		}
		conf[k] = PolicyConfig{
			EnforcementLevel: EnforcementLevel(v.EnforcementLevel),
			Properties:       props,
		}
	}

	srv.config = conf

	return &pbempty.Empty{}, nil
}

func (srv *analyzerServer) Analyze(
	ctx context.Context, req *pulumirpc.AnalyzeRequest,
) (*pulumirpc.AnalyzeResponse, error) {
	var ds []*pulumirpc.AnalyzeDiagnostic
	policyManager := &policyManager{}

	for _, p := range srv.policyPack.Policies() {
		switch p := p.(type) {
		case ResourceValidationPolicy:
			config, hasConfig := srv.config[p.Name()]

			enforcementLevel := p.EnforcementLevel()
			if hasConfig {
				enforcementLevel = config.EnforcementLevel
			}

			if enforcementLevel != EnforcementLevelDisabled {
				policyManager.reportViolation = func(message string, urn string) {
					if urn == "" {
						urn = req.GetUrn()
					}

					violationMessage := p.Description()
					if message != "" {
						violationMessage += "\n" + message
					}

					ds = append(ds, &pulumirpc.AnalyzeDiagnostic{
						PolicyName:        p.Name(),
						PolicyPackName:    srv.policyPack.Name(),
						PolicyPackVersion: srv.policyPack.Version().String(),
						Description:       p.Description(),
						Message:           violationMessage,
						EnforcementLevel:  pulumirpc.EnforcementLevel(enforcementLevel),
						Urn:               urn,
					})
				}

				pm, err := plugin.UnmarshalProperties(req.GetProperties(), plugin.MarshalOptions{
					Label:            fmt.Sprintf("%s.%s.analyze", srv.policyPack.Name(), p.Name()),
					KeepUnknowns:     true,
					KeepSecrets:      true,
					KeepResources:    true,
					KeepOutputValues: true,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to unmarshal properties for policy %q: %w", p.Name(), err)
				}

				provider, err := convertProtoAnalyzerProvider(req.GetProvider(),
					fmt.Sprintf("%s.%s.analyze.provider", srv.policyPack.Name(), p.Name()))
				if err != nil {
					return nil, fmt.Errorf("failed to unmarshal provider for policy %q: %w", p.Name(), err)
				}

				args := ResourceValidationArgs{
					Manager:   policyManager,
					Config:    config.Properties,
					StackTags: srv.stacktags,
					Resource: convertAnalyzerResource(plugin.AnalyzerResource{
						Type:       tokens.Type(req.GetType()),
						Properties: pm,
						URN:        resource.URN(req.GetUrn()),
						Name:       req.GetName(),
						Options:    convertProtoAnalyzerResourceOptions(req.GetOptions()),
						Provider:   provider,
					}),
				}

				err = p.Validate(ctx, args)
				if err != nil {
					return nil, fmt.Errorf("failed to validate resource %q with policy %q: %w", req.GetUrn(), p.Name(), err)
				}
			}
		}
	}

	return &pulumirpc.AnalyzeResponse{
		Diagnostics: ds,
	}, nil
}

func (srv *analyzerServer) Remediate(
	ctx context.Context, req *pulumirpc.AnalyzeRequest,
) (*pulumirpc.RemediateResponse, error) {
	var rs []*pulumirpc.Remediation

	pm, err := plugin.UnmarshalProperties(req.GetProperties(), plugin.MarshalOptions{
		Label:            srv.policyPack.Name() + ".remediate",
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepResources:    true,
		KeepOutputValues: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal properties for policy pack %q: %w", srv.policyPack.Name(), err)
	}
	props := resource.FromResourcePropertyMap(pm)

	for _, p := range srv.policyPack.Policies() {
		switch p := p.(type) {
		case ResourceRemediationPolicy:
			config, hasConfig := srv.config[p.Name()]

			disabled := false
			if hasConfig {
				disabled = config.EnforcementLevel == EnforcementLevelDisabled
			}

			if !disabled {
				provider, err := convertProtoAnalyzerProvider(req.GetProvider(),
					fmt.Sprintf("%s.%s.remediate.provider", srv.policyPack.Name(), p.Name()))
				if err != nil {
					return nil, fmt.Errorf("failed to unmarshal provider for policy %q: %w", p.Name(), err)
				}

				args := ResourceRemediationArgs{
					Resource: convertAnalyzerResource(plugin.AnalyzerResource{
						Type:       tokens.Type(req.GetType()),
						Properties: resource.ToResourcePropertyMap(props),
						URN:        resource.URN(req.GetUrn()),
						Name:       req.GetName(),
						Options:    convertProtoAnalyzerResourceOptions(req.GetOptions()),
						Provider:   provider,
					}),
					Config: config.Properties,
				}

				newProps, err := p.Remediate(ctx, args)
				if err != nil {
					return nil, fmt.Errorf("failed to remediate resource %q with policy %q: %w", req.GetUrn(), p.Name(), err)
				}

				if newProps != nil {
					props = *newProps
					pm = resource.ToResourcePropertyMap(props)
					rpcProps, err := plugin.MarshalProperties(pm, plugin.MarshalOptions{
						Label:            srv.policyPack.Name() + ".remediate",
						KeepUnknowns:     true,
						KeepSecrets:      true,
						KeepResources:    true,
						KeepOutputValues: true,
					})
					if err != nil {
						return nil, fmt.Errorf("failed to marshal properties for policy pack %q: %w", srv.policyPack.Name(), err)
					}

					rs = append(rs, &pulumirpc.Remediation{
						PolicyName:        p.Name(),
						Description:       p.Description(),
						PolicyPackName:    srv.policyPack.Name(),
						PolicyPackVersion: srv.policyPack.Version().String(),
						Properties:        rpcProps,
					})
				}
			}
		}
	}

	return &pulumirpc.RemediateResponse{
		Remediations: rs,
	}, nil
}

func (srv *analyzerServer) AnalyzeStack(ctx context.Context, req *pulumirpc.AnalyzeStackRequest) (*pulumirpc.
	AnalyzeResponse,
	error,
) {
	var ds []*pulumirpc.AnalyzeDiagnostic
	policyManager := &policyManager{}

	resources := make([]AnalyzerResource, 0, len(req.GetResources()))
	for _, r := range req.GetResources() {
		pm, err := plugin.UnmarshalProperties(r.GetProperties(), plugin.MarshalOptions{
			Label:            srv.policyPack.Name() + ".analyzeStack",
			KeepUnknowns:     true,
			KeepSecrets:      true,
			KeepResources:    true,
			KeepOutputValues: true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal properties for resource %q: %w", r.GetUrn(), err)
		}

		provider, err := convertProtoAnalyzerProvider(r.GetProvider(), srv.policyPack.Name()+".analyzeStack.provider")
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal provider for resource %q: %w", r.GetUrn(), err)
		}

		analyzerResource := convertAnalyzerResource(plugin.AnalyzerResource{
			Type:       tokens.Type(r.GetType()),
			Properties: pm,
			URN:        resource.URN(r.GetUrn()),
			Name:       r.GetName(),
			Options:    convertProtoAnalyzerResourceOptions(r.GetOptions()),
			Provider:   provider,
		})
		analyzerResource.Parent = r.GetParent()
		analyzerResource.Dependencies = r.GetDependencies()
		analyzerResource.PropertyDependencies = convertProtoPropertyDependencies(r.GetPropertyDependencies())
		resources = append(resources, analyzerResource)
	}

	for _, p := range srv.policyPack.Policies() {
		p, ok := p.(StackValidationPolicy)
		if !ok {
			continue
		}
		config, hasConfig := srv.config[p.Name()]

		enforcementLevel := p.EnforcementLevel()
		if hasConfig {
			enforcementLevel = config.EnforcementLevel
		}

		if enforcementLevel == EnforcementLevelDisabled {
			continue
		}

		policyManager.reportViolation = func(message string, urn string) {
			violationMessage := p.Description()
			if message != "" {
				violationMessage += "\n" + message
			}

			ds = append(ds, &pulumirpc.AnalyzeDiagnostic{
				PolicyName:        p.Name(),
				PolicyPackName:    srv.policyPack.Name(),
				PolicyPackVersion: srv.policyPack.Version().String(),
				Description:       p.Description(),
				Message:           violationMessage,
				EnforcementLevel:  pulumirpc.EnforcementLevel(enforcementLevel),
				Urn:               urn,
			})
		}

		args := StackValidationArgs{
			Manager:   policyManager,
			Resources: resources,
		}

		if err := p.Validate(ctx, args); err != nil {
			return nil, fmt.Errorf("failed to validate stack with policy %q: %w", p.Name(), err)
		}
	}

	return &pulumirpc.AnalyzeResponse{Diagnostics: ds}, nil
}

func (srv *analyzerServer) Cancel(ctx context.Context, req *pbempty.Empty) (*pbempty.Empty, error) {
	return &pbempty.Empty{}, nil
}
