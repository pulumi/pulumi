package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestAnnotateSecrets(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	from := resource.PropertyMap{
		"secretString": resource.MakeSecret(resource.NewStringProperty("shh")),
		"secretArray": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("hello"),
			resource.MakeSecret(resource.NewStringProperty("shh")),
			resource.NewStringProperty("goodbye"),
		}),
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
			resource.NewStringProperty("goodbye"),
		}),
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
			resource.NewStringProperty("goodbye"),
		})),
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

func TestRestoreElidedAssetContents(t *testing.T) {
	t.Parallel()
	textAsset := func(text string) resource.PropertyValue {
		asset, err := resource.NewTextAsset(text)
		require.NoError(t, err)
		return resource.NewAssetProperty(asset)
	}

	original := resource.PropertyMap{
		"source": textAsset("Hello world"),
		"nested": resource.NewObjectProperty(resource.PropertyMap{
			"another":      textAsset("Another"),
			"doubleNested": textAsset("Double nested"),
			"tripleNested": resource.NewObjectProperty(resource.PropertyMap{
				"secret": resource.MakeSecret(textAsset("Secret content")),
			}),
		}),
		"insideArray": resource.NewArrayProperty([]resource.PropertyValue{
			textAsset("First"),
			textAsset("Second"),
			resource.NewObjectProperty(resource.PropertyMap{
				"nestedArray": resource.NewArrayProperty([]resource.PropertyValue{
					textAsset("Nested array"),
					resource.MakeSecret(textAsset("another secret content")),
				}),
			}),
		}),
	}

	serialized, err := MarshalProperties(original, MarshalOptions{
		ElideAssetContents: true,
		KeepSecrets:        true,
	})
	require.NoError(t, err, "failed to marshal properties")

	deserialized, err := UnmarshalProperties(serialized, MarshalOptions{
		KeepSecrets: true,
	})
	require.NoError(t, err, "failed to unmarshal properties")

	originalRaw := original.Mappable()
	deserializedRaw := deserialized.Mappable()

	// the deserialized properties are not the same as the original, because during marshalling
	// we skipped the contents of assets with the option `ElideAssetContents` set to true.
	assert.NotEqual(t, originalRaw, deserializedRaw)

	// but if we restore the elided contents, we should get the original properties back.
	restoreElidedAssetContents(original, deserialized)
	deserializedRaw = deserialized.Mappable()
	assert.Equal(t, originalRaw, deserializedRaw)
}

