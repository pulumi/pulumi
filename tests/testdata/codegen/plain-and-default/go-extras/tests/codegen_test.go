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
	"context"
	"fmt"
	"plain-and-default/foo"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

type mocks int

// Create the mock.
func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name, args.Inputs, nil
}

func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	panic("methods not supported")
}

func (mocks) Invoke(args pulumi.MockInvokeArgs) (resource.PropertyMap, error) {
	panic("functions not supported")
}

func TestDefaults(t *testing.T) {
	pulumiTest(t, "explicit false", func(ctx *pulumi.Context) error {
		output, err := foo.NewModuleResource(ctx, "test", &foo.ModuleResourceArgs{
			OptionalBool: pulumi.Bool(false),
		})
		assert.NoError(t, err)
		assert.Equalf(t, *waitOut(t, output.OptionalBool).(*bool), false,
			"Value has been set to false, make sure it doesn't change.")
		return nil
	})

	pulumiTest(t, "explicit true", func(ctx *pulumi.Context) error {
		output, err := foo.NewModuleResource(ctx, "test", &foo.ModuleResourceArgs{
			OptionalBool: pulumi.Bool(true),
		})
		assert.NoError(t, err)
		assert.Equalf(t, *waitOut(t, output.OptionalBool).(*bool), true,
			"Value has been set to true, make sure it doesn't change.")
		return nil
	})

	pulumiTest(t, "default value", func(ctx *pulumi.Context) error {
		output, err := foo.NewModuleResource(ctx, "test", &foo.ModuleResourceArgs{})
		assert.NoError(t, err)
		assert.Equalf(t, *waitOut(t, output.OptionalBool).(*bool), true,
			"Default value is true, and the value has not been specified")
		return nil
	})
}

func pulumiTest(t *testing.T, name string, testBody func(*pulumi.Context) error) {
	t.Run(name, func(t *testing.T) {
		err := pulumi.RunErr(testBody, pulumi.WithMocks("project", "stack", mocks(0)))
		assert.NoError(t, err)
	})
}

func waitOut(t *testing.T, output pulumi.Output) interface{} {
	result, err := waitOutput(output, 1*time.Second)
	if !assert.NoError(t, err, "output not received") {
		return nil
	}
	return result
}

func waitOutput(output pulumi.Output, timeout time.Duration) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ch := make(chan interface{})
	output.ApplyT(func(v interface{}) interface{} {
		ch <- v
		return v
	})

	select {
	case v := <-ch:
		return v, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("timed out waiting for pulumi.Output after %v: %w", timeout, ctx.Err())
	}
}
