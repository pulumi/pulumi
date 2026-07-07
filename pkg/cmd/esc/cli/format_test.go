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

package cli

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFormatAliases(t *testing.T) {
	t.Parallel()

	cases := map[string]renderFormat{
		"json":     {objectValue, encodingJSON},
		"yaml":     {objectValue, encodingYAML},
		"string":   {objectValue, encodingString},
		"detailed": {objectValue, encodingJSONDetailed},
		"dotenv":   {objectProcess, encodingDotenv},
		"shell":    {objectProcess, encodingShell},
	}
	for alias, want := range cases {
		t.Run(alias, func(t *testing.T) {
			t.Parallel()
			got, err := parseFormat(alias)
			require.NoError(t, err)
			assert.Equal(t, want, got)

			// The compositional spelling must resolve identically to its alias.
			spelled := map[string]string{
				"json": "value:json", "yaml": "value:yaml", "string": "value:string",
				"detailed": "value:json-detailed", "dotenv": "process:dotenv", "shell": "process:shell",
			}[alias]
			gotSpelled, err := parseFormat(spelled)
			require.NoError(t, err)
			assert.Equal(t, got, gotSpelled)
		})
	}
}

func TestParseFormatValid(t *testing.T) {
	t.Parallel()

	valid := []string{
		"value:string", "value:json", "value:yaml", "value:json-detailed",
		"process:json", "process:yaml", "process:json-detailed", "process:dotenv", "process:shell",
	}
	for _, s := range valid {
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			_, err := parseFormat(s)
			require.NoError(t, err)
		})
	}
}

func TestParseFormatInvalid(t *testing.T) {
	t.Parallel()

	invalid := []string{
		"value:dotenv",   // dotenv encodes only a flat string map
		"value:shell",    // shell encodes only a flat string map
		"process:string", // process has no bare string encoding
		"bogus",
		"value:bogus",
		"bogus:json",
		"value:",
		":json",
	}
	for _, s := range invalid {
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			_, err := parseFormat(s)
			assert.Error(t, err)
		})
	}
}

func TestValidateFormatPathGuard(t *testing.T) {
	t.Parallel()

	path := resource.PropertyPath{"foo"}

	_, err := validateFormat("value:json", path)
	require.NoError(t, err)

	for _, s := range []string{"process:json", "process:dotenv", "process:json-detailed", "shell"} {
		_, err := validateFormat(s, path)
		assert.Error(t, err, s)
	}

	_, err = validateFormat("process:json", nil)
	require.NoError(t, err)
}
