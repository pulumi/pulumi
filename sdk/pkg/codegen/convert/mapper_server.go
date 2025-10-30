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

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

type mapperServer struct {
	codegenrpc.UnsafeMapperServer // opt out of forward compat

	mapper Mapper
}

func NewMapperServer(mapper Mapper) codegenrpc.MapperServer {
	return &mapperServer{mapper: mapper}
}

func (m *mapperServer) GetMapping(ctx context.Context,
	req *codegenrpc.GetMappingRequest,
) (*codegenrpc.GetMappingResponse, error) {
	label := "GetMapping"
	logging.V(7).Infof("%s executing: provider=%s, pulumi=%s", label, req.Provider, req.PulumiProvider)

	var hint *MapperPackageHint
	if len(req.PulumiProvider) > 0 {
		hint = &MapperPackageHint{PluginName: req.PulumiProvider}

		if req.ParameterizationHint != nil {
			version, err := semver.ParseTolerant(req.ParameterizationHint.Version)
			if err != nil {
				return nil, err
			}

			hint.Parameterization = &workspace.Parameterization{
				Name:    req.ParameterizationHint.Name,
				Version: version,
				Value:   req.ParameterizationHint.Value,
			}
		}
	}

	data, err := m.mapper.GetMapping(ctx, req.Provider, hint)
	if err != nil {
		logging.V(7).Infof("%s failed: %v", label, err)
		return nil, err
	}

	logging.V(7).Infof("%s success: data=#%d", label, len(data))
	return &codegenrpc.GetMappingResponse{
		Data: data,
	}, nil
}

func MapperRegistration(m codegenrpc.MapperServer) func(*grpc.Server) {
	return func(srv *grpc.Server) {
		codegenrpc.RegisterMapperServer(srv, m)
	}
}
