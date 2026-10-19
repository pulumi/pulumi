// Copyright 2026, Pulumi Corporation.
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

package pulumi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/internal"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// unknownComponentMonitor answers every RegisterResource with a successful response whose "out" output is unknown,
// and leaves the response's Unknown flag unset, as the engine does for remote components.
type unknownComponentMonitor struct {
	pulumirpc.ResourceMonitorClient
}

func (m *unknownComponentMonitor) RegisterResource(
	ctx context.Context, in *pulumirpc.RegisterResourceRequest, opts ...grpc.CallOption,
) (*pulumirpc.RegisterResourceResponse, error) {
	resp, err := m.ResourceMonitorClient.RegisterResource(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	obj, err := plugin.MarshalProperties(
		resource.PropertyMap{"out": resource.MakeComputed(resource.NewProperty(""))},
		plugin.MarshalOptions{KeepUnknowns: true},
	)
	if err != nil {
		return nil, err
	}
	resp.Object = obj
	return resp, nil
}

type unknownOutComponent struct {
	ResourceState

	Out StringOutput `pulumi:"out"`
}

// A remote component's unknown output must be observed as unknown by the program in a real update.
func TestRemoteComponentUnknownOutputRealUpdate(t *testing.T) {
	t.Parallel()

	err := RunErr(func(ctx *Context) error {
		var comp unknownOutComponent
		require.NoError(t, ctx.RegisterRemoteComponentResource("pkg:index:Comp", "comp", nil, &comp))

		_, known, _, _, err := internal.AwaitOutput(ctx.Context(), comp.Out)
		require.NoError(t, err)
		assert.False(t, known, "unknown output of a remote component resolved as known")
		return nil
	}, WithMocks("project", "stack", &testMonitor{}), WrapResourceMonitorClient(
		func(c pulumirpc.ResourceMonitorClient) pulumirpc.ResourceMonitorClient {
			return &unknownComponentMonitor{ResourceMonitorClient: c}
		}))
	require.NoError(t, err)
}
