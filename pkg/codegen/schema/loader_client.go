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

// loaderClient reflects a loader service, loaded dynamically from the engine process over gRPC.
type loaderClient struct {
	conn      *grpc.ClientConn        // the underlying gRPC connection.
	clientRaw codegenrpc.LoaderClient // the raw loader client; usually unsafe to use directly.
}

func NewLoaderClient(target string) (ReferenceLoader, error) {
	contract.Assertf(target != "", "unexpected empty target for loader")

	conn, err := grpc.Dial(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, err
	}

	l := &loaderClient{
		conn:      conn,
		clientRaw: codegenrpc.NewLoaderClient(conn),
	}

	return l, nil
}

func (l *loaderClient) Close() error {
	if l.clientRaw != nil {
		err := l.conn.Close()
		l.conn = nil
		l.clientRaw = nil
		return err
	}
	return nil
}

func (l *loaderClient) LoadPackageReference(pkg string, version *semver.Version) (PackageReference, error) {
	label := "GetSchema"
	logging.V(7).Infof("%s executing: package=%s, version=%s", label, pkg, version)

	var versionString string
	if version != nil {
		versionString = version.String()
	}

	resp, err := l.clientRaw.GetSchema(context.TODO(), &codegenrpc.GetSchemaRequest{
		Package: pkg,
		Version: versionString,
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

	// Insert a version into the spec if the package does not provide one or if the
	// existing version is less than the provided one
	if version != nil {
		setVersion := true
		if spec.PackageInfoSpec.Version != "" {
			vSemver, err := semver.Make(spec.PackageInfoSpec.Version)
			if err == nil {
				if vSemver.Compare(*version) == 1 {
					setVersion = false
				}
			}
		}
		if setVersion {
			spec.PackageInfoSpec.Version = version.String()
		}
	}

	p, err := ImportPartialSpec(spec, nil, l)
	if err != nil {
		return nil, err
	}

	logging.V(7).Infof("%s success", label)
	return p, nil
}

func (l *loaderClient) LoadPackage(pkg string, version *semver.Version) (*Package, error) {
	ref, err := l.LoadPackageReference(pkg, version)
	if err != nil {
		return nil, err
	}
	return ref.Definition()
}
