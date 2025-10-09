// Copyright 2019-2024, Pulumi Corporation.
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

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestAnnotateSecrets(t *testing.T) {
	t.Parallel()

	from := resource.PropertyMap{
		"stringValue": resource.MakeSecret(resource.NewProperty("hello")),
		"numberValue": resource.MakeSecret(resource.NewProperty(1.00)),
		"boolValue":   resource.MakeSecret(resource.NewProperty(true)),
		"secretArrayValue": resource.MakeSecret(resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("a"),
			resource.NewProperty("b"),
			resource.NewProperty("c"),
		})),
		"secretObjectValue": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty("aValue"),
			"b": resource.NewProperty("bValue"),
			"c": resource.NewProperty("cValue"),
		})),
		"objectWithSecretValue": resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty("aValue"),
			"b": resource.MakeSecret(resource.NewProperty("bValue")),
			"c": resource.NewProperty("cValue"),
		}),
	}

	to := resource.PropertyMap{
		"stringValue": resource.NewProperty("hello"),
		"numberValue": resource.NewProperty(1.00),
		"boolValue":   resource.NewProperty(true),
		"secretArrayValue": resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("a"),
			resource.NewProperty("b"),
			resource.NewProperty("c"),
		}),
		"secretObjectValue": resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty("aValue"),
			"b": resource.NewProperty("bValue"),
			"c": resource.NewProperty("cValue"),
		}),
		"objectWithSecretValue": resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty("aValue"),
			"b": resource.NewProperty("bValue"),
			"c": resource.NewProperty("cValue"),
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
		"stringValue": resource.MakeSecret(resource.NewProperty("hello")),
		"numberValue": resource.MakeSecret(resource.NewProperty(1.00)),
		"boolValue":   resource.MakeSecret(resource.NewProperty(true)),
		"secretObjectValue": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty("aValue"),
			"b": resource.NewProperty("bValue"),
			"c": resource.NewProperty("cValue"),
		})),
		"objectWithSecretValue": resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty("aValue"),
			"b": resource.MakeSecret(resource.NewProperty("bValue")),
			"c": resource.NewProperty("cValue"),
		}),
		"extraFromValue": resource.NewProperty("extraFromValue"),
	}

	to := resource.PropertyMap{
		"stringValue": resource.NewProperty("hello"),
		"numberValue": resource.NewProperty(1.00),
		"boolValue":   resource.NewProperty(true),
		"secretObjectValue": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty("aValue"),
			"b": resource.NewProperty("bValue"),
			"c": resource.NewProperty("cValue"),
		})),
		"objectWithSecretValue": resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty("aValue"),
			"b": resource.NewProperty("bValue"),
			"c": resource.NewProperty("cValue"),
		}),
		"extraToValue": resource.NewProperty("extraToValue"),
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
		"secretArray": resource.MakeSecret(resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("a"),
			resource.NewProperty("b"),
			resource.NewProperty("c"),
		})),
		"arrayWithSecrets": resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("a"),
			resource.MakeSecret(resource.NewProperty("b")),
			resource.NewProperty("c"),
		}),
	}

	to := resource.PropertyMap{
		"secretArray": resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("a"),
			resource.NewProperty("b"),
			resource.NewProperty("c"),
		}),
		"arrayWithSecrets": resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("a"),
			resource.NewProperty("c"),
			resource.NewProperty("b"),
		}),
	}

	expected := resource.PropertyMap{
		"secretArray": resource.MakeSecret(resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("a"),
			resource.NewProperty("b"),
			resource.NewProperty("c"),
		})),
		"arrayWithSecrets": resource.MakeSecret(resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("a"),
			resource.NewProperty("c"),
			resource.NewProperty("b"),
		})),
	}

	annotateSecrets(to, from)

	assert.Truef(t, reflect.DeepEqual(to, expected), "did not match expected after annotation")
}

