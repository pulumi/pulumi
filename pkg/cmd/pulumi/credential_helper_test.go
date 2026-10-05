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

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
)

func TestMain(m *testing.M) {
	if strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe") == "pulumi-credential-helper" ||
		(len(os.Args) > 1 && os.Args[1] == "--credential-helper-test") {
		var request credentialhelper.Request
		if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprint(os.Stderr, os.Getenv("PULUMI_TEST_HELPER_STDERR"))
		fmt.Fprintln(os.Stdout, os.Getenv("PULUMI_TEST_HELPER_"+strings.ToUpper(string(request.Reason))))
		os.Exit(0)
	}
	if len(os.Args) > 1 && os.Args[1] == "--pulumi-command-test" {
		root, cleanup := NewPulumiCmd()
		root.SetArgs(os.Args[2:])
		err := root.Execute()
		cleanup()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runCredentialCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	t.Setenv("PULUMI_SKIP_UPDATE_CHECK", "true")
	command := exec.CommandContext(t.Context(), executable, append([]string{
		"--pulumi-command-test", "--non-interactive",
	}, args...)...)
	output, err := command.CombinedOutput()
	return string(output), err
}

func TestCredentialHelperLifecycle(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Chdir(t.TempDir())
	t.Setenv("PULUMI_CREDENTIAL_HELPER", filepath.Join(t.TempDir(), "missing-helper"))
	t.Setenv("PULUMI_TEST_HELPER_INITIAL", `{"version":1,"env":{"HELPER_TEST_VALUE":"ephemeral"}}`)
	t.Setenv("PULUMI_TEST_HELPER_STDERR", "helper diagnostic\n")
	executable, err := os.Executable()
	require.NoError(t, err)
	name := "pulumi-credential-helper"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	dir := t.TempDir()
	helper := filepath.Join(dir, name)
	if err := os.Link(executable, helper); err != nil {
		require.NoError(t, fsutil.CopyFile(helper, executable, nil))
	}
	t.Setenv("PATH", dir)

	output, err := runCredentialCommand(t, "version")
	require.NoError(t, err, output)
	assert.NotContains(t, output, "helper diagnostic")
	t.Setenv("PULUMI_CREDENTIAL_HELPER", "")
	backendURL := "file://" + filepath.ToSlash(t.TempDir())
	output, err = runCredentialCommand(t, "login", backendURL)
	require.NoError(t, err, output)
	assert.Equal(t, 1, strings.Count(output, "helper diagnostic"))
}
