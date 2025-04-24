// Copyright 2016-2022, Pulumi Corporation.
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

package engine

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type clientLanguageRuntimeHost struct {
	plugin.Host

	languageRuntime plugin.LanguageRuntime
}

func connectToLanguageRuntime(ctx *plugin.Context, address string) (plugin.Host, error) {
	// Dial the language runtime.
	conn, err := grpc.NewClient(address, langRuntimePluginDialOptions(ctx, address)...)
	if err != nil {
		return nil, fmt.Errorf("could not connect to language host: %w", err)
	}

	client := pulumirpc.NewLanguageRuntimeClient(conn)
	return &clientLanguageRuntimeHost{
		Host:            ctx.Host,
		languageRuntime: plugin.NewLanguageRuntimeClient(ctx, clientRuntimeName, client),
	}, nil
}

func (host *clientLanguageRuntimeHost) LanguageRuntime(
	runtime string,
	info plugin.ProgramInfo,
) (plugin.LanguageRuntime, error) {
	return host.languageRuntime, nil
}

func langRuntimePluginDialOptions(ctx *plugin.Context, address string) []grpc.DialOption {
	dialOpts := append(
		rpcutil.OpenTracingInterceptorDialOptions(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)

	if ctx.DialOptions != nil {
		metadata := map[string]interface{}{
			"mode": "client",
			"kind": "language",
		}
		if address != "" {
			metadata["address"] = address
		}
		dialOpts = append(dialOpts, ctx.DialOptions(metadata)...)
	}

	return dialOpts
}
