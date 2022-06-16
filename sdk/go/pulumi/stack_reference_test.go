package pulumi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestStackReference(t *testing.T) {
	t.Parallel()
	var resName string
	outputs := map[string]interface{}{
		"foo": "bar",
		"baz": []interface{}{"qux"},
		"zed": map[string]interface{}{
			"alpha": "beta",
		},
		"numf": 123.4,
		"numi": 567.0,
	}
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			assert.Equal(t, "pulumi:pulumi:StackReference", args.TypeToken)
			assert.Equal(t, resName, args.Name)
			assert.True(t, args.Inputs.DeepEquals(resource.NewPropertyMapFromMap(map[string]interface{}{
				"name": "stack",
			})))
			assert.Equal(t, "", args.Provider)
			assert.Equal(t, args.Inputs["name"].StringValue(), args.ID)
			return args.Inputs["name"].StringValue(), resource.NewPropertyMapFromMap(map[string]interface{}{
				"name":    "stack",
				"outputs": outputs,
			}), nil
		},
	}
	err := RunErr(func(ctx *Context) error {
		resName = "stack"
		ref0, err := NewStackReference(ctx, resName, nil)
		assert.NoError(t, err)
		_, _, _, _, err = await(ref0.ID())
		assert.NoError(t, err)
		resName = "stack2"
		ref1, err := NewStackReference(ctx, resName, &StackReferenceArgs{Name: String("stack")})
		assert.NoError(t, err)
		outs0, _, _, _, err := await(ref0.Outputs)
		assert.NoError(t, err)
		assert.Equal(t, outputs, outs0)
		zed0, _, _, _, err := await(ref0.GetOutput(String("zed")))
		assert.NoError(t, err)
		assert.Equal(t, outputs["zed"], zed0)
		outs1, _, _, _, err := await(ref1.Outputs)
		assert.NoError(t, err)
		assert.Equal(t, outputs, outs1)
		zed1, _, _, _, err := await(ref1.GetOutput(String("zed")))
		assert.NoError(t, err)
		assert.Equal(t, outputs["zed"], zed1)
		nonexistant, _, _, _, err := await(ref0.GetOutput(String("nonexistant")))
		assert.NoError(t, err)
		assert.Equal(t, nil, nonexistant)
		numf, _, _, _, err := await(ref1.GetFloat64Output(String("numf")))
		assert.NoError(t, err)
		assert.Equal(t, outputs["numf"], numf)
		_, _, _, _, err = await(ref1.GetFloat64Output(String("foo")))
		assert.Error(t, err)
		assert.Equal(t, fmt.Errorf("failed to convert %T to float64", outputs["foo"]), err)
		numi, _, _, _, err := await(ref1.GetIntOutput(String("numi")))
		assert.NoError(t, err)
		assert.Equal(t, int(outputs["numi"].(float64)), numi)
		_, _, _, _, err = await(ref1.GetIntOutput(String("foo")))
		assert.Error(t, err)
		assert.Equal(t, fmt.Errorf("failed to convert %T to int", outputs["foo"]), err)
		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)

	err = RunErr(func(ctx *Context) error {
		ctx.info.DryRun = true
		ref0, err := NewStackReference(ctx, resName, nil)
		assert.NoError(t, err)
		_, known, _, _, err := await(ref0.GetIntOutput(String("does-not-exist")))
		assert.NoError(t, err)
		assert.False(t, known)

		return nil
	}, WithMocks("project", "stack", &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return args.Inputs["name"].StringValue(), resource.NewPropertyMapFromMap(map[string]interface{}{
				"name":    "stack",
				"outputs": outputs,
			}), nil
		},
	}))
	assert.NoError(t, err)
}

func TestStackReferenceSecrets(t *testing.T) {
	t.Parallel()
	var resName string

	expected := map[string]interface{}{
		"foo": "bar",
		"baz": []interface{}{"qux"},
		"zed": map[string]interface{}{
			"alpha": "beta",
		},
		"numf": 123.4,
		"numi": 567.0,

		"secret-foo": "bar",
		"secret-baz": []interface{}{"qux"},
		"secret-zed": map[string]interface{}{
			"alpha": "beta",
		},
		"secret-numf": 123.4,
		"secret-numi": 567.0,
	}

	properties := resource.PropertyMap{}
	for k, v := range expected {
		v := resource.NewPropertyValue(v)
		if strings.HasPrefix(k, "secret-") {
			v = resource.MakeSecret(v)
		}
		properties[resource.PropertyKey(k)] = v
	}

	outputs := resource.NewObjectProperty(properties)

	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			assert.Equal(t, "pulumi:pulumi:StackReference", args.TypeToken)
			assert.Equal(t, resName, args.Name)
			assert.True(t, args.Inputs.DeepEquals(resource.NewPropertyMapFromMap(map[string]interface{}{
				"name": "stack",
			})))
			assert.Equal(t, "", args.Provider)
			assert.Equal(t, args.Inputs["name"].StringValue(), args.ID)
			return args.Inputs["name"].StringValue(), resource.PropertyMap{
				"name":    resource.NewStringProperty("stack"),
				"outputs": outputs,
			}, nil
		},
	}
	err := RunErr(func(ctx *Context) error {
		resName = "stack"
		ref0, err := NewStackReference(ctx, resName, nil)
		assert.NoError(t, err)
		_, _, _, _, err = await(ref0.ID())
		assert.NoError(t, err)
		resName = "stack2"
		ref1, err := NewStackReference(ctx, resName, &StackReferenceArgs{Name: String("stack")})
		assert.NoError(t, err)

		outs0, _, _, _, err := await(ref0.Outputs)
		assert.NoError(t, err)
		assert.Equal(t, expected, outs0)
		outs1, _, _, _, err := await(ref1.Outputs)
		assert.NoError(t, err)
		assert.Equal(t, expected, outs1)

		for _, ref := range []*StackReference{ref0, ref1} {
			for k, v := range expected {
				shouldSecret := strings.HasPrefix(k, "secret-")

				outputV, known, secret, _, err := await(ref.GetOutput(String(k)))
				assert.NoError(t, err)
				assert.True(t, known)
				assert.Equal(t, shouldSecret, secret)
				assert.Equal(t, v, outputV)
			}

			outputV, known, secret, _, err := await(ref.GetOutput(String("nonexistant-key")))
			assert.NoError(t, err)
			assert.True(t, known)
			assert.Equal(t, false, secret)
			assert.Equal(t, nil, outputV)
		}

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)
}
