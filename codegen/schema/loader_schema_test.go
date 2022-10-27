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
