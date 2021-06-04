package plugin

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	empty "github.com/golang/protobuf/ptypes/empty"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/stretchr/testify/assert"
	grpc "google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestAnnotateSecrets(t *testing.T) {
	from := resource.PropertyMap{
		"stringValue": resource.MakeSecret(resource.NewStringProperty("hello")),
		"numberValue": resource.MakeSecret(resource.NewNumberProperty(1.00)),
		"boolValue":   resource.MakeSecret(resource.NewBoolProperty(true)),
		"secretArrayValue": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("b"),
			resource.NewStringProperty("c"),
		})),
		"secretObjectValue": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.NewStringProperty("bValue"),
			"c": resource.NewStringProperty("cValue"),
		})),
		"objectWithSecretValue": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.MakeSecret(resource.NewStringProperty("bValue")),
			"c": resource.NewStringProperty("cValue"),
		}),
	}

	to := resource.PropertyMap{
		"stringValue": resource.NewStringProperty("hello"),
		"numberValue": resource.NewNumberProperty(1.00),
		"boolValue":   resource.NewBoolProperty(true),
		"secretArrayValue": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("b"),
			resource.NewStringProperty("c"),
		}),
		"secretObjectValue": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.NewStringProperty("bValue"),
			"c": resource.NewStringProperty("cValue"),
		}),
		"objectWithSecretValue": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.NewStringProperty("bValue"),
			"c": resource.NewStringProperty("cValue"),
		}),
	}

	annotateSecrets(to, from)

	assert.Truef(t, reflect.DeepEqual(to, from), "objects should be deeply equal")
}

func TestAnnotateSecretsDifferentProperties(t *testing.T) {
	// ensure that if from and and to have different shapes, values on from are not put into to, values on to which
	// are not present in from stay in to, but any secretness is propigated for shared keys.

	from := resource.PropertyMap{
		"stringValue": resource.MakeSecret(resource.NewStringProperty("hello")),
		"numberValue": resource.MakeSecret(resource.NewNumberProperty(1.00)),
		"boolValue":   resource.MakeSecret(resource.NewBoolProperty(true)),
		"secretObjectValue": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.NewStringProperty("bValue"),
			"c": resource.NewStringProperty("cValue"),
		})),
		"objectWithSecretValue": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.MakeSecret(resource.NewStringProperty("bValue")),
			"c": resource.NewStringProperty("cValue"),
		}),
		"extraFromValue": resource.NewStringProperty("extraFromValue"),
	}

	to := resource.PropertyMap{
		"stringValue": resource.NewStringProperty("hello"),
		"numberValue": resource.NewNumberProperty(1.00),
		"boolValue":   resource.NewBoolProperty(true),
		"secretObjectValue": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.NewStringProperty("bValue"),
			"c": resource.NewStringProperty("cValue"),
		})),
		"objectWithSecretValue": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.NewStringProperty("bValue"),
			"c": resource.NewStringProperty("cValue"),
		}),
		"extraToValue": resource.NewStringProperty("extraToValue"),
	}

	annotateSecrets(to, from)

	for key, val := range to {
		fromVal, fromHas := from[key]
		if !fromHas {
			continue
		}

		assert.Truef(t, reflect.DeepEqual(fromVal, val), "expected properties %s to be deeply equal", key)
	}

	_, has := to["extraFromValue"]
	assert.Falsef(t, has, "to should not have a key named extraFromValue, it was not present before annotating secrets")

	_, has = to["extraToValue"]
	assert.True(t, has, "to should have a key named extraToValue, even though it was not in the from value")
}

func TestAnnotateSecretsArrays(t *testing.T) {
	from := resource.PropertyMap{
		"secretArray": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("b"),
			resource.NewStringProperty("c"),
		})),
		"arrayWithSecrets": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.MakeSecret(resource.NewStringProperty("b")),
			resource.NewStringProperty("c"),
		}),
	}

	to := resource.PropertyMap{
		"secretArray": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("b"),
			resource.NewStringProperty("c"),
		}),
		"arrayWithSecrets": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("c"),
			resource.NewStringProperty("b"),
		}),
	}

	expected := resource.PropertyMap{
		"secretArray": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("b"),
			resource.NewStringProperty("c"),
		})),
		"arrayWithSecrets": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("c"),
			resource.NewStringProperty("b"),
		})),
	}

	annotateSecrets(to, from)

	assert.Truef(t, reflect.DeepEqual(to, expected), "did not match expected after annotation")
}

