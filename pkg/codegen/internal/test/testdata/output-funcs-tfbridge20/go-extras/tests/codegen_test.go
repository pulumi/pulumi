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

	"output-funcs-tfbridge20/mypkg"
)

type mocks int

// Create the mock.
func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	panic("NewResource not supported")
}

func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	if args.Token == "mypkg::listStorageAccountKeys" {

		targs := mypkg.ListStorageAccountKeysArgs{}
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
		return resource.NewPropertyMapFromMap(outputs), nil
	}

	panic(fmt.Errorf("Unknown token: %s", args.Token))
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
