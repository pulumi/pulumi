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
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumix"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	pbempty "google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// Main starts the analyzer server with the provided policy pack factory function.
func Main(policyPack func(pulumix.Engine) (PolicyPack, error)) error {
	// Fire up a gRPC server, letting the kernel choose a free port for us.
	port, done, err := rpcutil.Serve(0, nil, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			analyzer := &analyzerServer{
				policyPackFactory: policyPack,
			}
			pulumirpc.RegisterAnalyzerServer(srv, analyzer)
			return nil
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("fatal: %v", err)
	}

	// The analyzer protocol requires that we now write out the port we have chosen to listen on.
	fmt.Printf("%d\n", port)

	// Finally, wait for the server to stop serving.
	if err := <-done; err != nil {
		return fmt.Errorf("fatal: %v", err)
	}

	return nil
}

type analyzerServer struct {
	pulumirpc.UnimplementedAnalyzerServer

	policyPackFactory func(pulumix.Engine) (PolicyPack, error)
	policyPack        PolicyPack

	config map[string]PolicyConfig
}

func (srv *analyzerServer) Handshake(
	ctx context.Context,
	req *pulumirpc.AnalyzerHandshakeRequest,
) (*pulumirpc.AnalyzerHandshakeResponse, error) {
	engine, err := pulumix.NewEngine(req.GetEngineAddress())
	if err != nil {
		return nil, fmt.Errorf("failed to create engine: %w", err)
	}

	srv.policyPack, err = srv.policyPackFactory(engine)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy pack: %w", err)
	}

	_, err = srv.policyPack.Handshake(ctx, HandshakeRequest{
		Engine:           engine,
		RootDirectory:    req.RootDirectory,
		ProgramDirectory: req.ProgramDirectory,
	})
	if err != nil {
		return nil, err
	}

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

func (srv *analyzerServer) Configure(ctx context.Context, req *pulumirpc.ConfigureAnalyzerRequest) (*pbempty.Empty,
	error,
) {
	conf := map[string]PolicyConfig{}
	for k, v := range req.PolicyConfig {
		data, err := v.GetProperties().MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal properties for policy %q: %w", k, err)
		}
		// Unmarshal the properties into an map[string]interface{}
		var props map[string]interface{}
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

				args := ResourceValidationArgs{
					Manager: policyManager,
					Config:  config.Properties,
					Resource: AnalyzerResource{
						Type:                 req.GetType(),
						Properties:           resource.FromResourcePropertyMap(pm),
						URN:                  req.GetUrn(),
						Name:                 req.GetName(),
						Options:              pulumi.ResourceOptions{},
						Provider:             AnalyzerProviderResource{},
						Parent:               "",  /* TODO */
						Dependencies:         nil, /* TODO */
						PropertyDependencies: nil, /* TODO */
					},
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
				args := ResourceRemediationArgs{
					Resource: AnalyzerResource{
						Type:                 req.GetType(),
						Properties:           props,
						URN:                  req.GetUrn(),
						Name:                 req.GetName(),
						Options:              pulumi.ResourceOptions{},
						Provider:             AnalyzerProviderResource{},
						Parent:               "",  /* TODO */
						Dependencies:         nil, /* TODO */
						PropertyDependencies: nil, /* TODO */
					},
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
	// TODO: Implement stack analysis
	return &pulumirpc.AnalyzeResponse{}, nil
}
