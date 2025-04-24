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
	"sync"
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

// TestCachingPluginMapper_ConcurrentAccess tests that the caching mapper correctly
// handles concurrent access from multiple goroutines.
func TestCachingPluginMapper_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	// Arrange.
	ws := &testWorkspace{}

	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		t.Fatal("should not be called")
		return nil, nil
	}

	installPlugin := func(pluginName string) *semver.Version {
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
	// Run multiple goroutines concurrently that all try to get mappings
	// for different providers to maximize map writes
	const numGoroutines = 100

	// Used to wait for all goroutines to complete
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Create multiple goroutines that will all try to access the map at the same time
	for i := 0; i < numGoroutines; i++ {
		// Use two different provider names to enforce multiple map writes
		provider := "gcp"
		if i%2 == 0 {
			provider = "aws"
		}

		go func(p string) {
			defer wg.Done()

			// Get the mapping - this will cause concurrent map writes without proper locking
			_, err := mapper.GetMapping(context.Background(), p, nil /*hint*/)
			assert.NoError(t, err)
		}(provider)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Assert.
	// This test will fail with "concurrent map writes" without proper locking
	// If it reaches here, the test passes
}
