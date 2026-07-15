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

package convert

import (
	"context"
	"sync"

	"google.golang.org/grpc"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

// NewMapperServerFromContext creates a MapperServer that sources mappings from the plugins installed in the global
// plugin storage, booting them via the given context's host. It has the signature required by
// [plugin.NewMapperFunc]. The underlying mapper is constructed lazily on first request because enumerating installed
// plugins can fail, and context construction has no way to surface that error.
func NewMapperServerFromContext(pctx *plugin.Context) codegenrpc.MapperServer {
	return NewMapperServer(&contextMapper{pctx: pctx})
}

type contextMapper struct {
	pctx *plugin.Context

	once   sync.Once
	mapper Mapper
	err    error
}

func (m *contextMapper) GetMapping(
	ctx context.Context, provider string, hint *MapperPackageHint, ecosystem string,
) ([]byte, error) {
	m.once.Do(func() {
		base, err := NewBasePluginMapper(
			pluginstorage.Instance,
			"terraform",
			ProviderFactoryFromHost(m.pctx.Base(), m.pctx),
			func(string) *semver.Version { return nil },
			nil,
		)
		if err != nil {
			m.err = err
			return
		}
		m.mapper = NewCachingMapper(base)
	})
	if m.err != nil {
		return nil, m.err
	}
	return m.mapper.GetMapping(ctx, provider, hint, ecosystem)
}

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
	logging.V(7).Infof("%s executing: provider=%s, pulumi=%s, ecosystem=%s",
		label, req.Provider, req.PulumiProvider, req.Ecosystem)

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

	data, err := m.mapper.GetMapping(ctx, req.Provider, hint, req.Ecosystem)
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
