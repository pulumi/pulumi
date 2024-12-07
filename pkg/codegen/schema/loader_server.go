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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
	schemarpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen/schema"
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

	pkg, err := m.loader.LoadPackage(req.Package, version)
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

func toPackageDescriptor(req *codegen.GetSchemaRequest) (*PackageDescriptor, error) {
	var version *semver.Version
	if req.Version != "" {
		v, err := semver.ParseTolerant(req.Version)
		if err != nil {
			return nil, fmt.Errorf("%s not a valid semver: %w", req.Version, err)
		}
		version = &v
	}

	var parameterization *ParameterizationDescriptor
	if req.Parameterization != nil {
		v, err := semver.ParseTolerant(req.Version)
		if err != nil {
			return nil, fmt.Errorf("%s not a valid semver: %w", req.Version, err)
		}

		parameterization = &ParameterizationDescriptor{
			Name:    req.Parameterization.Name,
			Version: v,
			Value:   req.Parameterization.Value,
		}
	}

	return &PackageDescriptor{
		Name:             req.Package,
		Version:          version,
		DownloadURL:      req.DownloadUrl,
		Parameterization: parameterization,
	}, nil
}

func (m *loaderServer) GetPackageInfo(
	ctx context.Context, req *codegen.GetSchemaRequest,
) (*schemarpc.PackageInfo, error) {
	label := "GetSchema"
	logging.V(7).Infof("%s executing: package=%s, version=%s", label, req.Package, req.Version)

	packageDescriptor, err := toPackageDescriptor(req)
	if err != nil {
		logging.V(7).Infof("%s failed: %v", label, err)
		return nil, err
	}

	pkg, err := m.loader.LoadPackageReferenceV2(ctx, packageDescriptor)
	if err != nil {
		logging.V(7).Infof("%s failed: %v", label, err)
		return nil, err
	}

	toString := func(v *semver.Version) *string {
		if v == nil {
			return nil
		}
		s := v.String()
		return &s
	}

	logging.V(7).Infof("%s success", label)
	return &schemarpc.PackageInfo{
		Name:    pkg.Name(),
		Version: toString(pkg.Version()),
	}, nil
}

func (m *loaderServer) GetResources(
	ctx context.Context, req *codegen.GetSchemaRequest,
) (*schemarpc.List, error) {
	label := "GetResources"
	logging.V(7).Infof("%s executing: package=%s, version=%s", label, req.Package, req.Version)

	packageDescriptor, err := toPackageDescriptor(req)
	if err != nil {
		logging.V(7).Infof("%s failed: %v", label, err)
		return nil, err
	}

	pkg, err := m.loader.LoadPackageReferenceV2(ctx, packageDescriptor)
	if err != nil {
		logging.V(7).Infof("%s failed: %v", label, err)
		return nil, err
	}

	resources := pkg.Resources()
	var items []string
	for it := resources.Range(); it.Next(); {
		items = append(items, it.Token())
	}

	logging.V(7).Infof("%s success", label)
	return &schemarpc.List{
		Items: items,
	}, nil
}

func (m *loaderServer) GetResource(
	ctx context.Context, req *codegen.GetSchemaPartRequest,
) (*schemarpc.Resource, error) {
	label := "GetResource"
	logging.V(7).Infof("%s executing: package=%s, version=%s, item=%s", label, req.Request.Package, req.Request.Version, req.Item)

	packageDescriptor, err := toPackageDescriptor(req.Request)
	if err != nil {
		logging.V(7).Infof("%s failed: %v", label, err)
		return nil, err
	}

	pkg, err := m.loader.LoadPackageReferenceV2(ctx, packageDescriptor)
	if err != nil {
		logging.V(7).Infof("%s failed: %v", label, err)
		return nil, err
	}

	resources := pkg.Resources()
	resource, has, err := resources.Get(req.Item)
	if err != nil {
		logging.V(7).Infof("%s failed: %v", label, err)
		return nil, err
	}
	if !has {
		logging.V(7).Infof("%s failed: resource not found", label)
		return nil, status.Errorf(codes.NotFound, "resource %q not found", req.Item)
	}

	toOptionalString := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}

	logging.V(7).Infof("%s success", label)
	return &schemarpc.Resource{
		Description: toOptionalString(resource.Comment),
	}, nil
}

func LoaderRegistration(l codegenrpc.LoaderServer) func(*grpc.Server) {
	return func(srv *grpc.Server) {
		codegenrpc.RegisterLoaderServer(srv, l)
	}
}
