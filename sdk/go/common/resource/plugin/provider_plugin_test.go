package plugin

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
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

func TestPluginConfigPromise(t *testing.T) {
	t.Parallel()

	t.Run("many gets", func(t *testing.T) {
		t.Parallel()

		prom := newPluginConfigPromise()
		ctx := context.Background()

		cfg := pluginConfig{
			known:           true,
			acceptSecrets:   true,
			acceptResources: true,
			acceptOutputs:   true,
			supportsPreview: true,
		}

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				got, err := prom.Await(ctx)
				assert.NoError(t, err)
				assert.Equal(t, cfg, got)
			}()
		}

		prom.Fulfill(cfg, nil)
		wg.Wait()
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		giveErr := errors.New("great sadness")
		prom := newPluginConfigPromise()
		ctx := context.Background()

		done := make(chan struct{})
		go func() {
			defer close(done)

			_, err := prom.Await(ctx)
			assert.ErrorIs(t, err, giveErr)
		}()

		prom.Fulfill(pluginConfig{}, giveErr)
		<-done
	})

	t.Run("set twice", func(t *testing.T) {
		t.Parallel()

		prom := newPluginConfigPromise()
		ctx := context.Background()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()

			got, err := prom.Await(ctx)
			assert.NoError(t, err)
			assert.Equal(t, pluginConfig{acceptSecrets: true}, got)
		}()

		prom.Fulfill(pluginConfig{acceptSecrets: true}, nil)
		prom.Fulfill(pluginConfig{acceptOutputs: true}, errors.New("ignored"))

		// Should still see the first configuration.
		got, err := prom.Await(ctx)
		assert.NoError(t, err)
		assert.Equal(t, pluginConfig{acceptSecrets: true}, got)

		wg.Wait()
	})

	t.Run("await cancelled", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		prom := newPluginConfigPromise()
		_, err := prom.Await(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
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

	cwd, err := os.Getwd()
	require.NoError(t, err)

	testw := iotest.LogWriter(t)
	sink := diag.DefaultSink(
		testw,
		testw,
		diag.FormatOptions{
			Color: colors.Never,
		},
	)
	ctx, err := NewContext(sink, sink, nil /* host */, nil /* source */, cwd, nil /* options */, false, nil /* span */)
	require.NoError(t, err)

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

	p := NewProviderWithClient(ctx, "foo", client, false)

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

type stubClient struct {
	pulumirpc.ResourceProviderClient

	ConfigureF func(*pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error)
	DeleteF    func(*pulumirpc.DeleteRequest) error
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
