package deploytest

import (
	"context"
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
		CallFunc: func(req *pulumirpc.CallRequest) (*pulumirpc.CallResponse, error) {
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
		"provider", "1.0")
	require.NoError(t, err)

	assert.Equal(t, map[resource.PropertyKey][]resource.URN{
		"foo": {"urn1", "urn2"},
		"bar": {"urn3", "urn4"},
	}, deps)
}

// stubResourceMonitorClient is a ResourceMonitorClient
// that can stub out specific functions.
type stubResourceMonitorClient struct {
	pulumirpc.ResourceMonitorClient

	CallFunc func(req *pulumirpc.CallRequest) (*pulumirpc.CallResponse, error)
}

func (cl *stubResourceMonitorClient) Call(
	ctx context.Context,
	req *pulumirpc.CallRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.CallResponse, error) {
	if cl.CallFunc != nil {
		return cl.CallFunc(req)
	}
	return cl.ResourceMonitorClient.Call(ctx, req, opts...)
}