func TestNestedSecret(t *testing.T) {
	t.Parallel()

	from := resource.PropertyMap{
		"secretString": resource.MakeSecret(resource.NewProperty("shh")),
		"secretArray": resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("hello"),
			resource.MakeSecret(resource.NewProperty("shh")),
			resource.NewProperty("goodbye"),
		}),
		"secretMap": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty("a"),
			"b": resource.NewProperty("b"),
		})),
		"deepSecretMap": resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty("a"),
			"b": resource.MakeSecret(resource.NewProperty("b")),
		}),
	}

	to := resource.PropertyMap{
		"secretString": resource.NewProperty("shh"),
		"secretArray": resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("shh"),
			resource.NewProperty("hello"),
			resource.NewProperty("goodbye"),
		}),
		"secretMap": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty("a"),
			"b": resource.NewProperty("b"),
		})),
		"deepSecretMap": resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty("a"),
			"b": resource.NewProperty("b"),
			// Note the additional property here, which we expect to be kept when annotating.
			"c": resource.NewProperty("c"),
		}),
	}

	expected := resource.PropertyMap{
		"secretString": resource.MakeSecret(resource.NewProperty("shh")),
		// The entire array has been marked a secret because it contained a secret member in from. Since arrays
		// are often used for sets, we didn't try to apply the secretness to a specific member of the array, like
		// we would have with maps (where we can use the keys to correlate related properties)
		"secretArray": resource.MakeSecret(resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("shh"),
			resource.NewProperty("hello"),
			resource.NewProperty("goodbye"),
		})),
		"secretMap": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty("a"),
			"b": resource.NewProperty("b"),
		})),
		"deepSecretMap": resource.NewProperty(resource.PropertyMap{
			"a": resource.NewProperty("a"),
			"b": resource.MakeSecret(resource.NewProperty("b")),
			"c": resource.NewProperty("c"),
		}),
	}

	annotateSecrets(to, from)

	assert.Truef(t, reflect.DeepEqual(to, expected), "did not match expected after annotation")
}

