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
	"output-funcs-tfbridge20/mypkg"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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
		return resource.NewPropertyMapFromMap(outputs), nil
	}

	if args.Token == "mypkg::getAmiIds" {
		// NOTE: only subset of possible fields are tested here in the smoke-test.

		targs := mypkg.GetAmiIdsArgs{}
		for k, v := range args.Args {
			switch k {
			case "owners":
				x := v.ArrayValue()
				for _, owner := range x {
					targs.Owners = append(targs.Owners, owner.StringValue())
				}
			case "nameRegex":
				x := v.StringValue()
				targs.NameRegex = &x
			case "sortAscending":
				x := v.BoolValue()
				targs.SortAscending = &x
			case "filters":
				filters := v.ArrayValue()
				for _, filter := range filters {
					propMap := filter.ObjectValue()
					name := propMap["name"].StringValue()
					values := propMap["values"].ArrayValue()
					var theValues []string
					for _, v := range values {
						theValues = append(theValues, v.StringValue())
					}
					targs.Filters = append(targs.Filters, mypkg.GetAmiIdsFilter{
						Name:   name,
						Values: theValues,
					})
				}
			}
		}

		var filterStrings []string
		for _, f := range targs.Filters {
			fs := fmt.Sprintf("name=%s values=[%s]", f.Name, strings.Join(f.Values, ", "))
			filterStrings = append(filterStrings, fs)
		}

		var id string = fmt.Sprintf("my-id [owners: %s] [filters: %s]",
			strings.Join(targs.Owners, ", "),
			strings.Join(filterStrings, ", "))

		result := mypkg.GetAmiIdsResult{
			Id:            id,
			NameRegex:     targs.NameRegex,
			SortAscending: targs.SortAscending,
		}

		outputs := map[string]interface{}{
			"id":            result.Id,
			"nameRegex":     result.NameRegex,
			"sortAscending": result.SortAscending,
		}

		return resource.NewPropertyMapFromMap(outputs), nil

	}

	panic(fmt.Errorf("Unknown token: %s", args.Token))
}

func (mocks) MethodCall(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
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

func TestGetAmiIdsWorks(t *testing.T) {
	makeFilter := func(n int) mypkg.GetAmiIdsFilterInput {
		return &mypkg.GetAmiIdsFilterArgs{
			Name: pulumi.Sprintf("filter-%d-name", n),
			Values: pulumi.StringArray{
				pulumi.Sprintf("value-%d-1", n),
				pulumi.Sprintf("value-%d-2", n),
			},
		}
	}

	pulumiTest(t, func(ctx *pulumi.Context) error {
		output := mypkg.GetAmiIdsOutput(ctx, mypkg.GetAmiIdsOutputArgs{
			NameRegex:     pulumi.String("[a-z]").ToStringPtrOutput(),
			SortAscending: pulumi.Bool(true).ToBoolPtrOutput(),
			Owners: pulumi.StringArray{
				pulumi.String("owner-1"),
				pulumi.String("owner-2"),
			}.ToStringArrayOutput(),
			Filters: mypkg.GetAmiIdsFilterArray{
				makeFilter(1),
				makeFilter(2),
			}.ToGetAmiIdsFilterArrayOutput(),
		})

		result := waitOut(t, output).(mypkg.GetAmiIdsResult)

		assert.Equal(t, *result.NameRegex, "[a-z]")

		expectId := strings.Join([]string{
			"my-id ",
			"[owners: owner-1, owner-2] ",
			"[filters: ",
			"name=filter-1-name values=[value-1-1, value-1-2], ",
			"name=filter-2-name values=[value-2-1, value-2-2]",
			"]",
		}, "")

		assert.Equal(t, result.Id, expectId)

		return nil
	})
}

func pulumiTest(t *testing.T, testBody func(ctx *pulumi.Context) error) {
	err := pulumi.RunErr(testBody, pulumi.WithMocks("project", "stack", mocks(0)))
	require.NoError(t, err)
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
