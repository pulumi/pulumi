// Copyright 2016-2025, Pulumi Corporation.
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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddStringToChunks(t *testing.T) {
	t.Run("Empty Chunks", func(t *testing.T) {
		t.Parallel()
		var chunks [][]string
		addStringToChunks(&chunks, "foo", 10)
		assert.Equal(t, 1, len(chunks))
		assert.Equal(t, []string{"foo"}, chunks[0])
	})

	t.Run("Add To Existing Chunk", func(t *testing.T) {
		t.Parallel()
		chunks := [][]string{{"foo"}}
		// "foo" is 3 bytes, "bar" is 3 bytes, maxChunkSize is 10
		addStringToChunks(&chunks, "bar", 10)
		assert.Equal(t, 1, len(chunks))
		assert.Equal(t, []string{"foo", "bar"}, chunks[0])
	})

	t.Run("New Chunk When Too Large", func(t *testing.T) {
		t.Parallel()
		chunks := [][]string{{"12345"}}
		// "12345" is 5 bytes, "67890" is 6 bytes, maxChunkSize is 10
		addStringToChunks(&chunks, "678901", 10)
		assert.Equal(t, 2, len(chunks))
		assert.Equal(t, []string{"12345"}, chunks[0])
		assert.Equal(t, []string{"678901"}, chunks[1])
	})

	t.Run("New Chunk When Full", func(t *testing.T) {
		t.Parallel()
		chunks := [][]string{{"12345"}}
		// "12345" is 5 bytes, "67890" is 5 bytes, maxChunkSize is 5
		addStringToChunks(&chunks, "67890", 5)
		assert.Equal(t, 2, len(chunks))
		assert.Equal(t, []string{"12345"}, chunks[0])
		assert.Equal(t, []string{"67890"}, chunks[1])
	})

	t.Run("Exact Fit", func(t *testing.T) {
		t.Parallel()
		chunks := [][]string{{"abc"}}
		// "abc" is 3 bytes, "de" is 2 bytes, maxChunkSize is 5
		addStringToChunks(&chunks, "de", 5)
		assert.Equal(t, 1, len(chunks))
		assert.Equal(t, []string{"abc", "de"}, chunks[0])
	})

	t.Run("Multiple Adds", func(t *testing.T) {
		t.Parallel()
		var chunks [][]string
		addStringToChunks(&chunks, "a", 2)
		addStringToChunks(&chunks, "b", 2)
		addStringToChunks(&chunks, "c", 2)
		// "a" and "b" fit in first chunk (1+1=2), "c" starts new chunk
		assert.Equal(t, 2, len(chunks))
		assert.Equal(t, []string{"a", "b"}, chunks[0])
		assert.Equal(t, []string{"c"}, chunks[1])
	})
}