func TestRestoreElidedAssetContents(t *testing.T) {
	t.Parallel()
	textAsset := func(text string) resource.PropertyValue {
		asset, err := asset.FromText(text)
		require.NoError(t, err)
		return resource.NewProperty(asset)
	}

	original := resource.PropertyMap{
		"source": textAsset("Hello world"),
		"nested": resource.NewProperty(resource.PropertyMap{
			"another":      textAsset("Another"),
			"doubleNested": textAsset("Double nested"),
			"tripleNested": resource.NewProperty(resource.PropertyMap{
				"secret": resource.MakeSecret(textAsset("Secret content")),
			}),
		}),
		"insideArray": resource.NewProperty([]resource.PropertyValue{
			textAsset("First"),
			textAsset("Second"),
			resource.NewProperty(resource.PropertyMap{
				"nestedArray": resource.NewProperty([]resource.PropertyValue{
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

// Tests that Delete requests are correctly marshalled and sent to the engine.
func TestProvider_DeleteRequests(t *testing.T) {
	t.Parallel()

	// Arrange.
	id := resource.ID("foo")
	urn := resource.NewURN("org/proj/dev", "foo", "", "pulumi:provider:aws", "qux")

	tests := []struct {
		desc string
		give DeleteRequest
		want *pulumirpc.DeleteRequest
	}{
		{
			desc: "empty",
			give: DeleteRequest{
				ID:      id,
				URN:     urn,
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
			want: &pulumirpc.DeleteRequest{
				Id:         string(id),
				Urn:        string(urn),
				Name:       "qux",
				Type:       "pulumi:provider:aws",
				OldInputs:  &structpb.Struct{Fields: map[string]*structpb.Value{}},
				Properties: &structpb.Struct{Fields: map[string]*structpb.Value{}},
			},
		},
		{
			desc: "inputs",
			give: DeleteRequest{
				ID:  id,
				URN: urn,
				Inputs: resource.PropertyMap{
					"foo": resource.NewProperty("bar"),
				},
				Outputs: resource.PropertyMap{},
			},
			want: &pulumirpc.DeleteRequest{
				Id:   string(id),
				Urn:  string(urn),
				Name: "qux",
				Type: "pulumi:provider:aws",
				OldInputs: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"foo": {Kind: &structpb.Value_StringValue{StringValue: "bar"}},
					},
				},
				Properties: &structpb.Struct{Fields: map[string]*structpb.Value{}},
			},
		},
		{
			desc: "outputs",
			give: DeleteRequest{
				ID:     id,
				URN:    urn,
				Inputs: resource.PropertyMap{},
				Outputs: resource.PropertyMap{
					"baz": resource.NewProperty("quux"),
				},
			},
			want: &pulumirpc.DeleteRequest{
				Id:        string(id),
				Urn:       string(urn),
				Name:      "qux",
				Type:      "pulumi:provider:aws",
				OldInputs: &structpb.Struct{Fields: map[string]*structpb.Value{}},
				Properties: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"baz": {Kind: &structpb.Value_StringValue{StringValue: "quux"}},
					},
				},
			},
		},
		{
			desc: "timeout",
			give: DeleteRequest{
				ID:      id,
				URN:     urn,
				Timeout: 30,
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
			want: &pulumirpc.DeleteRequest{
				Id:         string(id),
				Urn:        string(urn),
				Name:       "qux",
				Type:       "pulumi:provider:aws",
				OldInputs:  &structpb.Struct{Fields: map[string]*structpb.Value{}},
				Properties: &structpb.Struct{Fields: map[string]*structpb.Value{}},
				Timeout:    30,
			},
		},
		{
			desc: "all",
			give: DeleteRequest{
				ID:  id,
				URN: urn,
				Inputs: resource.PropertyMap{
					"foo": resource.NewProperty("bar"),
				},
				Outputs: resource.PropertyMap{
					"baz": resource.NewProperty("quux"),
				},
				Timeout: 30,
			},
			want: &pulumirpc.DeleteRequest{
				Id:   string(id),
				Urn:  string(urn),
				Name: "qux",
				Type: "pulumi:provider:aws",
				OldInputs: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"foo": {Kind: &structpb.Value_StringValue{StringValue: "bar"}},
					},
				},
				Properties: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"baz": {Kind: &structpb.Value_StringValue{StringValue: "quux"}},
					},
				},
				Timeout: 30,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			var got *pulumirpc.DeleteRequest
			client := &stubClient{
				ConfigureF: func(req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
					return &pulumirpc.ConfigureResponse{
						AcceptSecrets: true,
					}, nil
				},
				DeleteF: func(req *pulumirpc.DeleteRequest) error {
					got = req
					return nil
				},
			}

			p := NewProviderWithClient(newTestContext(t), "pkgA", client, false /* disablePreview */)

			// We have to configure before we can use Delete.
			_, err := p.Configure(context.Background(), ConfigureRequest{})
			require.NoError(t, err, "Configure failed")

			// Act.
			_, err = p.Delete(context.Background(), tt.give)
			require.NoError(t, err)

			// Assert.
			require.NotNil(t, got, "Delete was not called")
			assert.Equal(t, tt.want, got)
		})
	}
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
	ptr := func(v bool) *bool {
		return &v
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
				Protect: ptr(true),
			},
			want: &pulumirpc.ConstructRequest{
				Protect: ptr(true),
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
				DeleteBeforeReplace: ptr(true),
			},
			want: &pulumirpc.ConstructRequest{
				DeleteBeforeReplace: ptr(true),
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
				RetainOnDelete: ptr(true),
			},
			want: &pulumirpc.ConstructRequest{
				RetainOnDelete: ptr(true),
			},
		},
	}

	for _, tt := range tests {
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
			tt.want.AcceptsOutputValues = true

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
			_, err := p.Configure(context.Background(), ConfigureRequest{})
			require.NoError(t, err, "configure failed")

			_, err = p.Construct(context.Background(),
				ConstructRequest{
					Info:    ConstructInfo{Project: "project", Stack: "stack"},
					Type:    "type",
					Name:    "name",
					Parent:  tt.parent,
					Inputs:  resource.PropertyMap{},
					Options: tt.give,
				},
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
		"foo": resource.NewProperty(&resource.Secret{
			Element: resource.NewProperty("bar"),
		}),
	}

	// Signal to specify that the Delete request was sent
	// and we should Configure now.
	deleting := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)

		close(deleting)
		_, err := p.Delete(context.Background(), DeleteRequest{
			URN:     resource.NewURN("org/proj/dev", "foo", "", "bar:baz", "qux"),
			Name:    "qux",
			Type:    "bar:baz",
			ID:      "whatever",
			Inputs:  props,
			Outputs: props,
			Timeout: 1000,
		})
		require.NoError(t, err, "Delete failed")
	}()

	// Wait until delete request has been sent to Configure
	// and then wait until Delete has finished.
	<-deleting
	_, err := p.Configure(context.Background(), ConfigureRequest{Inputs: props})
	require.NoError(t, err)
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
		context.Background(),
		sink, sink,
		nil /* host */, nil /* source */, cwd, nil /* options */, false, nil /* span */)
	require.NoError(t, err, "build context")

	return ctx
}

