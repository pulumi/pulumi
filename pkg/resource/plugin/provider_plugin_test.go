// Copyright 2019, Pulumi Corporation.
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
	"io"
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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/pulumi/pulumi/sdk/v3/go/propertyrpc"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestAnnotateSecrets(t *testing.T) {
	t.Parallel()

	secret := func(v property.Value) property.Value { return v.WithSecret(true) }

	from := property.NewMap(map[string]property.Value{
		"stringValue": secret(property.New("hello")),
		"numberValue": secret(property.New(1.00)),
		"boolValue":   secret(property.New(true)),
		"secretArrayValue": secret(property.New([]property.Value{
			property.New("a"),
			property.New("b"),
			property.New("c"),
		})),
		"secretObjectValue": secret(property.New(map[string]property.Value{
			"a": property.New("aValue"),
			"b": property.New("bValue"),
			"c": property.New("cValue"),
		})),
		"objectWithSecretValue": property.New(map[string]property.Value{
			"a": property.New("aValue"),
			"b": secret(property.New("bValue")),
			"c": property.New("cValue"),
		}),
	})

	to := property.NewMap(map[string]property.Value{
		"stringValue": property.New("hello"),
		"numberValue": property.New(1.00),
		"boolValue":   property.New(true),
		"secretArrayValue": property.New([]property.Value{
			property.New("a"),
			property.New("b"),
			property.New("c"),
		}),
		"secretObjectValue": property.New(map[string]property.Value{
			"a": property.New("aValue"),
			"b": property.New("bValue"),
			"c": property.New("cValue"),
		}),
		"objectWithSecretValue": property.New(map[string]property.Value{
			"a": property.New("aValue"),
			"b": property.New("bValue"),
			"c": property.New("cValue"),
		}),
	})

	got := annotateSecrets(to, from)

	assert.Truef(t, got.Equals(from), "objects should be equal")
}

func TestAnnotateSecretsDifferentProperties(t *testing.T) {
	t.Parallel()

	// ensure that if from and and to have different shapes, values on from are not put into to, values on to which
	// are not present in from stay in to, but any secretness is propigated for shared keys.

	secret := func(v property.Value) property.Value { return v.WithSecret(true) }

	from := property.NewMap(map[string]property.Value{
		"stringValue": secret(property.New("hello")),
		"numberValue": secret(property.New(1.00)),
		"boolValue":   secret(property.New(true)),
		"secretObjectValue": secret(property.New(map[string]property.Value{
			"a": property.New("aValue"),
			"b": property.New("bValue"),
			"c": property.New("cValue"),
		})),
		"objectWithSecretValue": property.New(map[string]property.Value{
			"a": property.New("aValue"),
			"b": secret(property.New("bValue")),
			"c": property.New("cValue"),
		}),
		"extraFromValue": property.New("extraFromValue"),
	})

	to := property.NewMap(map[string]property.Value{
		"stringValue": property.New("hello"),
		"numberValue": property.New(1.00),
		"boolValue":   property.New(true),
		"secretObjectValue": secret(property.New(map[string]property.Value{
			"a": property.New("aValue"),
			"b": property.New("bValue"),
			"c": property.New("cValue"),
		})),
		"objectWithSecretValue": property.New(map[string]property.Value{
			"a": property.New("aValue"),
			"b": property.New("bValue"),
			"c": property.New("cValue"),
		}),
		"extraToValue": property.New("extraToValue"),
	})

	got := annotateSecrets(to, from)

	got.All(func(key string, val property.Value) bool {
		fromVal, fromHas := from.GetOk(key)
		if !fromHas {
			return true
		}
		assert.Truef(t, fromVal.Equals(val), "expected properties %s to be equal", key)
		return true
	})

	_, has := got.GetOk("extraFromValue")
	assert.Falsef(t, has, "to should not have a key named extraFromValue, it was not present before annotating secrets")

	_, has = got.GetOk("extraToValue")
	assert.True(t, has, "to should have a key named extraToValue, even though it was not in the from value")
}