func TestProvider_ConstructOptions(t *testing.T) {
	t.Parallel()

	// Helper to keep a some test cases simple.
	// Takes a pointer to a container (slice or map)
	// and sets it to nil if it's empty.
	nilIfEmpty := func(s any) {
		// The code below is roughly equivalent to:
		//	if len(*s) == 0 {
		//		*s = nil
		//	}
		v := reflect.ValueOf(s) // *T for some T = []T or map[T]*
		v = v.Elem()            // *T -> T
		if v.Len() == 0 {
			// Zero value of a slice or map is nil.
			v.Set(reflect.Zero(v.Type()))
		}
	}

	tests := []struct {
		desc   string
		give   ConstructOptions
		want   *pulumirpc.ConstructRequest
		parent resource.URN
	}{
		{
			desc: "empty",
			want: &pulumirpc.ConstructRequest{},
		},
		{
			desc: "aliases",
			give: ConstructOptions{
				Aliases: []resource.Alias{
					{URN: resource.URN("urn:pulumi:stack::project::type::oldName")},
					{URN: resource.URN("urn:pulumi:stack::project::type::anotherOldName")},
				},
			},
			want: &pulumirpc.ConstructRequest{
				Aliases: []string{
					"urn:pulumi:stack::project::type::oldName",
					"urn:pulumi:stack::project::type::anotherOldName",
				},
			},
		},
		{
			desc: "dependencies",
			give: ConstructOptions{
				Dependencies: []resource.URN{
					"urn:pulumi:stack::project::type::dep1",
					"urn:pulumi:stack::project::type::dep2",
				},
			},
			want: &pulumirpc.ConstructRequest{
				Dependencies: []string{
					"urn:pulumi:stack::project::type::dep1",
					"urn:pulumi:stack::project::type::dep2",
				},
			},
		},
		{
			desc: "protect",
			give: ConstructOptions{
				Protect: true,
			},
			want: &pulumirpc.ConstructRequest{
				Protect: true,
			},
		},
		{
			desc: "providers",
			give: ConstructOptions{
				Providers: map[string]string{
					"pkg1": "prov1",
					"pkg2": "prov2",
				},
			},
			want: &pulumirpc.ConstructRequest{
				Providers: map[string]string{
					"pkg1": "prov1",
					"pkg2": "prov2",
				},
			},
		},
		{
			desc: "property dependencies",
			give: ConstructOptions{
				PropertyDependencies: map[resource.PropertyKey][]resource.URN{
					"foo": {"urn:pulumi:stack::project::type::dep1"},
					"bar": {"urn:pulumi:stack::project::type::dep2"},
				},
			},
			want: &pulumirpc.ConstructRequest{
				InputDependencies: map[string]*pulumirpc.ConstructRequest_PropertyDependencies{
					"foo": {Urns: []string{"urn:pulumi:stack::project::type::dep1"}},
					"bar": {Urns: []string{"urn:pulumi:stack::project::type::dep2"}},
				},
			},
		},
		{
			desc: "additional secret outputs",
			give: ConstructOptions{
				AdditionalSecretOutputs: []string{"foo", "bar"},
			},
			want: &pulumirpc.ConstructRequest{
				AdditionalSecretOutputs: []string{"foo", "bar"},
			},
		},
		{
			desc: "custom timeouts",
			give: ConstructOptions{
				CustomTimeouts: &CustomTimeouts{
					Create: "1s",
					Update: "2s",
					Delete: "3s",
				},
			},
			want: &pulumirpc.ConstructRequest{
				CustomTimeouts: &pulumirpc.ConstructRequest_CustomTimeouts{
					Create: "1s",
					Update: "2s",
					Delete: "3s",
				},
			},
		},
		{
			desc: "deleted with",
			give: ConstructOptions{
				DeletedWith: "urn:pulumi:stack::project::type::dep1",
			},
			want: &pulumirpc.ConstructRequest{
				DeletedWith: "urn:pulumi:stack::project::type::dep1",
			},
		},
		{
			desc: "delete before replace",
			give: ConstructOptions{
				DeleteBeforeReplace: true,
			},
			want: &pulumirpc.ConstructRequest{
				DeleteBeforeReplace: true,
			},
		},
		{
			desc: "ignore changes",
			give: ConstructOptions{
				IgnoreChanges: []string{"foo", "bar"},
			},
			want: &pulumirpc.ConstructRequest{
				IgnoreChanges: []string{"foo", "bar"},
			},
		},
		{
			desc: "replace on changes",
			give: ConstructOptions{
				ReplaceOnChanges: []string{"foo", "bar"},
			},
			want: &pulumirpc.ConstructRequest{
				ReplaceOnChanges: []string{"foo", "bar"},
			},
		},
		{
			desc: "retain on delete",
			give: ConstructOptions{
				RetainOnDelete: true,
			},
			want: &pulumirpc.ConstructRequest{
				RetainOnDelete: true,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			// These values are the same for all test cases,
			// and are not affected by ConstructOptions.
			tt.want.Project = "project"
			tt.want.Stack = "stack"
			tt.want.Type = "type"
			tt.want.Name = "name"
			tt.want.Config = make(map[string]string)
			tt.want.Inputs = &structpb.Struct{Fields: make(map[string]*structpb.Value)}

			var got *pulumirpc.ConstructRequest
			client := &stubClient{
				ConfigureF: func(req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
					return &pulumirpc.ConfigureResponse{
						AcceptSecrets: true,
					}, nil
				},
				ConstructF: func(req *pulumirpc.ConstructRequest) (*pulumirpc.ConstructResponse, error) {
					// To keep test cases simple and avoid
					// having to duplicate empty slices for
					// them, nil out empty slices that are
					// otherwise always set.
					nilIfEmpty(&req.Aliases)
					nilIfEmpty(&req.ConfigSecretKeys)
					nilIfEmpty(&req.Dependencies)
					nilIfEmpty(&req.InputDependencies)

					got = req
					return &pulumirpc.ConstructResponse{
						Urn: "urn:pulumi:stack::project::type::name",
					}, nil
				},
			}

			p := NewProviderWithClient(newTestContext(t), "foo", client, false /* disablePreview */)

			// Must configure before we can use Construct.
			require.NoError(t, p.Configure(nil), "configure failed")

			_, err := p.Construct(
				ConstructInfo{Project: "project", Stack: "stack"},
				"type",
				"name",
				tt.parent,
				resource.PropertyMap{},
				tt.give,
			)
			require.NoError(t, err)

			require.NotNil(t, got, "Client.Construct was not called")
			assert.Equal(t, tt.want, got)
		})
	}
}

// This test detects a data race between Configure and Delete
// reported in https://github.com/pulumi/pulumi/issues/11971.
//
// The root cause of the data race was that
// Delete read properties from provider
// before they were set by Configure.
//
// To simulate the data race, we won't send the Configure request
// until after Delete.
func TestProvider_ConfigureDeleteRace(t *testing.T) {
	t.Parallel()

	var gotSecret *structpb.Value
	client := &stubClient{
		ConfigureF: func(req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{
				AcceptSecrets: true,
			}, nil
		},
		DeleteF: func(req *pulumirpc.DeleteRequest) error {
			gotSecret = req.Properties.Fields["foo"]
			return nil
		},
	}

	p := NewProviderWithClient(newTestContext(t), "foo", client, false /* disablePreview */)

	props := resource.PropertyMap{
		"foo": resource.NewSecretProperty(&resource.Secret{
			Element: resource.NewStringProperty("bar"),
		}),
	}

	// Signal to specify that the Delete request was sent
	// and we should Configure now.
	deleting := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)

		close(deleting)
		_, err := p.Delete(
			resource.NewURN("org/proj/dev", "foo", "", "bar:baz", "qux"),
			"whatever",
			props,
			props,
			1000,
		)
		assert.NoError(t, err, "Delete failed")
	}()

	// Wait until delete request has been sent to Configure
	// and then wait until Delete has finished.
	<-deleting
	assert.NoError(t, p.Configure(props))
	<-done

	s, ok := gotSecret.Kind.(*structpb.Value_StructValue)
	require.True(t, ok, "must be a strongly typed secret, got %v", gotSecret.Kind)
	assert.Equal(t, &structpb.Value_StringValue{
		StringValue: "bar",
	}, s.StructValue.Fields["value"].GetKind())
}

