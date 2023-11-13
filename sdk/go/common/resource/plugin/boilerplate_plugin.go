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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// boilerplate reflects a boilerplate plugin, loaded dynamically from another process over gRPC.
type boilerplate struct {
	name      string
	plug      *plugin
	clientRaw pulumirpc.BoilerplateClient
}

func NewBoilerplate(ctx *Context, name string, version *semver.Version) (Boilerplate, error) {
	prefix := fmt.Sprintf("%v (boilerplate)", name)

	// Load the plugin's path by using the standard workspace logic.
	path, err := workspace.GetPluginPath(ctx.Diag, workspace.BoilerplatePlugin,
		name, version, ctx.Host.GetProjectPlugins())
	if err != nil {
		return nil, err
	}

	contract.Assertf(path != "", "unexpected empty path for plugin %s", name)

	plug, err := newPlugin(ctx, ctx.Pwd, path, prefix,
		workspace.BoilerplatePlugin, []string{}, os.Environ(), boilerplatePluginDialOptions(ctx, name, ""))
	if err != nil {
		return nil, err
	}

	contract.Assertf(plug != nil, "unexpected nil boilerplate plugin for %s", name)

	b := &boilerplate{
		name:      name,
		plug:      plug,
		clientRaw: pulumirpc.NewBoilerplateClient(plug.Conn),
	}

	return b, nil
}

func boilerplatePluginDialOptions(ctx *Context, name string, path string) []grpc.DialOption {
	dialOpts := append(
		rpcutil.OpenTracingInterceptorDialOptions(otgrpc.SpanDecorator(decorateProviderSpans)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)

	if ctx.DialOptions != nil {
		metadata := map[string]interface{}{
			"mode": "client",
			"kind": "boilerplate",
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
func (b *boilerplate) label() string {
	return fmt.Sprintf("Boilerplate[%s, %p]", b.name, b)
}

func (b *boilerplate) Close() error {
	if b.plug == nil {
		return nil
	}
	return b.plug.Close()
}

func (b *boilerplate) CreatePackage(ctx context.Context, req *CreatePackageRequest) (*CreatePackageResponse, error) {
	label := fmt.Sprintf("%s.CreatePackage", b.label())
	logging.V(7).Infof("%s executing", label)

	_, err := b.clientRaw.CreatePackage(ctx, &pulumirpc.CreatePackageRequest{
		Name:   req.Name,
		Config: req.Config,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(8).Infof("%s boilerplate received rpc error `%s`: `%s`", label, rpcError.Code(), rpcError.Message())
		return nil, err
	}

	logging.V(7).Infof("%s success", label)
	return &CreatePackageResponse{}, nil
}
