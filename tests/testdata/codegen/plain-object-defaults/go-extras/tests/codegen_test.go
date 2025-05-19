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
	"plain-object-defaults/example"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type mocks int

// We assert that default values were passed to our constuctor
func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	checkFloat64 := func(v resource.PropertyValue, k string, expected float64) {
		m := v.ObjectValue()
		if m[resource.PropertyKey(k)].NumberValue() != expected {
			panic(fmt.Sprintf("Expected %s to have value %.2f", k, expected))
		}
	}
	for k, v := range args.Inputs {
		switch k {
		case "kubeClientSettings":
			checkFloat64(v, "burst", 42)
		case "backupKubeClientSettings":
			checkFloat64(v, "qps", 7)
		}
	}
	return args.Name, args.Inputs.Copy(), nil
}

func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	panic("Call not supported")
}

func (mocks) Invoke(args pulumi.MockInvokeArgs) (resource.PropertyMap, error) {
	panic("Invoke not supported")
}

func TestObjectDefaults(t *testing.T) {
	path := "thePath"
	defaultDriver := "secret"
	kcs := example.HelmReleaseSettings{
		PluginsPath: &path,
		RequiredArg: "This is required",
	}
	withDefaults := kcs.Defaults()
	assert.Equal(t, kcs.RequiredArg, withDefaults.RequiredArg)
	assert.Equal(t, kcs.PluginsPath, withDefaults.PluginsPath)
	assert.Nil(t, kcs.Driver)
	assert.Equal(t, withDefaults.Driver, &defaultDriver)
}

func TestTransitiveObjectDefaults(t *testing.T) {
	layered := example.LayeredType{
		Other: example.HelmReleaseSettings{},
	}
	withDefaults := layered.Defaults()
	assert.Equal(t, "secret", *withDefaults.Other.Driver)
}

// We already have that defaults for resources. We test that they translate through objects.
func TestDefaultResource(t *testing.T) {
	t.Setenv("PULUMI_K8S_CLIENT_BURST", "42")
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := example.NewFoo(ctx, "foo", &example.FooArgs{
			KubeClientSettings:       example.KubeClientSettingsPtr(&example.KubeClientSettingsArgs{}),
			BackupKubeClientSettings: &example.KubeClientSettingsArgs{Qps: pulumi.Float64(7)},
		})
		assert.NoError(t, err)
		return nil
	}, pulumi.WithMocks("example", "stack", mocks(0)))
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
