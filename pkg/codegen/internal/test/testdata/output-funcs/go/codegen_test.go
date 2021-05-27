package codegentest

import (
	"fmt"
	//"context"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	//"sync"

	"github.com/stretchr/testify/assert"
	//"fmt"
	"testing"
	"time"
)

type mocks int

// Create the mock.
func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	panic("NewResource not supported")
}

func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {

	if args.Token == "madeup-package:codegentest:listStorageAccountKeys" {
		inputs := make(map[string]string)
		for k, v := range args.Args {
			inputs[fmt.Sprintf("%v", k)] = v.V.(string)
		}
		result := ListStorageAccountKeysResult{
			Keys: []map[string]string{inputs},
		}
		outputs := map[string]interface{}{
			"keys": result.Keys,
		}
		return resource.NewPropertyMapFromMap(outputs), nil
	}

	if args.Token == "madeup-package:codegentest:funcWithDefaultValue" ||
		args.Token == "madeup-package:codegentest:funcWithAllOptionalInputs" ||
		args.Token == "madeup-package:codegentest:funcWithListParam" ||
		args.Token == "madeup-package:codegentest:funcWithDictParam" {
		result := FuncWithDefaultValueResult{
			R: fmt.Sprintf("%v", args.Args),
		}
		outputs := map[string]interface{}{
			"r": result.R,
		}
		return resource.NewPropertyMapFromMap(outputs), nil
	}

	panic(fmt.Errorf("Unknown token: %s", args.Token))
}

func TestListStorageAccountKeysOutput(t *testing.T) {
	pulumiTest(t, func(ctx *pulumi.Context) error {
		output := ListStorageAccountKeysOutput(ctx, ListStorageAccountKeysOutputArgs{
			AccountName:       pulumi.String("my-account-name"),
			ResourceGroupName: pulumi.String("my-resource-group-name"),
		})

		keys := waitOut(t, output.Keys()).([]map[string]string)

		assert.Equal(t, 1, len(keys))
		assert.Equal(t, 2, len(keys[0]))
		assert.Equal(t, "my-account-name", keys[0]["accountName"])
		assert.Equal(t, "my-resource-group-name", keys[0]["resourceGroupName"])

		output = ListStorageAccountKeysOutput(ctx, ListStorageAccountKeysOutputArgs{
			AccountName:       pulumi.String("my-account-name"),
			ResourceGroupName: pulumi.String("my-resource-group-name"),
			Expand:            pulumi.String("my-expand"),
		})

		keys = waitOut(t, output.Keys()).([]map[string]string)

		assert.Equal(t, 1, len(keys))
		assert.Equal(t, 3, len(keys[0]))
		assert.Equal(t, "my-account-name", keys[0]["accountName"])
		assert.Equal(t, "my-resource-group-name", keys[0]["resourceGroupName"])
		assert.Equal(t, "my-expand", keys[0]["expand"])

		return nil
	})
}

// TODO: it seems that default values are not supported by Go codegen
// yet, hence we do not observe "B" populated to default at all here.
// This could be good to fix.
func TestFuncWithDefaultValueOutput(t *testing.T) {
	pulumiTest(t, func(ctx *pulumi.Context) error {
		output := FuncWithDefaultValueOutput(ctx, FuncWithDefaultValueOutputArgs{
			A: pulumi.String("my-a"),
		})
		r := waitOut(t, output.R())
		assert.Equal(t, "map[a:{my-a}]", r)
		return nil
	})
}

func TestFuncWithAllOptionalInputsOutput(t *testing.T) {
	pulumiTest(t, func(ctx *pulumi.Context) error {
		output := FuncWithAllOptionalInputsOutput(ctx, FuncWithAllOptionalInputsOutputArgs{
			A: pulumi.String("my-a"),
		})
		r := waitOut(t, output.R())
		assert.Equal(t, "map[a:{my-a}]", r)
		return nil
	})
}

func TestFuncWithListParamOutput(t *testing.T) {
	pulumiTest(t, func(ctx *pulumi.Context) error {
		output := FuncWithListParamOutput(ctx, FuncWithListParamOutputArgs{
			A: pulumi.StringArray{
				pulumi.String("my-a1"),
				pulumi.String("my-a2"),
				pulumi.String("my-a3"),
			},
		})
		r := waitOut(t, output.R())
		assert.Equal(t, "map[a:{[{my-a1} {my-a2} {my-a3}]}]", r)
		return nil
	})
}

func TestFuncWithDictParamOutput(t *testing.T) {
	pulumiTest(t, func(ctx *pulumi.Context) error {
		output := FuncWithDictParamOutput(ctx, FuncWithDictParamOutputArgs{
			A: pulumi.StringMap{
				"one": pulumi.String("1"),
				"two": pulumi.String("2"),
			},
		})
		r := waitOut(t, output.R())
		assert.Equal(t, "map[a:{map[one:{1} two:{2}]}]", r)
		return nil
	})
}

func pulumiTest(t *testing.T, testBody func(ctx *pulumi.Context) error) {
	err := pulumi.RunErr(testBody, pulumi.WithMocks("project", "stack", mocks(0)))
	assert.NoError(t, err)
}

func waitOut(t *testing.T, output pulumi.Output) interface{} {
	result, err := waitOutput(output, 1*time.Second)
	if err != nil {
		t.Error(err)
		return nil
	}
	return result
}

func waitOutput(output pulumi.Output, timeout time.Duration) (interface{}, error) {
	c := make(chan interface{}, 2)
	output.ApplyT(func(v interface{}) interface{} {
		c <- v
		return v
	})
	var timeoutMarker *int = new(int)
	go func() {
		time.Sleep(timeout)
		c <- timeoutMarker
	}()

	result := <-c
	if result == timeoutMarker {
		return nil, fmt.Errorf("Timed out waiting for pulumi.Output after %v", timeout)
	} else {
		return result, nil
	}
}
