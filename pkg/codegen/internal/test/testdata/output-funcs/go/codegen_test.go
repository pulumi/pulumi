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

func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	if args.Token == "azure-native:codegentest:listStorageAccountKeys" {

		targs := ListStorageAccountKeysArgs{}
		for k, v := range args.Args {
			switch k {
			case "accountName":
				targs.AccountName = v.V.(string)
			case "expand":
				expand := v.V.(string)
				targs.Expand = &expand
			case "resourceGroupName":
				targs.ResourceGroupName = v.V.(string)
			}
		}

		var expand string
		if targs.Expand != nil {
			expand = *targs.Expand
		}

		inputs := []StorageAccountKeyResponse{
			{
				KeyName:     "key",
				Permissions: "permissions",
				Value: fmt.Sprintf("accountName=%v, resourceGroupName=%v, expand=%v",
					targs.AccountName,
					targs.ResourceGroupName,
					expand),
			},
		}
		result := ListStorageAccountKeysResult{
			Keys: inputs,
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

	if args.Token == "azure-native:codegentest:getIntegrationRuntimeObjectMetadatum" {
		targs := GetIntegrationRuntimeObjectMetadatumArgs{}
		for k, v := range args.Args {
			switch k {
			case "factoryName":
				targs.FactoryName = v.V.(string)
			case "integrationRuntimeName":
				targs.IntegrationRuntimeName = v.V.(string)
			case "metadataPath":
				metadataPath := v.V.(string)
				targs.MetadataPath = &metadataPath
			case "resourceGroupName":
				targs.ResourceGroupName = v.V.(string)
			}
		}
		nextLink := "my-next-link"
		result := GetIntegrationRuntimeObjectMetadatumResult{
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

func TestListStorageAccountKeysOutput(t *testing.T) {
	pulumiTest(t, func(ctx *pulumi.Context) error {
		output := ListStorageAccountKeysOutput(ctx, ListStorageAccountKeysOutputArgs{
			AccountName:       pulumi.String("my-account-name"),
			ResourceGroupName: pulumi.String("my-resource-group-name"),
		})

		keys := waitOut(t, output.Keys()).([]StorageAccountKeyResponse)

		assert.Equal(t, 1, len(keys))
		assert.Equal(t, "key", keys[0].KeyName)
		assert.Equal(t, "permissions", keys[0].Permissions)
		assert.Equal(t, "accountName=my-account-name, resourceGroupName=my-resource-group-name, expand=",
			keys[0].Value)

		output = ListStorageAccountKeysOutput(ctx, ListStorageAccountKeysOutputArgs{
			AccountName:       pulumi.String("my-account-name"),
			ResourceGroupName: pulumi.String("my-resource-group-name"),
			Expand:            pulumi.String("my-expand"),
		})

		keys = waitOut(t, output.Keys()).([]StorageAccountKeyResponse)

		assert.Equal(t, 1, len(keys))
		assert.Equal(t, "key", keys[0].KeyName)
		assert.Equal(t, "permissions", keys[0].Permissions)
		assert.Equal(t, "accountName=my-account-name, resourceGroupName=my-resource-group-name, expand=my-expand",
			keys[0].Value)

		return nil
	})
}

// TODO[pulumi/pulumi#7811]: it seems that default values are not
// supported by Go codegen yet, hence we do not observe "B" populated
// to default at all here.
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

func TestGetIntegrationRuntimeObjectMetadatumOutput(t *testing.T) {
	pulumiTest(t, func(ctx *pulumi.Context) error {
		output := GetIntegrationRuntimeObjectMetadatumOutput(ctx, GetIntegrationRuntimeObjectMetadatumOutputArgs{
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
