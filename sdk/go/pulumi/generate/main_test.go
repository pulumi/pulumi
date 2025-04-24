// Copyright 2023-2024, Pulumi Corporation.
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

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplateFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give string
		want string
	}{
		{"foo-bar.go.template", "foo/bar.go"},
		{"bar.go.template", "bar.go"},
		{"fizz-buz-bar.go.template", "fizz/buz/bar.go"},
		{"foo-bar.tmpl.template", "foo/bar.tmpl"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.give, func(t *testing.T) {
			t.Parallel()

			got := templateFilePath(tt.give)
			assert.Equal(t, tt.want, got)
		})
	}
}