type stubClient struct {
	pulumirpc.ResourceProviderClient

	DiffConfigF    func(*pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error)
	ConstructF     func(*pulumirpc.ConstructRequest) (*pulumirpc.ConstructResponse, error)
	ConfigureF     func(*pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error)
	CreateF        func(*pulumirpc.CreateRequest) (*pulumirpc.CreateResponse, error)
	DeleteF        func(*pulumirpc.DeleteRequest) error
	GetSchemaF     func(*pulumirpc.GetSchemaRequest) (*pulumirpc.GetSchemaResponse, error)
	GetPluginInfoF func() (*pulumirpc.PluginInfo, error)
	ReadF          func(*pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error)
	UpdateF        func(*pulumirpc.UpdateRequest) (*pulumirpc.UpdateResponse, error)
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

func (c *stubClient) Create(
	ctx context.Context,
	req *pulumirpc.CreateRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.CreateResponse, error) {
	if f := c.CreateF; f != nil {
		return f(req)
	}
	return c.ResourceProviderClient.Create(ctx, req, opts...)
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

func (c *stubClient) GetSchema(
	ctx context.Context,
	req *pulumirpc.GetSchemaRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.GetSchemaResponse, error) {
	if f := c.GetSchemaF; f != nil {
		return f(req)
	}
	return c.ResourceProviderClient.GetSchema(ctx, req, opts...)
}

func (c *stubClient) GetPluginInfo(
	ctx context.Context,
	in *emptypb.Empty,
	opts ...grpc.CallOption,
) (*pulumirpc.PluginInfo, error) {
	if f := c.GetPluginInfoF; f != nil {
		return f()
	}
	return c.ResourceProviderClient.GetPluginInfo(ctx, in, opts...)
}

func (c *stubClient) Read(
	ctx context.Context,
	req *pulumirpc.ReadRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.ReadResponse, error) {
	if f := c.ReadF; f != nil {
		return f(req)
	}
	return c.ResourceProviderClient.Read(ctx, req, opts...)
}

func (c *stubClient) Update(
	ctx context.Context,
	req *pulumirpc.UpdateRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.UpdateResponse, error) {
	if f := c.UpdateF; f != nil {
		return f(req)
	}
	return c.ResourceProviderClient.Update(ctx, req, opts...)
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
	_, err := az.DiffConfig(context.Background(), DiffConfigRequest{
		resource.NewURN("org/proj/dev", "foo", "", "pulumi:provider:azure", "qux"),
		"",
		"",
		resource.PropertyMap{},
		resource.PropertyMap{},
		resource.PropertyMap{},
		false,
		nil,
	})
	assert.ErrorContains(t, err, "failed to parse kubeconfig")

	// Test that the error from 14529 is ignored if reported by kubernetes
	k8s := NewProviderWithClient(newTestContext(t), "kubernetes", client, false /* disablePreview */)
	diff, err := k8s.DiffConfig(context.Background(), DiffConfigRequest{
		resource.NewURN("org/proj/dev", "foo", "", "pulumi:provider:kubernetes", "qux"),
		"",
		"",
		resource.PropertyMap{},
		resource.PropertyMap{},
		resource.PropertyMap{},
		false,
		nil,
	})
	require.NoError(t, err)
	assert.Equal(t, DiffUnknown, diff.Changes)

	// Test that some other error is not ignored if reported by kubernetes
	diffErr = status.Errorf(codes.Unknown, "some other error")
	_, err = k8s.DiffConfig(context.Background(), DiffConfigRequest{
		resource.NewURN("org/proj/dev", "foo", "", "pulumi:provider:kubernetes", "qux"),
		"",
		"",
		resource.PropertyMap{},
		resource.PropertyMap{},
		resource.PropertyMap{},
		false,
		nil,
	})
	assert.ErrorContains(t, err, "some other error")
}

func TestOverrideVersion(t *testing.T) {
	t.Parallel()

	client := &stubClient{
		GetPluginInfoF: func() (*pulumirpc.PluginInfo, error) {
			return &pulumirpc.PluginInfo{
				Version: "0.0.0",
			}, nil
		},
		GetSchemaF: func(req *pulumirpc.GetSchemaRequest) (*pulumirpc.GetSchemaResponse, error) {
			schema := `{"name": "test", "version": "0.0.0"}`
			return &pulumirpc.GetSchemaResponse{
				Schema: schema,
			}, nil
		},
	}

	version := semver.MustParse("1.2.3")

	prov := NewProviderWithVersionOverride(newTestContext(t), "azure", client, false /* disablePreview */, &version)
	resp, err := prov.GetPluginInfo(context.Background())
	require.NoError(t, err)
	require.Equal(t, &version, resp.Version)

	schema, err := prov.GetSchema(context.Background(), GetSchemaRequest{})
	require.NoError(t, err)

	var unmarshalledSchema map[string]any
	err = json.Unmarshal(schema.Schema, &unmarshalledSchema)
	require.NoError(t, err)
	require.Equal(t, "1.2.3", unmarshalledSchema["version"])
}

//nolint:paralleltest // using t.Setenv which is incompatible with t.Parallel
func TestGetProviderAttachPort(t *testing.T) {
	t.Run("no attach", func(t *testing.T) {
		aws := tokens.Package("aws")
		port, err := GetProviderAttachPort(aws)
		require.NoError(t, err)
		require.Nil(t, port)
	})
	t.Run("aws:12345", func(t *testing.T) {
		t.Setenv("PULUMI_DEBUG_PROVIDERS", "aws:12345")
		aws := tokens.Package("aws")
		port, err := GetProviderAttachPort(aws)
		require.NoError(t, err)
		require.NotNil(t, port)
		require.Equal(t, 12345, *port)
	})
	t.Run("gcp:999,aws:12345", func(t *testing.T) {
		t.Setenv("PULUMI_DEBUG_PROVIDERS", "gcp:999,aws:12345")
		aws := tokens.Package("aws")
		port, err := GetProviderAttachPort(aws)
		require.NoError(t, err)
		require.NotNil(t, port)
		require.Equal(t, 12345, *port)
	})
	t.Run("gcp:999", func(t *testing.T) {
		t.Setenv("PULUMI_DEBUG_PROVIDERS", "gcp:999")
		aws := tokens.Package("aws")
		port, err := GetProviderAttachPort(aws)
		require.NoError(t, err)
		require.Nil(t, port)
	})
	t.Run("invalid", func(t *testing.T) {
		t.Setenv("PULUMI_DEBUG_PROVIDERS", "aws:port")
		aws := tokens.Package("aws")
		port, err := GetProviderAttachPort(aws)
		require.Error(t, err)
		require.Nil(t, port)
	})
}

func TestProvider_PartialFailure_RefreshBeforeUpdate(t *testing.T) {
	t.Parallel()

	urn := resource.NewURN("org/proj/dev", "foo", "", "bar:baz", "qux")

	client := &stubClient{
		ConfigureF: func(req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{}, nil
		},
		CreateF: func(req *pulumirpc.CreateRequest) (*pulumirpc.CreateResponse, error) {
			reasons := []string{"create issue"}
			detail := pulumirpc.ErrorResourceInitFailed{
				Id:                  "some-id",
				Reasons:             reasons,
				RefreshBeforeUpdate: true,
			}
			return nil, rpcerror.WithDetails(rpcerror.New(codes.Unknown, reasons[0]), &detail)
		},
		ReadF: func(req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
			reasons := []string{"read issue"}
			detail := pulumirpc.ErrorResourceInitFailed{
				Id:                  "some-id",
				Reasons:             reasons,
				RefreshBeforeUpdate: true,
			}
			return nil, rpcerror.WithDetails(rpcerror.New(codes.Unknown, reasons[0]), &detail)
		},
		UpdateF: func(req *pulumirpc.UpdateRequest) (*pulumirpc.UpdateResponse, error) {
			reasons := []string{"update issue"}
			detail := pulumirpc.ErrorResourceInitFailed{
				Id:                  "some-id",
				Reasons:             reasons,
				RefreshBeforeUpdate: true,
			}
			return nil, rpcerror.WithDetails(rpcerror.New(codes.Unknown, reasons[0]), &detail)
		},
	}

	p := NewProviderWithClient(newTestContext(t), "foo", client, false /* disablePreview */)

	_, err := p.Configure(context.Background(), ConfigureRequest{})
	require.NoError(t, err, "configure failed")

	var initErr *InitError

	createResp, err := p.Create(context.Background(), CreateRequest{
		URN:        urn,
		Name:       urn.Name(),
		Type:       urn.Type(),
		Properties: resource.PropertyMap{},
	})
	assert.True(t, createResp.RefreshBeforeUpdate)
	assert.ErrorAs(t, err, &initErr, "expected an InitError")
	assert.Equal(t, []string{"create issue"}, initErr.Reasons)

	readResp, err := p.Read(context.Background(), ReadRequest{
		URN:    urn,
		Name:   urn.Name(),
		Type:   urn.Type(),
		ID:     "some-id",
		Inputs: resource.PropertyMap{},
		State:  resource.PropertyMap{},
	})
	assert.True(t, readResp.RefreshBeforeUpdate)
	assert.ErrorAs(t, err, &initErr, "expected an InitError")
	assert.Equal(t, []string{"read issue"}, initErr.Reasons)

	updateResp, err := p.Update(context.Background(), UpdateRequest{
		URN:        urn,
		Name:       urn.Name(),
		Type:       urn.Type(),
		ID:         "some-id",
		OldInputs:  resource.PropertyMap{},
		OldOutputs: resource.PropertyMap{},
		NewInputs:  resource.PropertyMap{},
	})
	assert.True(t, updateResp.RefreshBeforeUpdate)
	assert.ErrorAs(t, err, &initErr, "expected an InitError")
	assert.Equal(t, []string{"update issue"}, initErr.Reasons)
}
