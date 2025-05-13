// Copyright 2016-2023, Pulumi Corporation.
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
	"os"

	"github.com/blang/semver"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/hashicorp/hcl/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// converter reflects a converter plugin, loaded dynamically from another process over gRPC.
type converter struct {
	name      string
	plug      *plugin                   // the actual plugin process wrapper.
	clientRaw pulumirpc.ConverterClient // the raw provider client; usually unsafe to use directly.
}

func NewConverter(ctx *Context, name string, version *semver.Version) (Converter, error) {
	prefix := fmt.Sprintf("%v (converter)", name)

	// Load the plugin's path by using the standard workspace logic.
	path, err := workspace.GetPluginPath(
		ctx.baseContext, ctx.Diag,
		workspace.PluginSpec{Name: name, Version: version, Kind: apitype.ConverterPlugin},
		ctx.Host.GetProjectPlugins())
	if err != nil {
		return nil, err
	}

	contract.Assertf(path != "", "unexpected empty path for plugin %s", name)

	plug, _, err := newPlugin(ctx, ctx.Pwd, path, prefix,
		apitype.ConverterPlugin, []string{}, os.Environ(),
		testConnection, converterPluginDialOptions(ctx, name, ""),
		ctx.Host.AttachDebugger())
	if err != nil {
		return nil, err
	}

	contract.Assertf(plug != nil, "unexpected nil converter plugin for %s", name)

	c := &converter{
		name:      name,
		plug:      plug,
		clientRaw: pulumirpc.NewConverterClient(plug.Conn),
	}

	return c, nil
}

func converterPluginDialOptions(ctx *Context, name string, path string) []grpc.DialOption {
	dialOpts := append(
		rpcutil.OpenTracingInterceptorDialOptions(otgrpc.SpanDecorator(decorateProviderSpans)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)

	if ctx.DialOptions != nil {
		metadata := map[string]interface{}{
			"mode": "client",
			"kind": "converter",
		}
		if name != "" {
			metadata["name"] = name
		}
		if path != "" {
			metadata["path"] = path
		}
		dialOpts = append(dialOpts, ctx.DialOptions(metadata)...)
	}

	return dialOpts
}

// label returns a base label for tracing functions.
func (c *converter) label() string {
	return fmt.Sprintf("Converter[%s, %p]", c.name, c)
}

func (c *converter) Close() error {
	if c.plug == nil {
		return nil
	}
	return c.plug.Close()
}

func (c *converter) ConvertState(ctx context.Context, req *ConvertStateRequest) (*ConvertStateResponse, error) {
	label := c.label() + ".ConvertState"
	logging.V(7).Infof("%s executing", label)

	resp, err := c.clientRaw.ConvertState(ctx, &pulumirpc.ConvertStateRequest{
		MapperTarget: req.MapperTarget,
		Args:         req.Args,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(8).Infof("%s converter received rpc error `%s`: `%s`", label, rpcError.Code(), rpcError.Message())
		return nil, err
	}

	resources := make([]ResourceImport, len(resp.Resources))
	for i, resource := range resp.Resources {
		resources[i] = ResourceImport{
			Type:              resource.Type,
			Name:              resource.Name,
			ID:                resource.Id,
			Version:           resource.Version,
			PluginDownloadURL: resource.PluginDownloadURL,
			LogicalName:       resource.LogicalName,
			IsRemote:          resource.IsRemote,
			IsComponent:       resource.IsComponent,
		}
	}

	// Translate the rpc diagnostics into hcl.Diagnostics.
	var diags hcl.Diagnostics
	for _, rpcDiag := range resp.Diagnostics {
		diags = append(diags, RPCDiagnosticToHclDiagnostic(rpcDiag))
	}

	logging.V(7).Infof("%s success", label)
	return &ConvertStateResponse{
		Resources:   resources,
		Diagnostics: diags,
	}, nil
}

func (c *converter) ConvertProgram(ctx context.Context, req *ConvertProgramRequest) (*ConvertProgramResponse, error) {
	label := c.label() + ".ConvertProgram"
	logging.V(7).Infof("%s executing", label)

	resp, err := c.clientRaw.ConvertProgram(ctx, &pulumirpc.ConvertProgramRequest{
		SourceDirectory: req.SourceDirectory,
		TargetDirectory: req.TargetDirectory,
		MapperTarget:    req.MapperTarget,
		LoaderTarget:    req.LoaderTarget,
		Args:            req.Args,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(8).Infof("%s converter received rpc error `%s`: `%s`", label, rpcError.Code(), rpcError.Message())
		return nil, err
	}

	// Translate the rpc diagnostics into hcl.Diagnostics.
	var diags hcl.Diagnostics
	for _, rpcDiag := range resp.Diagnostics {
		diags = append(diags, RPCDiagnosticToHclDiagnostic(rpcDiag))
	}

	logging.V(7).Infof("%s success", label)
	return &ConvertProgramResponse{
		Diagnostics: diags,
	}, nil
}
