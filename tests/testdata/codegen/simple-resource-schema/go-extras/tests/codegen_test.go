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
	"simple-resource-schema/example"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type mocks int

func pulumiTest(t *testing.T, body func(ctx *pulumi.Context) error) {
	err := pulumi.RunErr(body, pulumi.WithMocks("project", "stack", mocks(0)))
	require.NoError(t, err)
}

func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.ID, args.Inputs, nil
}

func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	panic("Invoke not supported")
}

func (mocks) MethodCall(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	panic("Call not supported")
}

func TestHasDefaultPluginDownloadURL(t *testing.T) {
	pulumiTest(t, func(ctx *pulumi.Context) error {
		r, err := example.NewResource(ctx, "resource", &example.ResourceArgs{})
		require.NoError(t, err)
		assert.Contains(t, fmt.Sprintf("%#v", r), `pluginDownloadURL:"example.com/download"`)
		return nil
	})
}

func TestCanOverrideDefaultPluginDownloadURL(t *testing.T) {
	pulumiTest(t, func(ctx *pulumi.Context) error {
		r, err := example.NewResource(ctx, "resource", &example.ResourceArgs{},
			pulumi.PluginDownloadURL("example.com/other"))
		require.NoError(t, err)
		assert.Contains(t, fmt.Sprintf("%#v", r), `pluginDownloadURL:"example.com/other"`)
		return nil
	})
}
