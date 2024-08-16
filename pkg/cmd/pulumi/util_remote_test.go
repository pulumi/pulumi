// Copyright 2016-2022, Pulumi Corporation.
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
	"context"
	"errors"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEnv(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input     string
		name      string
		value     string
		errString string
	}{
		"name val":            {input: "FOO=bar", name: "FOO", value: "bar"},
		"name empty val":      {input: "FOO=", name: "FOO"},
		"name val extra seps": {input: "FOO=bar=baz", name: "FOO", value: "bar=baz"},
		"empty":               {input: "", errString: `expected value of the form "NAME=value": missing "=" in ""`},
		"no sep":              {input: "foo", errString: `expected value of the form "NAME=value": missing "=" in "foo"`},
		"empty name val":      {input: "=", errString: `expected non-empty environment name in "="`},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			name, value, err := parseEnv(tc.input)
			if tc.errString != "" {
				assert.EqualError(t, err, tc.errString)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.name, name)
			assert.Equal(t, tc.value, value)
		})
	}
}

//nolint:paralleltest // uses mock backend
func TestRemoteExclusiveURL(t *testing.T) {
	stackRef := "org/foo/bar"
	mockBackendInstance(t, &backend.MockBackend{
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			if s != stackRef {
				return nil, errors.New("unexpected stack reference")
			}
			return &backend.MockStackReference{
				StringV:             "org/project/name",
				NameV:               tokens.MustParseStackName("name"),
				ProjectV:            "project",
				FullyQualifiedNameV: tokens.QName("org/project/name"),
			}, nil
		},
	})

	type testCase struct {
		name string

		url              string
		gitHubRepository string

		expectUrlError bool
	}

	testCases := []testCase{
		{
			name: "url only",
			url:  "https://example.com/foo/bar.git",
		},
		{
			name:             "github-repository only",
			gitHubRepository: "thwomp/quux",
		},
		{
			name:             "both",
			url:              "https://example.com/foo/bar.git",
			gitHubRepository: "thwomp/quux",
			expectUrlError:   true,
		},
		{
			name:           "neither",
			expectUrlError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res := runDeployment(context.Background(), display.Options{}, apitype.Update, stackRef, tc.url, RemoteArgs{
				remote:           true,
				gitHubRepository: tc.gitHubRepository,
			})

			err := res.Error()
			require.Error(t, err)
			if tc.expectUrlError {
				assert.Contains(t, err.Error(), "one of `url` or `github-repository` must be specified, and not both")
			} else {
				assert.Contains(t, err.Error(), "no cloud backend available")
			}
		})
	}
}
