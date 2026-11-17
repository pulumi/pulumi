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

package do

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// TestMain also runs this test executable as a credential helper or plugin child.
// Tests use os.Executable() with -credential-helper-test, followed by either "credentials" and a JSON response
// or "environment" and a log path. Other invocations run the tests normally.
func TestMain(m *testing.M) {
	for i, arg := range os.Args {
		if arg != "-credential-helper-test" {
			continue
		}
		args := os.Args[i+1:]
		if len(args) != 2 {
			os.Exit(1)
		}
		switch args[0] {
		case "credentials":
			if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
				os.Exit(1)
			}
			if _, err := io.WriteString(os.Stdout, args[1]); err != nil {
				os.Exit(1)
			}
		case "environment":
			credentials := map[string]string{}
			for _, name := range []string{"PULUMI_API", "PULUMI_ACCESS_TOKEN"} {
				if value, ok := os.LookupEnv(name); ok {
					credentials[name] = value
				}
			}
			data, err := json.Marshal(credentials)
			if err != nil {
				os.Exit(1)
			}
			if err := os.WriteFile(args[1], data, 0o600); err != nil {
				os.Exit(1)
			}
		default:
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func newTestWorkspace(t *testing.T) *pkgWorkspace.MockContext {
	t.Helper()

	root := t.TempDir()
	return &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) {
			return &workspace.Project{
				Name:    tokens.PackageName("test-project"),
				Runtime: workspace.NewProjectRuntimeInfo("yaml", nil),
			}, root, nil
		},
	}
}
