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

package schema

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
	"github.com/pulumi/pulumi/sdk/v3/proto/go/codegen/schema"
	"github.com/segmentio/encoding/json"
)

type loaderServer struct {
	codegenrpc.UnsafeLoaderServer // opt out of forward compat

	loader ReferenceLoader
}

func NewLoaderServer(loader ReferenceLoader) codegenrpc.LoaderServer {
	return &loaderServer{loader: loader}
}

func (m *loaderServer) GetSchema(ctx context.Context,
	req *codegenrpc.GetSchemaRequest,
) (*codegenrpc.GetSchemaResponse, error) {
	label := "GetSchema"
	logging.V(7).Infof("%s executing: package=%s, version=%s", label, req.Package, req.Version)

	var version *semver.Version
	if req.Version != "" {
		v, err := semver.ParseTolerant(req.Version)
		if err != nil {
			logging.V(7).Infof("%s failed: %v", label, err)
			return nil, fmt.Errorf("%s not a valid semver: %w", req.Version, err)
		}
		version = &v
	}

	descriptor := &PackageDescriptor{
		Name:        req.Package,
		Version:     version,
		DownloadURL: req.DownloadUrl,
	}
	if req.Parameterization != nil {
		descriptor.Parameterization = &ParameterizationDescriptor{
			Name:  req.Parameterization.Name,
			Value: req.Parameterization.Value,
		}

		v, err := semver.ParseTolerant(req.Parameterization.Version)
		if err != nil {
			logging.V(7).Infof("%s failed: %v", label, err)
			return nil, fmt.Errorf("%s not a valid semver: %w", req.Version, err)
		}
		descriptor.Parameterization.Version = v
	}

	pkg, err := m.loader.LoadPackageV2(ctx, descriptor)
	if err != nil {
		logging.V(7).Infof("%s failed: %v", label, err)
		return nil, err
	}

	// Marshal the package into a JSON string.
	spec, err := pkg.MarshalSpec()
	if err != nil {
		logging.V(7).Infof("%s failed: %v", label, err)
		return nil, err
	}

	data, err := json.Marshal(spec)
	if err != nil {
		logging.V(7).Infof("%s failed: %v", label, err)
		return nil, err
	}

	logging.V(7).Infof("%s success: data=#%d", label, len(data))
	return &codegenrpc.GetSchemaResponse{
		Schema: data,
	}, nil
}

func LoaderRegistration(l codegenrpc.LoaderServer) func(*grpc.Server) {
	return func(srv *grpc.Server) {
		codegenrpc.RegisterLoaderServer(srv, l)
	}
}