// newTestContext builds a *Context for use in tests.
func newTestContext(t testing.TB) *Context {
	t.Helper()

	cwd, err := os.Getwd()
	require.NoError(t, err, "get working directory")

	sink := diagtest.LogSink(t)
	ctx, err := NewContext(
		sink, sink,
		nil /* host */, nil /* source */, cwd, nil /* options */, false, nil /* span */)
	require.NoError(t, err, "build context")

	return ctx
}

type stubClient struct {
	pulumirpc.ResourceProviderClient

	DiffConfigF func(*pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error)
	ConstructF  func(*pulumirpc.ConstructRequest) (*pulumirpc.ConstructResponse, error)
	ConfigureF  func(*pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error)
	DeleteF     func(*pulumirpc.DeleteRequest) error
}

func (c *stubClient) DiffConfig(
	ctx context.Context,
	req *pulumirpc.DiffRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.DiffResponse, error) {
	if f := c.DiffConfigF; f != nil {
		return f(req)
	}
	return c.ResourceProviderClient.DiffConfig(ctx, req, opts...)
}

func (c *stubClient) Construct(
	ctx context.Context,
	req *pulumirpc.ConstructRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.ConstructResponse, error) {
	if f := c.ConstructF; f != nil {
		return f(req)
	}
	return c.ResourceProviderClient.Construct(ctx, req, opts...)
}

func (c *stubClient) Configure(
	ctx context.Context,
	req *pulumirpc.ConfigureRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.ConfigureResponse, error) {
	if f := c.ConfigureF; f != nil {
		return f(req)
	}
	return c.ResourceProviderClient.Configure(ctx, req, opts...)
}

func (c *stubClient) Delete(
	ctx context.Context,
	req *pulumirpc.DeleteRequest,
	opts ...grpc.CallOption,
) (*emptypb.Empty, error) {
	if f := c.DeleteF; f != nil {
		err := f(req)
		return &emptypb.Empty{}, err
	}
	return c.ResourceProviderClient.Delete(ctx, req, opts...)
}

// Test for https://github.com/pulumi/pulumi/issues/14529, ensure a kubernetes DiffConfig error is ignored
func TestKubernetesDiffError(t *testing.T) {
	t.Parallel()

	diffErr := status.Errorf(codes.Unknown, "failed to parse kubeconfig: %s",
		fmt.Errorf("couldn't get version/kind; json parse error: %w",
			errors.New("json: cannot unmarshal string into Go value of type struct "+
				"{ APIVersion string \"json:\\\"apiVersion,omitempty\\\"\"; Kind string \"json:\\\"kind,omitempty\\\"\" }")))

	client := &stubClient{
		DiffConfigF: func(req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
			return nil, diffErr
		},
	}

	// Test that the error from 14529 is NOT ignored if reported by something other than kubernetes
	az := NewProviderWithClient(newTestContext(t), "azure", client, false /* disablePreview */)
	_, err := az.DiffConfig(
		resource.NewURN("org/proj/dev", "foo", "", "pulumi:provider:azure", "qux"),
		resource.PropertyMap{}, resource.PropertyMap{}, resource.PropertyMap{},
		false, nil)
	assert.ErrorContains(t, err, "failed to parse kubeconfig")

	// Test that the error from 14529 is ignored if reported by kubernetes
	k8s := NewProviderWithClient(newTestContext(t), "kubernetes", client, false /* disablePreview */)
	diff, err := k8s.DiffConfig(
		resource.NewURN("org/proj/dev", "foo", "", "pulumi:provider:kubernetes", "qux"),
		resource.PropertyMap{}, resource.PropertyMap{}, resource.PropertyMap{},
		false, nil)
	assert.NoError(t, err)
	assert.Equal(t, DiffUnknown, diff.Changes)

	// Test that some other error is not ignored if reported by kubernetes
	diffErr = status.Errorf(codes.Unknown, "some other error")
	_, err = k8s.DiffConfig(
		resource.NewURN("org/proj/dev", "foo", "", "pulumi:provider:kubernetes", "qux"),
		resource.PropertyMap{}, resource.PropertyMap{}, resource.PropertyMap{},
		false, nil)
	assert.ErrorContains(t, err, "some other error")
}