func TestAnnotateSecretsArrays(t *testing.T) {
	t.Parallel()

	secret := func(v property.Value) property.Value { return v.WithSecret(true) }

	from := property.NewMap(map[string]property.Value{
		"secretArray": secret(property.New([]property.Value{
			property.New("a"),
			property.New("b"),
			property.New("c"),
		})),
		"arrayWithSecrets": property.New([]property.Value{
			property.New("a"),
			secret(property.New("b")),
			property.New("c"),
		}),
	})

	to := property.NewMap(map[string]property.Value{
		"secretArray": property.New([]property.Value{
			property.New("a"),
			property.New("b"),
			property.New("c"),
		}),
		"arrayWithSecrets": property.New([]property.Value{
			property.New("a"),
			property.New("c"),
			property.New("b"),
		}),
	})

	expected := property.NewMap(map[string]property.Value{
		"secretArray": secret(property.New([]property.Value{
			property.New("a"),
			property.New("b"),
			property.New("c"),
		})),
		"arrayWithSecrets": secret(property.New([]property.Value{
			property.New("a"),
			property.New("c"),
			property.New("b"),
		})),
	})

	got := annotateSecrets(to, from)

	assert.Truef(t, got.Equals(expected), "did not match expected after annotation")
}

func TestNestedSecret(t *testing.T) {
	t.Parallel()

	secret := func(v property.Value) property.Value { return v.WithSecret(true) }

	from := property.NewMap(map[string]property.Value{
		"secretString": secret(property.New("shh")),
		"secretArray": property.New([]property.Value{
			property.New("hello"),
			secret(property.New("shh")),
			property.New("goodbye"),
		}),
		"secretMap": secret(property.New(map[string]property.Value{
			"a": property.New("a"),
			"b": property.New("b"),
		})),
		"deepSecretMap": property.New(map[string]property.Value{
			"a": property.New("a"),
			"b": secret(property.New("b")),
		}),
	})

	to := property.NewMap(map[string]property.Value{
		"secretString": property.New("shh"),
		"secretArray": property.New([]property.Value{
			property.New("shh"),
			property.New("hello"),
			property.New("goodbye"),
		}),
		"secretMap": secret(property.New(map[string]property.Value{
			"a": property.New("a"),
			"b": property.New("b"),
		})),
		"deepSecretMap": property.New(map[string]property.Value{
			"a": property.New("a"),
			"b": property.New("b"),
			// Note the additional property here, which we expect to be kept when annotating.
			"c": property.New("c"),
		}),
	})

	expected := property.NewMap(map[string]property.Value{
		"secretString": secret(property.New("shh")),
		// The entire array has been marked a secret because it contained a secret member in from. Since arrays
		// are often used for sets, we didn't try to apply the secretness to a specific member of the array, like
		// we would have with maps (where we can use the keys to correlate related properties)
		"secretArray": secret(property.New([]property.Value{
			property.New("shh"),
			property.New("hello"),
			property.New("goodbye"),
		})),
		"secretMap": secret(property.New(map[string]property.Value{
			"a": property.New("a"),
			"b": property.New("b"),
		})),
		"deepSecretMap": property.New(map[string]property.Value{
			"a": property.New("a"),
			"b": secret(property.New("b")),
			"c": property.New("c"),
		}),
	})

	got := annotateSecrets(to, from)

	assert.Truef(t, got.Equals(expected), "did not match expected after annotation")
}

