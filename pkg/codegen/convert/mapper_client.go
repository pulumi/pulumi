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

package convert

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

// mapperClient reflects a mapper service, loaded dynamically from the engine process over gRPC.
type mapperClient struct {
	conn      *grpc.ClientConn        // the underlying gRPC connection.
	clientRaw codegenrpc.MapperClient // the raw mapper client; usually unsafe to use directly.
}

func NewMapperClient(target string) (Mapper, error) {
	contract.Assertf(target != "", "unexpected empty target for mapper")

	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, err
	}

	m := &mapperClient{
		conn:      conn,
		clientRaw: codegenrpc.NewMapperClient(conn),
	}

	return m, nil
}

func (m *mapperClient) Close() error {
	if m.clientRaw != nil {
		err := m.conn.Close()
		m.conn = nil
		m.clientRaw = nil
		return err
	}
	return nil
}

func (m *mapperClient) GetMapping(
	ctx context.Context,
	provider string,
	hint *MapperPackageHint,
) ([]byte, error) {
	label := "GetMapping"
	logging.V(7).Infof("%s executing: provider=%s, hint=%v", label, provider, hint)

	pluginName := ""
	var parameterizationHint *codegenrpc.MapperParameterizationHint
	if hint != nil {
		pluginName = hint.PluginName
		if hint.Parameterization != nil {
			parameterizationHint = &codegenrpc.MapperParameterizationHint{
				Name:    hint.Parameterization.Name,
				Version: hint.Parameterization.Version.String(),
				Value:   hint.Parameterization.Value,
			}
		}
	}

	resp, err := m.clientRaw.GetMapping(ctx, &codegenrpc.GetMappingRequest{
		Provider:             provider,
		PulumiProvider:       pluginName,
		ParameterizationHint: parameterizationHint,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: %v", label, rpcError)
		return nil, err
	}

	logging.V(7).Infof("%s success: data=#%d", label, len(resp.Data))
	return resp.Data, nil
}
