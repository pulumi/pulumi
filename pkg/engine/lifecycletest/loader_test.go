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

package lifecycletest

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

func TestLoader(t *testing.T) {
	t.Parallel()

	expectedSpec := schema.PackageSpec{
		Name:    "pkgA",
		Version: "1.0.0",
		Resources: map[string]schema.ResourceSpec{
			"pkgA:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "Resource",
					Properties: map[string]schema.PropertySpec{
						"foo": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
				},
			},
		},
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(ctx context.Context, req plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					bytes, err := json.Marshal(expectedSpec)
					if err != nil {
						contract.Failf("marshal schema: %v", err)
					}

					return plugin.GetSchemaResponse{
						Schema: bytes,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		// Check we can connect to the schema loader and query a schema.
		conn, err := grpc.NewClient(
			info.LoaderAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			rpcutil.GrpcChannelOptions(),
		)
		if err != nil {
			return fmt.Errorf("could not connect to resource monitor: %w", err)
		}
		defer conn.Close()
		loader := codegenrpc.NewLoaderClient(conn)

		ctx := context.Background()
		resp, err := loader.GetSchema(ctx, &codegenrpc.GetSchemaRequest{
			Package: "pkgA",
		})
		if err != nil {
			return fmt.Errorf("could not get schema: %w", err)
		}

		var actualSpec schema.PackageSpec
		err = json.Unmarshal(resp.Schema, &actualSpec)
		if err != nil {
			return fmt.Errorf("could not unmarshal schema: %w", err)
		}

		// Check the schema fields match
		assert.Equal(t, expectedSpec.Name, actualSpec.Name)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	_, err := lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
}
