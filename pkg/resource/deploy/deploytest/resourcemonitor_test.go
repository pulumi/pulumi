// Copyright 2023-2024, Pulumi Corporation.
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

package deploytest

import (
	"context"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

// Verifies that ReturnDependencies in a ResourceMonitor.Call
// are propagated back to the caller.
func TestResourceMonitor_Call_deps(t *testing.T) {
	t.Parallel()

	client := stubResourceMonitorClient{
		// ResourceMonitorClient is unset
		// so this will panic if an unexpected method is called.
		CallFunc: func(req *pulumirpc.ResourceCallRequest) (*pulumirpc.CallResponse, error) {
			assert.ElementsMatch(t, req.ArgDependencies["k1"].Urns, []string{"urn1", "urn2"})

			res, err := structpb.NewStruct(map[string]interface{}{
				"k3": "value3",
				"k4": "value4",
			})
			require.NoError(t, err)

			return &pulumirpc.CallResponse{
				Return: res,
				ReturnDependencies: map[string]*pulumirpc.CallResponse_ReturnDependencies{
					"foo": {Urns: []string{"urn1", "urn2"}},
					"bar": {Urns: []string{"urn3", "urn4"}},
				},
			}, nil
		},
	}

	_, deps, _, err := NewResourceMonitor(&client).Call(
		"org/proj/stack:module:member",
		resource.NewPropertyMapFromMap(map[string]interface{}{
			"k1": "value1",
			"k2": "value2",
		}),
		map[resource.PropertyKey][]resource.URN{
			"k1": {"urn1", "urn2"},
		},
		"provider", "1.0", "")
	require.NoError(t, err)

	assert.Equal(t, map[resource.PropertyKey][]resource.URN{
		"foo": {"urn1", "urn2"},
		"bar": {"urn3", "urn4"},
	}, deps)
}

func TestResourceMonitor_RegisterResource_customTimeouts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give *resource.CustomTimeouts
		want *pulumirpc.RegisterResourceRequest_CustomTimeouts
	}{
		{desc: "nil", give: nil, want: nil},
		{
			desc: "create",
			give: &resource.CustomTimeouts{Create: 1},
			want: &pulumirpc.RegisterResourceRequest_CustomTimeouts{Create: "1s"},
		},
		{
			desc: "update",
			give: &resource.CustomTimeouts{Update: 1},
			want: &pulumirpc.RegisterResourceRequest_CustomTimeouts{Update: "1s"},
		},
		{
			desc: "delete",
			give: &resource.CustomTimeouts{Delete: 1},
			want: &pulumirpc.RegisterResourceRequest_CustomTimeouts{Delete: "1s"},
		},
		{
			desc: "all",
			give: &resource.CustomTimeouts{Create: 1, Update: 2, Delete: 3},
			want: &pulumirpc.RegisterResourceRequest_CustomTimeouts{
				Create: "1s",
				Update: "2s",
				Delete: "3s",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			client := stubResourceMonitorClient{
				RegisterResourceFunc: func(
					req *pulumirpc.RegisterResourceRequest,
				) (*pulumirpc.RegisterResourceResponse, error) {
					assert.Equal(t, tt.want, req.CustomTimeouts)
					return &pulumirpc.RegisterResourceResponse{}, nil
				},
			}

			_, err := NewResourceMonitor(&client).RegisterResource("pkg:m:typ", "foo", true, ResourceOptions{
				CustomTimeouts: tt.give,
			})
			require.NoError(t, err)
		})
	}
}

// stubResourceMonitorClient is a ResourceMonitorClient
// that can stub out specific functions.
type stubResourceMonitorClient struct {
	pulumirpc.ResourceMonitorClient

	CallFunc             func(req *pulumirpc.ResourceCallRequest) (*pulumirpc.CallResponse, error)
	RegisterResourceFunc func(req *pulumirpc.RegisterResourceRequest) (*pulumirpc.RegisterResourceResponse, error)
}

func (cl *stubResourceMonitorClient) Call(
	ctx context.Context,
	req *pulumirpc.ResourceCallRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.CallResponse, error) {
	if cl.CallFunc != nil {
		return cl.CallFunc(req)
	}
	return cl.ResourceMonitorClient.Call(ctx, req, opts...)
}

func (cl *stubResourceMonitorClient) RegisterResource(
	ctx context.Context,
	req *pulumirpc.RegisterResourceRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.RegisterResourceResponse, error) {
	if cl.RegisterResourceFunc != nil {
		return cl.RegisterResourceFunc(req)
	}
	return cl.ResourceMonitorClient.RegisterResource(ctx, req, opts...)
}

func TestPrepareTestTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give float64
		want string
	}{
		{0, ""},
		{1, "1s"},
		{1.5, "1.5s"},
		{62, "1m2s"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprint(tt.give), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, prepareTestTimeout(tt.give))
		})
	}
}
