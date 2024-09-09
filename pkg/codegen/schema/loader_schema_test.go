// Copyright 2022-2024, Pulumi Corporation.
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

package schema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmptySchemaResponse(t *testing.T) {
	t.Parallel()

	assert.True(t, schemaIsEmpty([]byte("{}")))
	assert.True(t, schemaIsEmpty([]byte("{  }")))
	assert.True(t, schemaIsEmpty([]byte("{	}")))           // tab character
	assert.True(t, schemaIsEmpty([]byte("{   	 	 				}"))) // mixed tabs and spaces]

	assert.True(t, schemaIsEmpty([]byte(" {} \n")))
	assert.True(t, schemaIsEmpty([]byte(" \n{  }	\n")))
	assert.True(t, schemaIsEmpty([]byte("{\n	}\n")))
	assert.True(t, schemaIsEmpty([]byte("\n		 {   	 	 				}  	\n\n")))

	assert.False(t, schemaIsEmpty([]byte(`{"key": "value"}`)))
}

func BenchmarkSchemaEmptyCheck(b *testing.B) {
	schemaPath, err := filepath.Abs("../testing/test/testdata/azure-native.json")
	assert.NoError(b, err)
	largeSchema, err := os.ReadFile(schemaPath)
	if err != nil {
		b.Fatalf("failed to read schema file, ensure that you have run "+
			"`make get_schemas` to create schema file %q", schemaPath)
	}

	b.Run("large-schema-empty-check-time", func(b *testing.B) {
		empty := schemaIsEmpty(largeSchema)
		assert.False(b, empty)
	})
}
