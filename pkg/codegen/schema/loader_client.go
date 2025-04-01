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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
	"github.com/segmentio/encoding/json"
)

// LoaderClient reflects a loader service, loaded dynamically from the engine process over gRPC.
type LoaderClient struct {
	conn      *grpc.ClientConn        // the underlying gRPC connection.
	clientRaw codegenrpc.LoaderClient // the raw loader client; usually unsafe to use directly.
}

var _ ReferenceLoader = (*LoaderClient)(nil)

func NewLoaderClient(target string) (*LoaderClient, error) {
	contract.Assertf(target != "", "unexpected empty target for loader")

	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, err
	}

	l := &LoaderClient{
		conn:      conn,
		clientRaw: codegenrpc.NewLoaderClient(conn),
	}

	return l, nil
}

func (l *LoaderClient) Close() error {
	if l.clientRaw != nil {
		err := l.conn.Close()
		l.conn = nil
		l.clientRaw = nil
		return err
	}
	return nil
}

func (l *LoaderClient) LoadPackageReference(pkg string, version *semver.Version) (PackageReference, error) {
	return l.LoadPackageReferenceV2(context.TODO(), &PackageDescriptor{
		Name:    pkg,
		Version: version,
	})
}

func (l *LoaderClient) LoadPackageReferenceV2(
	ctx context.Context, descriptor *PackageDescriptor,
) (PackageReference, error) {
	label := "GetSchema"
	logging.V(7).Infof("%s executing: package=%s, version=%s", label, descriptor.Name, descriptor.Version)

	var versionString string
	if descriptor.Version != nil {
		versionString = descriptor.Version.String()
	}

	var parameterization *codegenrpc.Parameterization
	if descriptor.Replacement != nil {
		parameterization = &codegenrpc.Parameterization{
			Name:    descriptor.Replacement.Name,
			Version: descriptor.Replacement.Version.String(),
			Value:   descriptor.Replacement.Value,
		}
	}

	resp, err := l.clientRaw.GetSchema(ctx, &codegenrpc.GetSchemaRequest{
		Package:          descriptor.Name,
		Version:          versionString,
		DownloadUrl:      descriptor.DownloadURL,
		Parameterization: parameterization,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: %v", label, rpcError)
		return nil, err
	}

	var spec PartialPackageSpec
	if _, err := json.Parse(resp.Schema, &spec, json.ZeroCopy); err != nil {
		return nil, err
	}

	p, err := ImportPartialSpec(spec, nil, l)
	if err != nil {
		return nil, err
	}

	logging.V(7).Infof("%s success", label)
	return p, nil
}

func (l *LoaderClient) LoadPackage(pkg string, version *semver.Version) (*Package, error) {
	ref, err := l.LoadPackageReference(pkg, version)
	if err != nil {
		return nil, err
	}
	return ref.Definition()
}

func (l *LoaderClient) LoadPackageV2(ctx context.Context, descriptor *PackageDescriptor) (*Package, error) {
	ref, err := l.LoadPackageReferenceV2(ctx, descriptor)
	if err != nil {
		return nil, err
	}
	return ref.Definition()
}
