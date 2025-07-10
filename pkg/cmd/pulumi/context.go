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

package main

import (
	"context"

	interceptors "github.com/pulumi/pulumi/pkg/v3/util/rpcdebug"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"google.golang.org/grpc"
)

func commandContext() context.Context {
	ctx := context.Background()
	ctx = plugin.WithServeOptions(ctx, serveOpts)
	return ctx
}

func serveOpts(pctx *plugin.Context) []grpc.ServerOption {
	logFile := env.DebugGRPC.Value()
	if logFile == "" {
		return nil
	}

	var serveOpts []grpc.ServerOption
	di, err := interceptors.NewDebugInterceptor(interceptors.DebugInterceptorOptions{
		LogFile: logFile,
		Mutex:   pctx.DebugTraceMutex,
	})
	if err != nil {
		// ignoring
		return nil
	}
	metadata := map[string]any{
		"mode": "server",
	}
	serveOpts = append(serveOpts, di.ServerOptions(interceptors.LogOptions{
		Metadata: metadata,
	})...)
	return serveOpts
}
