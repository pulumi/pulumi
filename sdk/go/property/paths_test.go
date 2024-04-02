// Copyright 2016-2024, Pulumi Corporation.
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

package property_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path     property.Path
		repr     string
		altPaths []string
	}{
		{
			path: property.NewPath().Field("root"),
			repr: "root",
		},
		{
			path:     property.NewPath().Field("root").Field("nested"),
			repr:     "root.nested",
			altPaths: []string{`root["nested"]`},
		},
		{
			path:     property.NewPath().Field("root").Field("double").Field("nest"),
			repr:     "root.double.nest",
			altPaths: []string{`root["double"].nest`, `root["double"]["nest"]`},
		},
		{
			path: property.NewPath().Field("root").Field("array").Index(0),
			repr: "root.array[0]",
		},
		{
			path: property.NewPath().Field("root").Field("array").Index(100),
			repr: "root.array[100]",
		},
		{
			path: property.NewPath().Field("root").Field("array").Index(0).Field("nested"),
			repr: `root.array[0].nested`,
		},
		{
			path: property.NewPath().Field("root").Field("array").Index(0).Index(1).Field("nested"),
			repr: `root.array[0][1].nested`,
		},
		{
			path: property.NewPath().Field("root").Field("nested").Field("array").
				Index(0).Field("double").Index(1),
			repr: `root.nested.array[0].double[1]`,
		},
		{
			path: property.NewPath().Field("root").Field(`key with "escaped" quotes`),
			repr: `root["key with \"escaped\" quotes"]`,
		},
		{
			path: property.NewPath().Field("root").Field("key with a ."),
			repr: `root["key with a ."]`,
		},
		{
			path: property.NewPath().Field(`root key with "escaped" quotes`).Field("nested"),
			repr: `["root key with \"escaped\" quotes"].nested`,
		},
		{
			path: property.NewPath().Field("root key with a .").Index(100),
			repr: `["root key with a ."][100]`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.path.String(), tt.repr)

			actual, err := property.ParsePath(tt.repr)
			if assert.NoError(t, err) {
				assert.Equal(t, actual, tt.path)
			}

			for _, alt := range tt.altPaths {
				a, err := property.ParsePath(alt)
				if assert.NoError(t, err) {
					assert.Equal(t, a, tt.path)
				}

			}
		})
	}
}