func TestNestedSecret(t *testing.T) {
	from := resource.PropertyMap{
		"secretString": resource.MakeSecret(resource.NewStringProperty("shh")),
		"secretArray": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("hello"),
			resource.MakeSecret(resource.NewStringProperty("shh")),
			resource.NewStringProperty("goodbye")}),
		"secretMap": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
			"b": resource.NewStringProperty("b"),
		})),
		"deepSecretMap": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
			"b": resource.MakeSecret(resource.NewStringProperty("b")),
		}),
	}

	to := resource.PropertyMap{
		"secretString": resource.NewStringProperty("shh"),
		"secretArray": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("shh"),
			resource.NewStringProperty("hello"),
			resource.NewStringProperty("goodbye")}),
		"secretMap": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
			"b": resource.NewStringProperty("b"),
		})),
		"deepSecretMap": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
			"b": resource.NewStringProperty("b"),
			// Note the additional property here, which we expect to be kept when annotating.
			"c": resource.NewStringProperty("c"),
		}),
	}

	expected := resource.PropertyMap{
		"secretString": resource.MakeSecret(resource.NewStringProperty("shh")),
		// The entire array has been marked a secret because it contained a secret member in from. Since arrays
		// are often used for sets, we didn't try to apply the secretness to a specific member of the array, like
		// we would have with maps (where we can use the keys to correlate related properties)
		"secretArray": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("shh"),
			resource.NewStringProperty("hello"),
			resource.NewStringProperty("goodbye")})),
		"secretMap": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
			"b": resource.NewStringProperty("b"),
		})),
		"deepSecretMap": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
			"b": resource.MakeSecret(resource.NewStringProperty("b")),
			"c": resource.NewStringProperty("c"),
		}),
	}

	annotateSecrets(to, from)

	assert.Truef(t, reflect.DeepEqual(to, expected), "did not match expected after annotation")
}

func TestInvokeWithUnknowns(t *testing.T) {
	// Create an example with list-nested unknowns that used to panic.
	proto := pbStruct("resources", pbList(pbString(UnknownStringValue)))
	propsWithUnknowns, err := UnmarshalProperties(proto, MarshalOptions{KeepUnknowns: true})
	if err != nil {
		t.Error(err)
	}

	// Invoke dynamically executes a built-in function in the provider.
	p := initProviderForTesting(t, &TestInvokeWithUnknownsClient{})
	resultPropertyMap, failures, err := p.Invoke("a:b:c", propsWithUnknowns)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, 0, len(failures))
	assert.Equal(t, 0, len(resultPropertyMap))
}

type TestInvokeWithUnknownsClient struct {
	t *testing.T
	UnimplementedResourceProviderClient
}

func (*TestInvokeWithUnknownsClient) Configure(ctx context.Context, in *pulumirpc.ConfigureRequest, opts ...grpc.CallOption) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{}, nil
}

func (m *TestInvokeWithUnknownsClient) Invoke(ctx context.Context, in *pulumirpc.InvokeRequest, opts ...grpc.CallOption) (*pulumirpc.InvokeResponse, error) {
	_, err := UnmarshalProperties(in.GetArgs(), MarshalOptions{})
	if err != nil {
		return nil, err
	}
	return &pulumirpc.InvokeResponse{}, nil
}

var _ pulumirpc.ResourceProviderClient = &TestInvokeWithUnknownsClient{}

func initProviderForTesting(t *testing.T, client pulumirpc.ResourceProviderClient) *provider {
	p := &provider{
		cfgdone:   make(chan bool),
		clientRaw: client,
	}
	err := p.Configure(resource.PropertyMap{})
	if err != nil {
		t.Error(err)
		return nil
	}
	return p
}

// Boilerplate to simplify implementing a large interface. This
// probably needs to move somewhere else.

type UnimplementedResourceProviderClient struct{}

