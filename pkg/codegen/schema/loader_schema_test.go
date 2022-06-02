package schema

import (
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
