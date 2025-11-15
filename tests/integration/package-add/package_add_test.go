// Copyright 2024, Pulumi Corporation.
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

package ints

import (
	"os"
	"path/filepath"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/require"
)

// Test `package add` with a schema that has a namespace set.
func TestPackageAddNamespace(t *testing.T) {
	t.Parallel()

	type testCase struct {
		runtime         string
		expectedMessage string
		filepath        string
	}

	testCases := []testCase{
		{
			runtime:         "dotnet",
			expectedMessage: "You can then use the SDK in your .NET code with:\n\n  using MyNamespace.Mypkg;",
			filepath:        filepath.Join("sdks", "my-namespace-mypkg", "MyNamespace.Mypkg.csproj"),
		},
		{
			runtime:         "nodejs",
			expectedMessage: "You can import the SDK in your TypeScript code with:\n\n  import * as mypkg from \"@my-namespace/mypkg\"", //nolint:lll
			filepath:        filepath.Join("sdks", "my-namespace-mypkg", "index.ts"),
		},
		{
			runtime:         "python",
			expectedMessage: "You can import the SDK in your Python code with:\n\n  import my_namespace_mypkg as mypkg",
			filepath:        filepath.Join("sdks", "my-namespace-mypkg", "my_namespace_mypkg"),
		},
		{
			runtime:         "go",
			expectedMessage: "You can import the SDK in your Go code with:\n\n  import (\n    \"github.com/my-namespace/pulumi-mypkg/sdk/go/mypkg\"\n  )", //nolint:lll
			filepath:        filepath.Join("sdks", "my-namespace-mypkg", "go.mod"),
		},
		{
			runtime:         "java",
			expectedMessage: "To use this SDK in your Java project, complete the following steps",
			filepath:        filepath.Join("sdks", "my-namespace-mypkg", "src", "main", "java", "com", "mynamespace", "mypkg", "Provider.java"), //nolint:lll
		},
	}

	for _, tc := range testCases {
		t.Run(tc.runtime, func(t *testing.T) {
			t.Parallel()
			e := ptesting.NewEnvironment(t)
			defer e.DeleteIfNotFailed()
			e.ImportDirectory(filepath.Join("namespace"))
			e.CWD = filepath.Join(e.RootPath, tc.runtime)

			stdout, _ := e.RunCommand("pulumi", "package", "add", "../provider/schema.json")

			require.Contains(t, stdout, tc.expectedMessage)
			_, err := os.Stat(filepath.Join(e.CWD, tc.filepath))
			require.NoError(t, err)
		})
	}
}
