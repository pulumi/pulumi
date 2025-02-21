// Copyright 2025, Pulumi Corporation.
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

package convert

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

// Tests that a plugin mapper wrapped with a caching layer only attempts to install a plugin once.
//
// Regression test for https://github.com/pulumi/pulumi/issues/13105
func TestCachingPluginMapper_OnlyInstallsOnce(t *testing.T) {
	t.Parallel()

	// Arrange.
	ws := &testWorkspace{}

	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		t.Fatal("should not be called")
		return nil, nil
	}

	installCalled := 0
	installPlugin := func(pluginName string) *semver.Version {
		assert.Equal(t, "gcp", pluginName)
		installCalled++

		return nil
	}

	baseMapper, err := NewBasePluginMapper(
		ws,
		"key", /*conversionKey*/
		providerFactory,
		installPlugin,
		nil, /*mappings*/
	)
	mapper := NewCachingMapper(baseMapper)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	// Act.
	//
	// After the first time, we should have attempted an installation.
	data, err := mapper.GetMapping(context.Background(), "gcp", nil /*hint*/)

	// Assert.
	assert.NoError(t, err)
	assert.Equal(t, []byte{}, data)
	assert.Equal(t, 1, installCalled, "install should be called the first time when a caching mapper is used")

	// Act.
	//
	// After the second time, we should still have only attempted an installation once.
	data, err = mapper.GetMapping(context.Background(), "gcp", nil /*hint*/)

	// Assert.
	assert.NoError(t, err)
	assert.Equal(t, []byte{}, data)
	assert.Equal(t, 1, installCalled, "install should only be called once when a caching mapper is used")
}
