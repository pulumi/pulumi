// Copyright 2020-2024, Pulumi Corporation.
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

package goversion

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_checkMinimumGoVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		goVersionOutput string
		err             string
	}{
		{
			name:            "ExactVersion",
			goVersionOutput: "go version go1.14.0 darwin/amd64",
		},
		{
			name:            "NewerVersion",
			goVersionOutput: "go version go1.15.1 darwin/amd64",
		},
		{
			name:            "BetaVersion",
			goVersionOutput: "go version go1.18beta2 darwin/amd64",
		},
		{
			name:            "OlderGoVersion",
			goVersionOutput: "go version go1.13.8 linux/amd64",
			err:             "go version must be 1.14.0 or higher (1.13.8 detected)",
		},
		{
			name:            "MalformedVersion",
			goVersionOutput: "go version xyz",
			err:             "parsing go version: Malformed version: xyz",
		},
		{
			name:            "GarbageVersionOutput",
			goVersionOutput: "gobble gobble",
			err:             "unexpected format for go version output: \"gobble gobble\"",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := checkMinimumGoVersion(tt.goVersionOutput)
			if tt.err != "" {
				assert.EqualError(t, err, tt.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
