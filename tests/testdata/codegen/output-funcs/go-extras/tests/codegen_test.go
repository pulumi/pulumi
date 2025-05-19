// Copyright 2016-2021, Pulumi Corporation.
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

package codegentest

import (
	"fmt"
	"output-funcs/mypkg"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type mocks int

// Create the mock.
func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	panic("NewResource not supported")
}

func (mocks) Invoke(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	switch args.Token {
	case "mypkg::listStorageAccountKeys":
		targs := mypkg.ListStorageAccountKeysArgs{}
		for k, v := range args.Args {
			switch k {
			case "accountName":
				targs.AccountName = v.StringValue()
			case "expand":
				expand := v.StringValue()
				targs.Expand = &expand
			case "resourceGroupName":
				targs.ResourceGroupName = v.StringValue()
			}
		}

		var expand string
		if targs.Expand != nil {
			expand = *targs.Expand
		}

		inputs := []mypkg.StorageAccountKeyResponse{
			{
				KeyName:     "key",
				Permissions: "permissions",
				Value: fmt.Sprintf("accountName=%v, resourceGroupName=%v, expand=%v",
					targs.AccountName,
					targs.ResourceGroupName,
					expand),
			},
		}
		result := mypkg.ListStorageAccountKeysResult{
			Keys: inputs,
		}
		outputs := map[string]interface{}{
			"keys": result.Keys,
		}
		invokeResponse := resource.NewPropertyMapFromMap(outputs)
		// turn every field into a secret
		for k, v := range invokeResponse {
			invokeResponse[k] = resource.MakeSecret(v)
		}
		return invokeResponse, nil

	case "mypkg::funcWithDefaultValue",
		"mypkg::funcWithAllOptionalInputs",
		"mypkg::funcWithListParam",
		"mypkg::funcWithDictParam":
		result := mypkg.FuncWithDefaultValueResult{
			R: fmt.Sprintf("%v", args.Args),
		}
		outputs := map[string]interface{}{
			"r": result.R,
		}
		return resource.NewPropertyMapFromMap(outputs), nil

	case "mypkg::getIntegrationRuntimeObjectMetadatum":
		targs := mypkg.GetIntegrationRuntimeObjectMetadatumArgs{}
		for k, v := range args.Args {
			switch k {
			case "factoryName":
				targs.FactoryName = v.StringValue()
			case "integrationRuntimeName":
				targs.IntegrationRuntimeName = v.StringValue()
			case "metadataPath":
				metadataPath := v.StringValue()
				targs.MetadataPath = &metadataPath
			case "resourceGroupName":
				targs.ResourceGroupName = v.StringValue()
			}
		}
		nextLink := "my-next-link"
		result := mypkg.GetIntegrationRuntimeObjectMetadatumResult{
			NextLink: &nextLink,
			Value:    []interface{}{targs},
		}
		outputs := map[string]interface{}{
			"nextLink": result.NextLink,
			"value":    []interface{}{fmt.Sprintf("factoryName=%s", targs.FactoryName)},
		}

		return resource.NewPropertyMapFromMap(outputs), nil
	}

	panic(fmt.Errorf("Unknown token: %s", args.Token))
}

func (mocks) Call(args pulumi.MockInvokeArgs) (resource.PropertyMap, error) {
	panic("Call not supported")
}

func TestListStorageAccountKeysOutput(t *testing.T) {
	pulumiTest(t, func(ctx *pulumi.Context) error {
		output := mypkg.ListStorageAccountKeysOutput(ctx, mypkg.ListStorageAccountKeysOutputArgs{
			AccountName:       pulumi.String("my-account-name"),
			ResourceGroupName: pulumi.String("my-resource-group-name"),
		})

		keys := waitOut(t, output.Keys()).([]mypkg.StorageAccountKeyResponse)

		assert.Equal(t, 1, len(keys))
		assert.Equal(t, "key", keys[0].KeyName)
		assert.Equal(t, "permissions", keys[0].Permissions)
		assert.Equal(t, "accountName=my-account-name, resourceGroupName=my-resource-group-name, expand=",
			keys[0].Value)

		output = mypkg.ListStorageAccountKeysOutput(ctx, mypkg.ListStorageAccountKeysOutputArgs{
			AccountName:       pulumi.String("my-account-name"),
			ResourceGroupName: pulumi.String("my-resource-group-name"),
			Expand:            pulumi.String("my-expand"),
		})

		keys = waitOut(t, output.Keys()).([]mypkg.StorageAccountKeyResponse)

		assert.Equal(t, 1, len(keys))
		assert.Equal(t, "key", keys[0].KeyName)
		assert.Equal(t, "permissions", keys[0].Permissions)
		assert.Equal(t, "accountName=my-account-name, resourceGroupName=my-resource-group-name, expand=my-expand",
			keys[0].Value)

		return nil
	})
}

func TestFuncWithDefaultValueOutput(t *testing.T) {
	pulumiTest(t, func(ctx *pulumi.Context) error {
		output := mypkg.FuncWithDefaultValueOutput(ctx, mypkg.FuncWithDefaultValueOutputArgs{
			A: pulumi.String("my-a"),
		})
		r := waitOut(t, output.R())
		assert.Equal(t, "map[a:{my-a} b:{b-default}]", r)
		return nil
	})
}

func TestFuncWithAllOptionalInputsOutput(t *testing.T) {
	pulumiTest(t, func(ctx *pulumi.Context) error {
		output := mypkg.FuncWithAllOptionalInputsOutput(ctx, mypkg.FuncWithAllOptionalInputsOutputArgs{
			A: pulumi.String("my-a"),
		})
		r := waitOut(t, output.R())
		assert.Equal(t, "map[a:{my-a}]", r)
		return nil
	})
}

func TestFuncWithListParamOutput(t *testing.T) {
	pulumiTest(t, func(ctx *pulumi.Context) error {
		output := mypkg.FuncWithListParamOutput(ctx, mypkg.FuncWithListParamOutputArgs{
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
		output := mypkg.FuncWithDictParamOutput(ctx, mypkg.FuncWithDictParamOutputArgs{
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

func TestGetIntegrationRuntimeObjectMetadatumOutput(t *testing.T) {
	pulumiTest(t, func(ctx *pulumi.Context) error {
		output := mypkg.GetIntegrationRuntimeObjectMetadatumOutput(ctx, mypkg.GetIntegrationRuntimeObjectMetadatumOutputArgs{
			FactoryName:            pulumi.String("my-factory-name"),
			IntegrationRuntimeName: pulumi.String("my-integration-runtime-name"),
			MetadataPath:           pulumi.String("my-metadata-path"),
			ResourceGroupName:      pulumi.String("my-resource-group-name"),
		})
		nextLink := waitOut(t, output.NextLink())
		assert.Equal(t, "my-next-link", *(nextLink.(*string)))

		value := waitOut(t, output.Value())
		assert.Equal(t, []interface{}{"factoryName=my-factory-name"}, value)
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