func TestRestoreElidedAssetContents(t *testing.T) {
	t.Parallel()
	textAsset := func(text string) property.Value {
		a, err := asset.FromText(text)
		require.NoError(t, err)
		return property.New(a)
	}
	secret := func(v property.Value) property.Value { return v.WithSecret(true) }

	original := property.NewMap(map[string]property.Value{
		"source": textAsset("Hello world"),
		"nested": property.New(map[string]property.Value{
			"another":      textAsset("Another"),
			"doubleNested": textAsset("Double nested"),
			"tripleNested": property.New(map[string]property.Value{
				"secret": secret(textAsset("Secret content")),
			}),
		}),
		"insideArray": property.New([]property.Value{
			textAsset("First"),
			textAsset("Second"),
			property.New(map[string]property.Value{
				"nestedArray": property.New([]property.Value{
					textAsset("Nested array"),
					secret(textAsset("another secret content")),
				}),
			}),
		}),
	})

	filtered, err := FilterProperties(original, FilterOptions{
		ElideAssetContents: true,
		KeepSecrets:        true,
	})
	require.NoError(t, err, "failed to filter properties")
	serialized := propertyrpc.Marshal(filtered)

	raw, err := propertyrpc.Unmarshal(serialized)
	require.NoError(t, err, "failed to unmarshal properties")
	deserialized, err := FilterProperties(raw, FilterOptions{KeepSecrets: true})
	require.NoError(t, err, "failed to filter unmarshaled properties")

	// the deserialized properties are not the same as the original, because during marshalling
	// we skipped the contents of assets with the option `ElideAssetContents` set to true.
	assert.Equal(t, "Hello world", original.Get("source").AsAsset().Text)
	assert.Empty(t, deserialized.Get("source").AsAsset().Text,
		"expected elided deserialization to have empty asset text")

	// but if we restore the elided contents, we should get the original properties back.
	restored := restoreElidedAssetContents(original, deserialized)
	assert.Equal(t, "Hello world", restored.Get("source").AsAsset().Text)
	assert.Equal(t, "Another",
		restored.Get("nested").AsMap().Get("another").AsAsset().Text)
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
				Inputs:  property.Map{},
				Outputs: property.Map{},
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
				Inputs: property.NewMap(map[string]property.Value{
					"foo": property.New("bar"),
				}),
				Outputs: property.Map{},
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
				Inputs: property.Map{},
				Outputs: property.NewMap(map[string]property.Value{
					"baz": property.New("quux"),
				}),
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
				Inputs:  property.Map{},
				Outputs: property.Map{},
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
				Inputs: property.NewMap(map[string]property.Value{
					"foo": property.New("bar"),
				}),
				Outputs: property.NewMap(map[string]property.Value{
					"baz": property.New("quux"),
				}),
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

			p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)

			// We have to configure before we can use Delete.
			_, err := p.Configure(t.Context(), ConfigureRequest{Type: new(tokens.Type("pulumi:providers:test"))})
			require.NoError(t, err, "Configure failed")

			// Act.
			_, err = p.Delete(t.Context(), tt.give)
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
				Aliases: []*pulumirpc.Alias{
					{Alias: &pulumirpc.Alias_Urn{Urn: "urn:pulumi:stack::project::type::oldName"}},
					{Alias: &pulumirpc.Alias_Urn{Urn: "urn:pulumi:stack::project::type::anotherOldName"}},
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
				Protect: new(true),
			},
			want: &pulumirpc.ConstructRequest{
				Protect: new(true),
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
				DeleteBeforeReplace: new(true),
			},
			want: &pulumirpc.ConstructRequest{
				DeleteBeforeReplace: new(true),
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
			desc: "replacement trigger",
			give: ConstructOptions{
				ReplacementTrigger: property.New("trigger-value"),
			},
			want: &pulumirpc.ConstructRequest{
				ReplacementTrigger: structpb.NewStringValue("trigger-value"),
			},
		},
		{
			desc: "retain on delete",
			give: ConstructOptions{
				RetainOnDelete: new(true),
			},
			want: &pulumirpc.ConstructRequest{
				RetainOnDelete: new(true),
			},
		},
		{
			desc: "resource hooks",
			give: ConstructOptions{
				ResourceHooks: map[resource.HookType][]string{
					resource.BeforeCreate: {"before-create"},
					resource.AfterCreate:  {"after-create"},
					resource.BeforeUpdate: {"before-update"},
					resource.AfterUpdate:  {"after-update"},
					resource.BeforeDelete: {"before-delete"},
					resource.AfterDelete:  {"after-delete"},
					resource.OnError:      {"on-error-1", "on-error-2"},
				},
			},
			want: &pulumirpc.ConstructRequest{
				ResourceHooks: &pulumirpc.ConstructRequest_ResourceHooksBinding{
					BeforeCreate: []string{"before-create"},
					AfterCreate:  []string{"after-create"},
					BeforeUpdate: []string{"before-update"},
					AfterUpdate:  []string{"after-update"},
					BeforeDelete: []string{"before-delete"},
					AfterDelete:  []string{"after-delete"},
					OnError:      []string{"on-error-1", "on-error-2"},
				},
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
			tt.want.Organization = "organization"
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
					nilIfEmpty(&req.ReplaceWith)

					got = req
					return &pulumirpc.ConstructResponse{
						Urn: "urn:pulumi:stack::project::type::name",
					}, nil
				},
			}

			p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)

			// Must configure before we can use Construct.
			_, err := p.Configure(t.Context(), ConfigureRequest{Type: new(tokens.Type("pulumi:providers:test"))})
			require.NoError(t, err, "configure failed")

			_, err = p.Construct(t.Context(),
				ConstructRequest{
					Info:    ConstructInfo{Project: "project", Stack: "stack", Organization: "organization"},
					Type:    "type",
					Name:    "name",
					Parent:  tt.parent,
					Inputs:  property.Map{},
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

	p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)

	props := property.NewMap(map[string]property.Value{
		"foo": property.New("bar").WithSecret(true),
	})

	// Signal to specify that the Delete request was sent
	// and we should Configure now.
	deleting := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)

		close(deleting)
		_, err := p.Delete(t.Context(), DeleteRequest{
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
	_, err := p.Configure(t.Context(), ConfigureRequest{
		Type:   new(tokens.Type("pulumi:providers:test")),
		Inputs: props,
	})
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
		t.Context(),
		sink, sink,
		// The tests using this context wire up in-process stub clients, so the host is never used.
		&MockHost{}, nil /* source */, cwd, nil /* options */, false, nil, nil /* span */)
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
	ListF          func(context.Context, *pulumirpc.ListRequest) (pulumirpc.ResourceProvider_ListClient, error)
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

func (c *stubClient) List(
	ctx context.Context,
	req *pulumirpc.ListRequest,
	opts ...grpc.CallOption,
) (pulumirpc.ResourceProvider_ListClient, error) {
	if f := c.ListF; f != nil {
		return f(ctx, req)
	}
	return c.ResourceProviderClient.List(ctx, req, opts...)
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

// stubListStream is a fake [pulumirpc.ResourceProvider_ListClient] that drains a pre-populated slice of responses.
// It only implements the Recv method exercised by [provider.List]; other grpc.ClientStream methods panic on the
// embedded nil interface if invoked.
type stubListStream struct {
	grpc.ClientStream

	responses []*pulumirpc.ListResponse
	err       error
}

func (s *stubListStream) Recv() (*pulumirpc.ListResponse, error) {
	if len(s.responses) == 0 {
		if s.err != nil {
			return nil, s.err
		}
		return nil, io.EOF
	}
	resp := s.responses[0]
	s.responses = s.responses[1:]
	return resp, nil
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

// drainListStream iterates stream.Items, collecting results into a slice and stopping at the first error.
func drainListStream(t *testing.T, stream *ListStream) []ListResult {
	t.Helper()
	var got []ListResult
	for item, err := range stream.Items {
		require.NoError(t, err)
		got = append(got, item)
	}
	return got
}

func TestProvider_List(t *testing.T) {
	t.Parallel()

	resultMsg := func(id, name string) *pulumirpc.ListResponse {
		return &pulumirpc.ListResponse{
			Response: &pulumirpc.ListResponse_Result_{
				Result: &pulumirpc.ListResponse_Result{Id: id, Name: name},
			},
		}
	}
	continuationMsg := func(token string) *pulumirpc.ListResponse {
		return &pulumirpc.ListResponse{
			Response: &pulumirpc.ListResponse_Continuation_{
				Continuation: &pulumirpc.ListResponse_Continuation{ContinuationToken: token},
			},
		}
	}
	computedMsg := &pulumirpc.ListResponse{
		Response: &pulumirpc.ListResponse_Computed_{
			Computed: &pulumirpc.ListResponse_Computed{},
		},
	}

	tests := []struct {
		desc                  string
		responses             []*pulumirpc.ListResponse
		wantResults           []ListResult
		wantComputed          bool
		wantContinuationToken string
	}{
		{
			desc: "single page",
			responses: []*pulumirpc.ListResponse{
				resultMsg("id-a", "alpha"),
				resultMsg("id-b", "beta"),
			},
			wantResults: []ListResult{
				{ID: "id-a", Name: "alpha"},
				{ID: "id-b", Name: "beta"},
			},
		},
		{
			desc: "page with continuation",
			responses: []*pulumirpc.ListResponse{
				resultMsg("id-a", "alpha"),
				continuationMsg("next-cursor"),
			},
			wantResults:           []ListResult{{ID: "id-a", Name: "alpha"}},
			wantContinuationToken: "next-cursor",
		},
		{
			desc:      "empty page",
			responses: nil,
		},
		{
			desc:         "computed",
			responses:    []*pulumirpc.ListResponse{computedMsg},
			wantComputed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			var got *pulumirpc.ListRequest
			client := &stubClient{
				ConfigureF: func(*pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
					return &pulumirpc.ConfigureResponse{AcceptSecrets: true}, nil
				},
				ListF: func(_ context.Context, req *pulumirpc.ListRequest) (pulumirpc.ResourceProvider_ListClient, error) {
					got = req
					return &stubListStream{responses: tt.responses}, nil
				},
			}

			p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
			_, err := p.Configure(t.Context(), ConfigureRequest{Type: new(tokens.Type("pulumi:providers:test"))})
			require.NoError(t, err)

			stream, err := p.List(t.Context(), ListRequest{
				Token:             tokens.Type("pkgA:index:Thing"),
				Limit:             10,
				PageSize:          5,
				ContinuationToken: "cursor",
			})
			require.NoError(t, err)

			gotResults := drainListStream(t, stream)
			assert.Equal(t, tt.wantResults, gotResults)
			assert.Equal(t, tt.wantComputed, stream.Computed)
			assert.Equal(t, tt.wantContinuationToken, stream.ContinuationToken)

			require.NotNil(t, got, "List was not called")
			assert.Equal(t, "pkgA:index:Thing", got.Token)
			assert.Equal(t, int64(10), got.Limit)
			assert.Equal(t, int64(5), got.PageSize)
			assert.Equal(t, "cursor", got.ContinuationToken)
		})
	}
}

func TestProvider_List_YieldsStreamError(t *testing.T) {
	t.Parallel()

	streamErr := status.Error(codes.Internal, "upstream blew up")
	client := &stubClient{
		ConfigureF: func(*pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{AcceptSecrets: true}, nil
		},
		ListF: func(context.Context, *pulumirpc.ListRequest) (pulumirpc.ResourceProvider_ListClient, error) {
			return &stubListStream{err: streamErr}, nil
		},
	}

	p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
	_, err := p.Configure(t.Context(), ConfigureRequest{Type: new(tokens.Type("pulumi:providers:test"))})
	require.NoError(t, err)

	stream, err := p.List(t.Context(), ListRequest{Token: "pkgA:index:Thing"})
	require.NoError(t, err)

	var gotErr error
	var gotItems []ListResult
	for item, err := range stream.Items {
		if err != nil {
			gotErr = err
			break
		}
		gotItems = append(gotItems, item)
	}
	require.Error(t, gotErr)
	assert.Contains(t, gotErr.Error(), "upstream blew up")
	assert.Empty(t, gotItems)
}

// TestProvider_List_EarlyBreakCancelsRPC asserts that breaking out of the Items range loop early cancels the
// context that was passed to the underlying gRPC call. Without that, the server stream would leak until the
// provider's request context tears down.
func TestProvider_List_EarlyBreakCancelsRPC(t *testing.T) {
	t.Parallel()

	resultMsg := func(id string) *pulumirpc.ListResponse {
		return &pulumirpc.ListResponse{
			Response: &pulumirpc.ListResponse_Result_{
				Result: &pulumirpc.ListResponse_Result{Id: id},
			},
		}
	}
	var rpcCtx context.Context
	client := &stubClient{
		ConfigureF: func(*pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{AcceptSecrets: true}, nil
		},
		ListF: func(ctx context.Context, _ *pulumirpc.ListRequest) (pulumirpc.ResourceProvider_ListClient, error) {
			rpcCtx = ctx
			return &stubListStream{responses: []*pulumirpc.ListResponse{
				resultMsg("id-a"), resultMsg("id-b"), resultMsg("id-c"),
			}}, nil
		},
	}

	p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
	_, err := p.Configure(t.Context(), ConfigureRequest{Type: new(tokens.Type("pulumi:providers:test"))})
	require.NoError(t, err)

	stream, err := p.List(t.Context(), ListRequest{Token: "pkgA:index:Thing"})
	require.NoError(t, err)
	require.NotNil(t, rpcCtx, "ListF should have been called")
	require.NoError(t, rpcCtx.Err(), "context should be live before iteration starts")

	for item := range stream.Items {
		_ = item
		break
	}

	assert.Error(t, rpcCtx.Err(), "context should be cancelled after early break")
	assert.ErrorIs(t, rpcCtx.Err(), context.Canceled)
}

// TestProvider_List_FullDrainCancelsRPC asserts that draining Items naturally (via EOF) also cancels the
// per-call context so gRPC resources are released.
func TestProvider_List_FullDrainCancelsRPC(t *testing.T) {
	t.Parallel()

	resultMsg := func(id string) *pulumirpc.ListResponse {
		return &pulumirpc.ListResponse{
			Response: &pulumirpc.ListResponse_Result_{
				Result: &pulumirpc.ListResponse_Result{Id: id},
			},
		}
	}
	var rpcCtx context.Context
	client := &stubClient{
		ConfigureF: func(*pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{AcceptSecrets: true}, nil
		},
		ListF: func(ctx context.Context, _ *pulumirpc.ListRequest) (pulumirpc.ResourceProvider_ListClient, error) {
			rpcCtx = ctx
			return &stubListStream{responses: []*pulumirpc.ListResponse{
				resultMsg("id-a"),
			}}, nil
		},
	}

	p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
	_, err := p.Configure(t.Context(), ConfigureRequest{Type: new(tokens.Type("pulumi:providers:test"))})
	require.NoError(t, err)

	stream, err := p.List(t.Context(), ListRequest{Token: "pkgA:index:Thing"})
	require.NoError(t, err)
	for item, err := range stream.Items {
		_ = item
		require.NoError(t, err)
	}
	assert.ErrorIs(t, rpcCtx.Err(), context.Canceled)
}

// TestProvider_List_StreamErrorCancelsRPC asserts that mid-stream errors also release the per-call context.
func TestProvider_List_StreamErrorCancelsRPC(t *testing.T) {
	t.Parallel()

	streamErr := status.Error(codes.Internal, "boom")
	var rpcCtx context.Context
	client := &stubClient{
		ConfigureF: func(*pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{AcceptSecrets: true}, nil
		},
		ListF: func(ctx context.Context, _ *pulumirpc.ListRequest) (pulumirpc.ResourceProvider_ListClient, error) {
			rpcCtx = ctx
			return &stubListStream{err: streamErr}, nil
		},
	}

	p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
	_, err := p.Configure(t.Context(), ConfigureRequest{Type: new(tokens.Type("pulumi:providers:test"))})
	require.NoError(t, err)

	stream, err := p.List(t.Context(), ListRequest{Token: "pkgA:index:Thing"})
	require.NoError(t, err)
	for _, err := range stream.Items {
		if err != nil {
			break
		}
	}
	assert.ErrorIs(t, rpcCtx.Err(), context.Canceled)
}

func TestProvider_List_StopsEarly(t *testing.T) {
	t.Parallel()

	resultMsg := func(id string) *pulumirpc.ListResponse {
		return &pulumirpc.ListResponse{
			Response: &pulumirpc.ListResponse_Result_{
				Result: &pulumirpc.ListResponse_Result{Id: id},
			},
		}
	}
	client := &stubClient{
		ConfigureF: func(*pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{AcceptSecrets: true}, nil
		},
		ListF: func(context.Context, *pulumirpc.ListRequest) (pulumirpc.ResourceProvider_ListClient, error) {
			return &stubListStream{responses: []*pulumirpc.ListResponse{
				resultMsg("id-a"), resultMsg("id-b"), resultMsg("id-c"),
			}}, nil
		},
	}

	p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
	_, err := p.Configure(t.Context(), ConfigureRequest{Type: new(tokens.Type("pulumi:providers:test"))})
	require.NoError(t, err)

	stream, err := p.List(t.Context(), ListRequest{Token: "pkgA:index:Thing"})
	require.NoError(t, err)

	var got []ListResult
	for item, err := range stream.Items {
		require.NoError(t, err)
		got = append(got, item)
		if len(got) == 1 {
			break
		}
	}
	require.Len(t, got, 1)
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
	az := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
	_, err := az.DiffConfig(t.Context(), DiffConfigRequest{
		resource.NewURN("org/proj/dev", "foo", "", "pulumi:providers:azure", "qux"),
		"",
		"",
		property.Map{},
		property.Map{},
		property.Map{},
		false,
		nil,
	})
	assert.ErrorContains(t, err, "failed to parse kubeconfig")

	// Test that the error from 14529 is ignored if reported by kubernetes
	k8s := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
	diff, err := k8s.DiffConfig(t.Context(), DiffConfigRequest{
		resource.NewURN("org/proj/dev", "foo", "", "pulumi:providers:kubernetes", "qux"),
		"",
		"",
		property.Map{},
		property.Map{},
		property.Map{},
		false,
		nil,
	})
	require.NoError(t, err)
	assert.Equal(t, DiffUnknown, diff.Changes)

	// Test that some other error is not ignored if reported by kubernetes
	diffErr = status.Errorf(codes.Unknown, "some other error")
	_, err = k8s.DiffConfig(t.Context(), DiffConfigRequest{
		resource.NewURN("org/proj/dev", "foo", "", "pulumi:providers:kubernetes", "qux"),
		"",
		"",
		property.Map{},
		property.Map{},
		property.Map{},
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

	prov := NewProviderWithVersionOverride(newTestContext(t), client, false /* disablePreview */, &version)
	resp, err := prov.GetPluginInfo(t.Context())
	require.NoError(t, err)
	require.Equal(t, &version, resp.Version)

	schema, err := prov.GetSchema(t.Context(), GetSchemaRequest{})
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

// Test that a gRPC error carrying an ErrorResourceInitFailed detail is translated into a
// partial-failure response (status, ID, live state) and an *InitError. The engine relies on this
// translation to run error hooks and to record the live state of partially-created resources.
func TestProvider_PartialFailure(t *testing.T) {
	t.Parallel()

	urn := resource.NewURN("org/proj/dev", "foo", "", "bar:baz", "qux")

	liveProperties := property.NewMap(map[string]property.Value{"out": property.New("live-output")})
	liveInputs := property.NewMap(map[string]property.Value{"in": property.New("live-input")})

	mliveProperties := propertyrpc.Marshal(liveProperties)
	mliveInputs := propertyrpc.Marshal(liveInputs)

	initFailed := func(reason string) error {
		detail := pulumirpc.ErrorResourceInitFailed{
			Id:                  "some-id",
			Properties:          mliveProperties,
			Inputs:              mliveInputs,
			Reasons:             []string{reason},
			RefreshBeforeUpdate: true,
		}
		return rpcerror.WithDetails(rpcerror.New(codes.Unknown, reason), &detail)
	}

	client := &stubClient{
		ConfigureF: func(req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{}, nil
		},
		CreateF: func(req *pulumirpc.CreateRequest) (*pulumirpc.CreateResponse, error) {
			return nil, initFailed("create issue")
		},
		ReadF: func(req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
			return nil, initFailed("read issue")
		},
		UpdateF: func(req *pulumirpc.UpdateRequest) (*pulumirpc.UpdateResponse, error) {
			return nil, initFailed("update issue")
		},
	}

	p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)

	_, err := p.Configure(t.Context(), ConfigureRequest{Type: new(tokens.Type("pulumi:providers:test"))})
	require.NoError(t, err, "configure failed")

	var initErr *InitError

	createResp, err := p.Create(t.Context(), CreateRequest{
		URN:        urn,
		Name:       urn.Name(),
		Type:       urn.Type(),
		Properties: property.Map{},
	})
	require.ErrorAs(t, err, &initErr, "expected an InitError")
	assert.Equal(t, []string{"create issue"}, initErr.Reasons)
	assert.Equal(t, CreateResponse{
		ID:                  "some-id",
		Properties:          liveProperties,
		Status:              resource.StatusPartialFailure,
		RefreshBeforeUpdate: true,
	}, createResp)

	readResp, err := p.Read(t.Context(), ReadRequest{
		URN:    urn,
		Name:   urn.Name(),
		Type:   urn.Type(),
		ID:     "some-id",
		Inputs: property.Map{},
		State:  property.Map{},
	})
	require.ErrorAs(t, err, &initErr, "expected an InitError")
	assert.Equal(t, []string{"read issue"}, initErr.Reasons)
	assert.Equal(t, ReadResponse{
		ReadResult: ReadResult{
			ID:                  "some-id",
			Inputs:              new(liveInputs),
			Outputs:             new(liveProperties),
			RefreshBeforeUpdate: true,
		},
		Status: resource.StatusPartialFailure,
	}, readResp)

	updateResp, err := p.Update(t.Context(), UpdateRequest{
		URN:        urn,
		Name:       urn.Name(),
		Type:       urn.Type(),
		ID:         "some-id",
		OldInputs:  property.Map{},
		OldOutputs: property.Map{},
		NewInputs:  property.Map{},
	})
	require.ErrorAs(t, err, &initErr, "expected an InitError")
	assert.Equal(t, []string{"update issue"}, initErr.Reasons)
	assert.Equal(t, UpdateResponse{
		Properties:          liveProperties,
		Status:              resource.StatusPartialFailure,
		RefreshBeforeUpdate: true,
	}, updateResp)
}

// Tests that assets echoed back by a provider are restored from the old state, not just the old inputs.
// Assets frequently live only in a resource's outputs, and eliding their contents on the way to the provider
// must not persist a contentless asset into the snapshot.
func TestProvider_RestoresElidedAssetsFromState(t *testing.T) {
	t.Parallel()

	urn := resource.NewURN("org/proj/dev", "foo", "", "test:index:Resource", "qux")

	textAsset, err := asset.FromText("Hello world")
	require.NoError(t, err)

	// The asset only ever appears in the outputs, never in the inputs.
	inputs := property.NewMap(map[string]property.Value{"assetPaths": property.New("build/**")})
	outputs := property.NewMap(map[string]property.Value{"assets": property.New(textAsset)})

	assertRestored := func(t *testing.T, got property.Map) {
		a := got.Get("assets").AsAsset()
		assert.Equal(t, textAsset.Hash, a.Hash)
		assert.True(t, a.HasContents(), "asset lost its contents")
		assert.Equal(t, "Hello world", a.Text)
	}

	newProvider := func(t *testing.T, client *stubClient) Provider {
		client.ConfigureF = func(req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{AcceptSecrets: true}, nil
		}
		p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
		_, err := p.Configure(t.Context(), ConfigureRequest{Type: new(tokens.Type("pulumi:providers:test"))})
		require.NoError(t, err, "Configure failed")
		return p
	}

	t.Run("read", func(t *testing.T) {
		t.Parallel()

		p := newProvider(t, &stubClient{
			ReadF: func(req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
				// Echo the state back, as a provider with no remote state to consult would.
				return &pulumirpc.ReadResponse{
					Id:         req.GetId(),
					Properties: req.GetProperties(),
					Inputs:     req.GetInputs(),
				}, nil
			},
		})

		resp, err := p.Read(t.Context(), ReadRequest{
			URN:    urn,
			ID:     "some-id",
			Inputs: inputs,
			State:  outputs,
		})
		require.NoError(t, err)
		require.NotNil(t, resp.Outputs)
		assertRestored(t, *resp.Outputs)
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()

		p := newProvider(t, &stubClient{
			UpdateF: func(req *pulumirpc.UpdateRequest) (*pulumirpc.UpdateResponse, error) {
				// Echo the old outputs back, as a provider that did not touch the asset would.
				return &pulumirpc.UpdateResponse{Properties: req.GetOlds()}, nil
			},
		})

		resp, err := p.Update(t.Context(), UpdateRequest{
			URN:        urn,
			ID:         "some-id",
			OldInputs:  inputs,
			OldOutputs: outputs,
			NewInputs:  inputs,
		})
		require.NoError(t, err)
		assertRestored(t, resp.Properties)
	})
}
