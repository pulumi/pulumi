// Copyright 2016, Pulumi Corporation.
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
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

type converterServer struct {
	pulumirpc.UnsafeConverterServer // opt out of forward compat

	converter Converter
}

func NewConverterServer(converter Converter) pulumirpc.ConverterServer {
	return &converterServer{converter: converter}
}

func (c *converterServer) ConvertState(ctx context.Context,
	req *pulumirpc.ConvertStateRequest,
) (*pulumirpc.ConvertStateResponse, error) {
	resp, err := c.converter.ConvertState(ctx, &ConvertStateRequest{
		MapperTarget: req.MapperTarget,
		Args:         req.Args,
		LoaderTarget: req.LoaderTarget,
	})
	if err != nil {
		return nil, err
	}

	resources := make([]*pulumirpc.ResourceImport, len(resp.Resources))
	for i, resource := range resp.Resources {
		resources[i] = &pulumirpc.ResourceImport{
			Type:              resource.Type,
			Name:              resource.Name,
			Id:                resource.ID,
			Version:           resource.Version,
			PluginDownloadURL: resource.PluginDownloadURL,
			LogicalName:       resource.LogicalName,
			IsRemote:          resource.IsRemote,
			IsComponent:       resource.IsComponent,
			Parent:            resource.Parent,
			Properties:        resource.Properties,
			Provider:          resource.Provider,
		}
		if p := resource.Parameterization; p != nil {
			resources[i].Parameterization = &pulumirpc.ResourceParameterization{
				PluginName:    p.PluginName,
				PluginVersion: p.PluginVersion,
				Value:         p.Value,
			}
		}
		if e := resource.Extension; e != nil {
			resources[i].Extension = &pulumirpc.ResourceExtension{
				Name:    e.Name,
				Version: e.Version,
				Value:   e.Value,
			}
		}
	}

	providerImports := make(map[string]*pulumirpc.ProviderImport, len(resp.Providers))
	for name, p := range resp.Providers {
		var inputs *structpb.Struct
		if p.Inputs != nil {
			inputs, err = MarshalProperties(p.Inputs, MarshalOptions{KeepSecrets: true})
			if err != nil {
				return nil, fmt.Errorf("marshaling inputs for provider %q: %w", name, err)
			}
		}
		providerImports[name] = &pulumirpc.ProviderImport{
			Package: p.Package,
			Inputs:  inputs,
		}
	}

	// Translate the hcl.Diagnostics into rpc diagnostics.
	diags := slice.Prealloc[*codegenrpc.Diagnostic](len(resp.Diagnostics))
	for _, diag := range resp.Diagnostics {
		diags = append(diags, HclDiagnosticToRPCDiagnostic(diag))
	}

	rpcResp := &pulumirpc.ConvertStateResponse{
		Resources:   resources,
		Providers:   providerImports,
		Diagnostics: diags,
	}
	return rpcResp, nil
}

func (c *converterServer) ConvertProgram(ctx context.Context,
	req *pulumirpc.ConvertProgramRequest,
) (*pulumirpc.ConvertProgramResponse, error) {
	resp, err := c.converter.ConvertProgram(ctx, &ConvertProgramRequest{
		SourceDirectory:           req.SourceDirectory,
		TargetDirectory:           req.TargetDirectory,
		MapperTarget:              req.MapperTarget,
		LoaderTarget:              req.LoaderTarget,
		Args:                      req.Args,
		GeneratedProjectDirectory: req.GeneratedProjectDirectory,
	})
	if err != nil {
		return nil, err
	}

	// Translate the hcl.Diagnostics into rpc diagnostics.
	diags := slice.Prealloc[*codegenrpc.Diagnostic](len(resp.Diagnostics))
	for _, diag := range resp.Diagnostics {
		diags = append(diags, HclDiagnosticToRPCDiagnostic(diag))
	}

	return &pulumirpc.ConvertProgramResponse{
		Diagnostics: diags,
	}, nil
}

func (c *converterServer) ConvertSnippet(ctx context.Context,
	req *pulumirpc.ConvertSnippetRequest,
) (*pulumirpc.ConvertSnippetResponse, error) {
	resp, err := c.converter.ConvertSnippet(ctx, &ConvertSnippetRequest{
		Filename:     req.Filename,
		Source:       req.Source,
		TargetLoader: req.TargetLoader,
		Package:      req.Package,
		Token:        req.Token,
		Attributes:   req.Attributes,
	})
	if err != nil {
		return nil, err
	}

	diags := slice.Prealloc[*codegenrpc.Diagnostic](len(resp.Diagnostics))
	for _, diag := range resp.Diagnostics {
		diags = append(diags, HclDiagnosticToRPCDiagnostic(diag))
	}

	return &pulumirpc.ConvertSnippetResponse{
		Diagnostics: diags,
		Filename:    resp.Filename,
		Source:      resp.Source,
		Attributes:  resp.Attributes,
	}, nil
}
