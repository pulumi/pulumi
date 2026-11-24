// Copyright 2026, Pulumi Corporation.
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

package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// Test that lifting a raw asset or archive into a [property.Value] and extracting it
// again is lossless. Unlike the round-trip tests in property_compatibility_test.go, this
// compares against the raw input, so it catches normalization introduced by the copy on
// construction — such as a nil archive Assets map becoming an empty one.
func TestNewPreservesAssetsAndArchives(t *testing.T) {
	t.Parallel()

	t.Run("asset", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			a := assetGen().Draw(t, "asset")
			assert.Equal(t, a, property.New(a).AsAsset())
		})
	})

	t.Run("archive", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			a := archiveGen(archiveNestingDepth).Draw(t, "archive")
			assert.Equal(t, a, property.New(a).AsArchive())
		})
	})
}
