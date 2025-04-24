// Copyright 2024, Pulumi Corporation.
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

package tests

import (
	"encoding/json"

	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

// Helper for writing asserts against gRPC requests received by the test provider. See also config_grpc_provider.go.
type grpcTestContext struct {
	l *L
	s *deploy.Snapshot
}

func (ctx *grpcTestContext) CheckConfigReq(resourceName string) *pulumirpc.CheckRequest {
	bytes := ctx.parseCapturedConfig(ctx.configGetterCapturedConfig(resourceName), providers.CheckConfigMethod)
	var req pulumirpc.CheckRequest
	err := protojson.Unmarshal(bytes, &req)
	require.NoError(ctx.l, err)
	return &req
}

func (ctx *grpcTestContext) ConfigureReq(resourceName string) *pulumirpc.ConfigureRequest {
	bytes := ctx.parseCapturedConfig(ctx.configGetterCapturedConfig(resourceName), providers.ConfigureMethod)
	var req pulumirpc.ConfigureRequest
	err := protojson.Unmarshal(bytes, &req)
	require.NoError(ctx.l, err)
	return &req
}

func (ctx *grpcTestContext) parseCapturedConfig(raw string, method providers.RPCMethod) json.RawMessage {
	var entries []providers.RPCRequest
	err := json.Unmarshal([]byte(raw), &entries)
	require.NoErrorf(ctx.l, err, "Failed to parse captured config")
	foundCount := 0
	var foundEntry providers.RPCRequest
	for _, e := range entries {
		if e.Method == method {
			foundEntry = e
			foundCount++
		}
	}
	require.Equal(ctx.l, 1, foundCount, "Expected exactly 1 call %s call, got %d", method, foundCount)
	return foundEntry.Message
}

func (ctx *grpcTestContext) configGetterCapturedConfig(resourceName string) string {
	l := ctx.l
	snap := ctx.s
	for _, r := range snap.Resources {
		if r.URN.Name() != resourceName {
			continue
		}
		require.Equal(l, "config-grpc:index:ConfigFetcher", string(r.Type))
		configOut, gotConfig := r.Outputs["config"]
		require.Truef(l, gotConfig, "No `config` output")
		require.Truef(l, configOut.IsString(), "`config` output must be a string")
		return configOut.StringValue()
	}
	require.Failf(l, "Resource not found", "resourceName=%s", resourceName)
	return ""
}
