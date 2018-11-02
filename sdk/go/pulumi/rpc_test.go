// Copyright 2016-2018, Pulumi Corporation.
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

package pulumi

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/go/pulumi/asset"
)

// TestMarshalRoundtrip ensures that marshaling a complex structure to and from its on-the-wire gRPC format succeeds.
func TestMarshalRoundtrip(t *testing.T) {
	// Create interesting inputs.
	out, resolve, _ := NewOutput(nil)
	resolve("outputty", true)
	input := map[string]interface{}{
		"s":            "a string",
		"a":            true,
		"b":            42,
		"cStringAsset": asset.NewStringAsset("put a lime in the coconut"),
		"cFileAsset":   asset.NewFileAsset("foo.txt"),
		"cRemoteAsset": asset.NewRemoteAsset("https://pulumi.com/fake/asset.txt"),
		"dAssetArchive": asset.NewAssetArchive(map[string]interface{}{
			"subAsset":   asset.NewFileAsset("bar.txt"),
			"subArchive": asset.NewFileArchive("bar.zip"),
		}),
		"dFileArchive":   asset.NewFileArchive("foo.zip"),
		"dRemoteArchive": asset.NewRemoteArchive("https://pulumi.com/fake/archive.zip"),
		"e":              out,
		"fArray":         []interface{}{0, 1.3, "x", false},
		"fMap": map[string]interface{}{
			"x": "y",
			"y": 999.9,
			"z": false,
		},
	}

	// Marshal those inputs.
	m, deps, err := marshalInputs(input)
	if !assert.Nil(t, err) {
		assert.Equal(t, 0, len(deps))

		// Now just unmarshal and ensure the resulting map matches.
		res, err := unmarshalOutputs(m)
		if !assert.Nil(t, err) {
			if !assert.NotNil(t, res) {
				assert.Equal(t, "a string", res["s"])
				assert.Equal(t, true, res["a"])
				assert.Equal(t, 42, res["b"])
				assert.Equal(t, "put a lime in the coconut", res["cStringAsset"].(asset.Asset).Text())
				assert.Equal(t, "foo.txt", res["cFileAsset"].(asset.Asset).Path())
				assert.Equal(t, "https://pulumi.com/fake/asset.txt", res["cRemoteAsset"].(asset.Asset).URI())
				ar := res["dAssetArchive"].(asset.Archive).Assets()
				assert.Equal(t, 2, len(ar))
				assert.Equal(t, "bar.txt", ar["subAsset"].(asset.Asset).Path())
				assert.Equal(t, "bar.zip", ar["subrchive"].(asset.Archive).Path())
				assert.Equal(t, "foo.zip", res["dFileArchive"].(asset.Archive).Path())
				assert.Equal(t, "https://pulumi.com/fake/archive.zip", res["dRemoteArchive"].(asset.Archive).URI())
				assert.Equal(t, "outputty", res["e"])
				aa := res["fArray"].([]interface{})
				assert.Equal(t, 4, len(aa))
				assert.Equal(t, 0, aa[0])
				assert.Equal(t, 1.3, aa[1])
				assert.Equal(t, "x", aa[2])
				assert.Equal(t, false, aa[3])
				am := res["fMap"].(map[string]interface{})
				assert.Equal(t, 3, len(am))
				assert.Equal(t, "y", am["x"])
				assert.Equal(t, 999.9, am["y"])
				assert.Equal(t, false, am["z"])
			}
		}
	}
}
