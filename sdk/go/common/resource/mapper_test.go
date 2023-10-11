// Copyright 2016-2022, Pulumi Corporation.
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

package resource_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"
)

type complexBag struct {
	asset   resource.Asset
	archive resource.Archive

	optionalAsset   *resource.Asset
	optionalArchive *resource.Archive
}

func TestAssetsAndArchives(t *testing.T) {
	t.Parallel()

	newAsset := func(s string) *resource.Asset {
		a, err := resource.NewTextAsset(s)
		require.NoError(t, err, "creating asset %s", s)
		return a
	}
	newArchive := func(m map[string]interface{}) *resource.Archive {
		a, err := resource.NewAssetArchive(m)
		require.NoError(t, err, "creating asset %#v", m)
		return a
	}

	bigArchive := func() *resource.Archive {
		return newArchive(map[string]interface{}{
			"asset1": newAsset("asset1"),
			"archive1": newArchive(map[string]interface{}{
				"asset2": newAsset("asset2"),
				"asset3": newAsset("asset3"),
			}),
		})
	}
	tree := map[string]interface{}{
		"asset":           newAsset("simple asset"),
		"optionalAsset":   newAsset("simple optional asset"),
		"archive":         bigArchive(),
		"optionalArchive": bigArchive(),
	}

	bag := complexBag{}
	md := mapper.New(nil)

	t.Run("asset", func(t *testing.T) { //nolint:parallelTest
		err := md.DecodeValue(tree, reflect.TypeOf(complexBag{}), "asset", &bag.asset, false)
		assert.NoError(t, err)
	})
	t.Run("optionalAsset", func(t *testing.T) { //nolint:parallelTest
		err := md.DecodeValue(tree, reflect.TypeOf(complexBag{}), "optionalAsset", &bag.optionalAsset, false)
		assert.NoError(t, err)
	})
	t.Run("archive", func(t *testing.T) { //nolint:parallelTest
		err := md.DecodeValue(tree, reflect.TypeOf(complexBag{}), "archive", &bag.archive, false)
		assert.NoError(t, err)
	})
	t.Run("optionalArchive", func(t *testing.T) { //nolint:parallelTest
		err := md.DecodeValue(tree, reflect.TypeOf(complexBag{}), "optionalArchive", &bag.optionalArchive, false)
		assert.NoError(t, err)
	})
}
