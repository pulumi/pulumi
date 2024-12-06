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

package stack

import (
	"bytes"
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
)

func TestShowStackName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		full     bool
		desc     string
		expected string
	}{
		{true, "full name", "text-corp/proj1/dev"},
		{false, "just stack name", "dev"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			args := stackArgs{showStackName: true, fullyQualifyStackNames: tt.full}
			var output bytes.Buffer
			s := backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						StringV: "text-corp/proj1/dev",
						NameV:   tokens.MustParseStackName("dev"),
					}
				},
			}

			err := runStack(context.Background(), &s, &output, args)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected+"\n", output.String())
		})
	}
}

func TestStringifyOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give any
		want string
	}{
		{"int", 42, "42"},
		{"string", "ABC", "ABC"},
		{
			desc: "array",
			give: []string{"hello", "goodbye"},
			want: `["hello","goodbye"]`,
		},
		{
			desc: "object",
			give: map[string]any{
				"foo": 42,
				"bar": map[string]any{
					"baz": true,
				},
			},
			want: `{"bar":{"baz":true},"foo":42}`,
		},
		{
			desc: "special characters",
			give: "pass&word",
			want: "pass&word",
		},
		{
			// https://github.com/pulumi/pulumi/issues/10561
			desc: "html/string",
			give: "<html>",
			want: "<html>",
		},
		{
			// https://github.com/pulumi/pulumi/issues/10561
			desc: "html/list",
			give: []string{"<html>"},
			want: `["<html>"]`,
		},
		{
			// https://github.com/pulumi/pulumi/issues/10561
			desc: "html/object",
			give: map[string]any{
				"foo": "<html>",
			},
			want: `{"foo":"<html>"}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			got := stringifyOutput(tt.give)
			assert.Equal(t, tt.want, got)
		})
	}
}

// mockBackendInstance sets the backend instance for the test and cleans it up after.
func mockBackendInstance(t *testing.T, b backend.Backend) {
	t.Cleanup(func() {
		cmdBackend.BackendInstance = nil
	})
	cmdBackend.BackendInstance = b
}
