package pulumi

import (
	"log"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"
)

type MockResourceMonitor interface {
	Call(token string, args resource.PropertyMap, provider string) (resource.PropertyMap, error)
	NewResource(typeToken, name string, inputs resource.PropertyMap,
		provider, id string) (string, resource.PropertyMap, error)
}

func WithMocks(project, stack string, mocks MockResourceMonitor) RunOption {
	return func(info *RunInfo) {
		info.Project, info.Stack, info.Mocks = project, stack, mocks
	}
}

type mockMonitor struct {
	project string
	stack   string
	mocks   MockResourceMonitor
}

func (m *mockMonitor) newURN(parent, typ, name string) string {
	parentType := tokens.Type("")
	if parentURN := resource.URN(parent); parentURN != "" && parentURN.Type() != resource.RootStackType {
		parentType = parentURN.QualifiedType()
	}

	return string(resource.NewURN(tokens.QName(m.stack), tokens.PackageName(m.project), parentType, tokens.Type(typ),
		tokens.QName(name)))
}

func (m *mockMonitor) SupportsFeature(ctx context.Context, in *pulumirpc.SupportsFeatureRequest,
	opts ...grpc.CallOption) (*pulumirpc.SupportsFeatureResponse, error) {

	return &pulumirpc.SupportsFeatureResponse{
		HasSupport: true,
	}, nil
}

func (m *mockMonitor) Invoke(ctx context.Context, in *pulumirpc.InvokeRequest,
	opts ...grpc.CallOption) (*pulumirpc.InvokeResponse, error) {

	args, err := plugin.UnmarshalProperties(in.GetArgs(), plugin.MarshalOptions{
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, err
	}

	resultV, err := m.mocks.Call(in.GetTok(), args, in.GetProvider())
	if err != nil {
		return nil, err
	}

	result, err := plugin.MarshalProperties(resultV, plugin.MarshalOptions{
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, err
	}

	return &pulumirpc.InvokeResponse{
		Return: result,
	}, nil
}

func (m *mockMonitor) StreamInvoke(ctx context.Context, in *pulumirpc.InvokeRequest,
	opts ...grpc.CallOption) (pulumirpc.ResourceMonitor_StreamInvokeClient, error) {

	panic("not implemented")
}

func (m *mockMonitor) ReadResource(ctx context.Context, in *pulumirpc.ReadResourceRequest,
	opts ...grpc.CallOption) (*pulumirpc.ReadResourceResponse, error) {

	stateIn, err := plugin.UnmarshalProperties(in.GetProperties(), plugin.MarshalOptions{
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, err
	}

	_, state, err := m.mocks.NewResource(in.GetType(), in.GetName(), stateIn, in.GetProvider(), in.GetId())
	if err != nil {
		return nil, err
	}

	stateOut, err := plugin.MarshalProperties(state, plugin.MarshalOptions{
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, err
	}

	return &pulumirpc.ReadResourceResponse{
		Urn:        m.newURN(in.GetParent(), in.GetType(), in.GetName()),
		Properties: stateOut,
	}, nil
}

func (m *mockMonitor) RegisterResource(ctx context.Context, in *pulumirpc.RegisterResourceRequest,
	opts ...grpc.CallOption) (*pulumirpc.RegisterResourceResponse, error) {

	if in.GetType() == string(resource.RootStackType) {
		return &pulumirpc.RegisterResourceResponse{
			Urn: m.newURN(in.GetParent(), in.GetType(), in.GetName()),
		}, nil
	}

	inputs, err := plugin.UnmarshalProperties(in.GetObject(), plugin.MarshalOptions{
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, err
	}

	id, state, err := m.mocks.NewResource(in.GetType(), in.GetName(), inputs, in.GetProvider(), in.GetImportId())
	if err != nil {
		return nil, err
	}

	stateOut, err := plugin.MarshalProperties(state, plugin.MarshalOptions{
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, err
	}

	return &pulumirpc.RegisterResourceResponse{
		Urn:    m.newURN(in.GetParent(), in.GetType(), in.GetName()),
		Id:     id,
		Object: stateOut,
	}, nil
}

func (m *mockMonitor) RegisterResourceOutputs(ctx context.Context, in *pulumirpc.RegisterResourceOutputsRequest,
	opts ...grpc.CallOption) (*empty.Empty, error) {

	return &empty.Empty{}, nil
}

type mockEngine struct {
	logger       *log.Logger
	rootResource string
}

// Log logs a global message in the engine, including errors and warnings.
func (m *mockEngine) Log(ctx context.Context, in *pulumirpc.LogRequest,
	opts ...grpc.CallOption) (*empty.Empty, error) {

	if m.logger != nil {
		m.logger.Printf("%s: %s", in.GetSeverity(), in.GetMessage())
	}
	return &empty.Empty{}, nil
}

// GetRootResource gets the URN of the root resource, the resource that should be the root of all
// otherwise-unparented resources.
func (m *mockEngine) GetRootResource(ctx context.Context, in *pulumirpc.GetRootResourceRequest,
	opts ...grpc.CallOption) (*pulumirpc.GetRootResourceResponse, error) {

	return &pulumirpc.GetRootResourceResponse{
		Urn: m.rootResource,
	}, nil
}

// SetRootResource sets the URN of the root resource.
func (m *mockEngine) SetRootResource(ctx context.Context, in *pulumirpc.SetRootResourceRequest,
	opts ...grpc.CallOption) (*pulumirpc.SetRootResourceResponse, error) {

	m.rootResource = in.GetUrn()
	return &pulumirpc.SetRootResourceResponse{}, nil
}
