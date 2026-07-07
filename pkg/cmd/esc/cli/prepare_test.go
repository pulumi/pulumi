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
	"testing/fstest"

	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func projectionTestEnvironment() *esc.Environment {
	return &esc.Environment{
		Properties: map[string]esc.Value{
			"environmentVariables": esc.NewValue(map[string]esc.Value{
				"FOO":    esc.NewValue("bar"),
				"SECRET": esc.NewSecret("shh"),
			}),
			"files": esc.NewValue(map[string]esc.Value{
				"FILE":        esc.NewValue("contents"),
				"SECRET_FILE": esc.NewSecret("secret contents"),
			}),
		},
	}
}

func TestProjectEnvironmentStructure(t *testing.T) {
	t.Parallel()

	fs := testFS{MapFS: fstest.MapFS{}}
	proj, err := projectEnvironment(projectionTestEnvironment(), &PrepareOptions{fs: fs})
	require.NoError(t, err)

	// Environment variables come first, sorted by name; then files, sorted by name.
	assert.Equal(t, []ProjectedVariable{
		{Name: "FOO", Value: "bar", Secret: false},
		{Name: "SECRET", Value: "shh", Secret: true},
	}, proj.Variables)
	assert.Equal(t, []ProjectedFile{
		{Name: "FILE", Path: "temp/esc-temp-0", Secret: false},
		{Name: "SECRET_FILE", Path: "temp/esc-temp-1", Secret: true},
	}, proj.Files)

	// env run's redactor depends on this order: secret env-var values, then secret file contents.
	assert.Equal(t, []string{"shh", "secret contents"}, proj.Secrets)
	assert.Equal(t, []string{"temp/esc-temp-0", "temp/esc-temp-1"}, proj.Paths)
}

func TestProjectEnvironmentPretendSkipsMaterialization(t *testing.T) {
	t.Parallel()

	fs := testFS{MapFS: fstest.MapFS{}}
	proj, err := projectEnvironment(projectionTestEnvironment(), &PrepareOptions{Pretend: true, fs: fs})
	require.NoError(t, err)

	assert.Equal(t, []ProjectedFile{
		{Name: "FILE", Path: "[unknown]", Secret: false},
		{Name: "SECRET_FILE", Path: "[unknown]", Secret: true},
	}, proj.Files)
	assert.Empty(t, proj.Paths)
	assert.Empty(t, fs.MapFS)
}

func TestPrepareEnvironmentQuoteRedact(t *testing.T) {
	t.Parallel()

	fs := testFS{MapFS: fstest.MapFS{}}
	files, environ, secrets, err := PrepareEnvironment(
		projectionTestEnvironment(),
		&PrepareOptions{Quote: true, Redact: true, fs: fs},
	)
	require.NoError(t, err)

	// Secret env-var values redact to [secret]; file paths are never redacted; everything is quoted.
	assert.Equal(t, []string{
		`FOO="bar"`,
		`SECRET="[secret]"`,
		`FILE="temp/esc-temp-0"`,
		`SECRET_FILE="temp/esc-temp-1"`,
	}, environ)
	assert.Equal(t, []string{"shh", "secret contents"}, secrets)
	assert.Equal(t, []string{"temp/esc-temp-0", "temp/esc-temp-1"}, files)
}

func TestPrepareEnvironmentUnquotedUnredacted(t *testing.T) {
	t.Parallel()

	fs := testFS{MapFS: fstest.MapFS{}}
	_, environ, _, err := PrepareEnvironment(projectionTestEnvironment(), &PrepareOptions{fs: fs})
	require.NoError(t, err)

	// This is the shape env run feeds to the child process: raw, unquoted, unredacted.
	assert.Equal(t, []string{
		"FOO=bar",
		"SECRET=shh",
		"FILE=temp/esc-temp-0",
		"SECRET_FILE=temp/esc-temp-1",
	}, environ)
}
