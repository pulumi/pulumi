// Copyright 2025, Pulumi Corporation.
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

func TestPackageAddWithNamespaceSetDotnet(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.ImportDirectory("packageadd-namespace")
	e.CWD = filepath.Join(e.RootPath, "dotnet")
	stdout, _ := e.RunCommand("pulumi", "package", "add", "../provider/schema.json")
	require.Contains(t, stdout,
		"You can then use the SDK in your .NET code with:\n\n  using MyNamespace.Mypkg;")

	// Make sure the SDK was generated in the expected directory
	_, err := os.Stat(filepath.Join(e.CWD, "sdks", "my-namespace-mypkg", "MyNamespace.Mypkg.csproj"))
	require.NoError(t, err)
}