func (*UnimplementedResourceProviderClient) GetSchema(ctx context.Context, in *pulumirpc.GetSchemaRequest, opts ...grpc.CallOption) (*pulumirpc.GetSchemaResponse, error) {
	return nil, fmt.Errorf("Not implemented: GetSchema")
}

func (*UnimplementedResourceProviderClient) CheckConfig(ctx context.Context, in *pulumirpc.CheckRequest, opts ...grpc.CallOption) (*pulumirpc.CheckResponse, error) {
	return nil, fmt.Errorf("Not implemented: CheckConfig")
}

func (*UnimplementedResourceProviderClient) DiffConfig(ctx context.Context, in *pulumirpc.DiffRequest, opts ...grpc.CallOption) (*pulumirpc.DiffResponse, error) {
	return nil, fmt.Errorf("Not implemented: DiffConfig")
}

func (*UnimplementedResourceProviderClient) Configure(ctx context.Context, in *pulumirpc.ConfigureRequest, opts ...grpc.CallOption) (*pulumirpc.ConfigureResponse, error) {
	return nil, fmt.Errorf("Not implemented: Configure")
}

func (*UnimplementedResourceProviderClient) Invoke(ctx context.Context, in *pulumirpc.InvokeRequest, opts ...grpc.CallOption) (*pulumirpc.InvokeResponse, error) {
	return nil, fmt.Errorf("Not implemented: Invoke")
}

func (*UnimplementedResourceProviderClient) StreamInvoke(ctx context.Context, in *pulumirpc.InvokeRequest, opts ...grpc.CallOption) (pulumirpc.ResourceProvider_StreamInvokeClient, error) {
	return nil, fmt.Errorf("Not implemented: StreamInvoke")
}

func (*UnimplementedResourceProviderClient) Check(ctx context.Context, in *pulumirpc.CheckRequest, opts ...grpc.CallOption) (*pulumirpc.CheckResponse, error) {
	return nil, fmt.Errorf("Not implemented: Check")
}

func (*UnimplementedResourceProviderClient) Diff(ctx context.Context, in *pulumirpc.DiffRequest, opts ...grpc.CallOption) (*pulumirpc.DiffResponse, error) {
	return nil, fmt.Errorf("Not implemented: Diff")
}

func (*UnimplementedResourceProviderClient) Create(ctx context.Context, in *pulumirpc.CreateRequest, opts ...grpc.CallOption) (*pulumirpc.CreateResponse, error) {
	return nil, fmt.Errorf("Not implemented: Create")
}

func (*UnimplementedResourceProviderClient) Read(ctx context.Context, in *pulumirpc.ReadRequest, opts ...grpc.CallOption) (*pulumirpc.ReadResponse, error) {
	return nil, fmt.Errorf("Not implemented: Read")
}

func (*UnimplementedResourceProviderClient) Update(ctx context.Context, in *pulumirpc.UpdateRequest, opts ...grpc.CallOption) (*pulumirpc.UpdateResponse, error) {
	return nil, fmt.Errorf("Not implemented: Update")
}

func (*UnimplementedResourceProviderClient) Delete(ctx context.Context, in *pulumirpc.DeleteRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	return nil, fmt.Errorf("Not implemented: Delete")
}

func (*UnimplementedResourceProviderClient) Construct(ctx context.Context, in *pulumirpc.ConstructRequest, opts ...grpc.CallOption) (*pulumirpc.ConstructResponse, error) {
	return nil, fmt.Errorf("Not implemented: Construct")
}

func (*UnimplementedResourceProviderClient) Cancel(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error) {
	return nil, fmt.Errorf("Not implemented: Cancel")
}

func (*UnimplementedResourceProviderClient) GetPluginInfo(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*pulumirpc.PluginInfo, error) {
	return nil, fmt.Errorf("Not implemented: GetPluginInfo")
}

var _ pulumirpc.ResourceProviderClient = &UnimplementedResourceProviderClient{}

// Protobuf value constructor utilities

func pbString(x string) *structpb.Value {
	return MarshalString(x, MarshalOptions{})
}

func pbStruct(name string, value *structpb.Value) *structpb.Struct {
	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			name: value,
		},
	}
}

func pbList(elems ...*structpb.Value) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_ListValue{
			ListValue: &structpb.ListValue{Values: elems},
		},
	}
}
