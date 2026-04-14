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

package otelreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestEncodeDecodePropertyValue(t *testing.T) {
	t.Parallel()

	s, err := structpb.NewStruct(map[string]any{
		"name": "my-bucket",
		"tags": map[string]any{"env": "prod"},
	})
	require.NoError(t, err)

	encoded, err := EncodePropertyValue(s)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	decoded, err := DecodePropertyValue(encoded)
	require.NoError(t, err)

	assert.Equal(t, "my-bucket", decoded.Fields["name"].GetStringValue())
	tags := decoded.Fields["tags"].GetStructValue()
	require.NotNil(t, tags)
	assert.Equal(t, "prod", tags.Fields["env"].GetStringValue())
}

func TestDecodePropertyValueRejectsGarbage(t *testing.T) {
	t.Parallel()

	_, err := DecodePropertyValue([]byte("not a property value"))
	assert.Error(t, err)
}

func TestDecodePropertyValueRejectsWrongMagic(t *testing.T) {
	t.Parallel()

	s, err := structpb.NewStruct(map[string]any{"key": "value"})
	require.NoError(t, err)

	encoded, err := EncodePropertyValue(s)
	require.NoError(t, err)

	// Corrupt the magic (bytes 2-9 are the fixed64 after the tag byte)
	encoded[2] = 0xFF

	_, err = DecodePropertyValue(encoded)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "magic mismatch")
}
