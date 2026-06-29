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

package neo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/acp"
)

func TestPromptText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		blocks []acp.ContentBlock
		want   string
	}{
		{
			name:   "empty",
			blocks: nil,
			want:   "",
		},
		{
			name:   "text only",
			blocks: []acp.ContentBlock{{Type: "text", Text: "hello"}},
			want:   "hello",
		},
		{
			name: "concatenates text blocks",
			blocks: []acp.ContentBlock{
				{Type: "text", Text: "hello "},
				{Type: "text", Text: "world"},
			},
			want: "hello world",
		},
		{
			name: "resource link renders its uri",
			blocks: []acp.ContentBlock{
				{Type: "resource_link", URI: "file:///repo/main.go", Name: "main.go"},
			},
			want: "@file:///repo/main.go",
		},
		{
			name: "resource link shows label when it differs from uri",
			blocks: []acp.ContentBlock{
				{Type: "resource_link", URI: "file:///repo/main.go", Name: "main.go", Title: "Entry point"},
			},
			want: "@file:///repo/main.go (Entry point)",
		},
		{
			name: "resource link falls back to name when no title",
			blocks: []acp.ContentBlock{
				{Type: "resource_link", URI: "file:///repo/main.go", Name: "main.go"},
			},
			want: "@file:///repo/main.go",
		},
		{
			name: "text interleaved with a resource link",
			blocks: []acp.ContentBlock{
				{Type: "text", Text: "look at "},
				{Type: "resource_link", URI: "file:///repo/main.go", Name: "main.go"},
				{Type: "text", Text: " please"},
			},
			want: "look at @file:///repo/main.go please",
		},
		{
			name: "capability-gated blocks are ignored",
			blocks: []acp.ContentBlock{
				{Type: "text", Text: "keep"},
				{Type: "image"},
				{Type: "audio"},
				{Type: "resource"},
			},
			want: "keep",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, promptText(tt.blocks))
		})
	}
}
