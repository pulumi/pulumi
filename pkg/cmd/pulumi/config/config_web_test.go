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

package config

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// fakeConsoleBackend is a MockBackend that also implements consoleURLProvider, recording the path
// segments passed to CloudConsoleURL and joining them onto a configurable base.
type fakeConsoleBackend struct {
	backend.MockBackend
	base     string
	gotPaths []string
}

func (b *fakeConsoleBackend) CloudConsoleURL(paths ...string) string {
	b.gotPaths = paths
	return b.base + "/" + strings.Join(paths, "/")
}

func consoleStack(t *testing.T, be backend.Backend, remote bool) *backend.MockStack {
	t.Helper()
	env := "envProject/envName"
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("testStack")}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			if remote {
				return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
			}
			return backend.StackConfigLocation{}
		},
		OrgNameF: func() string { return "myorg" },
		BackendF: func() backend.Backend { return be },
	}
}

func TestConfigEnvConsoleLocalRejected(t *testing.T) {
	t.Parallel()
	be := &fakeConsoleBackend{base: "https://app.pulumi.com"}
	cmd := &configEnvConsoleCmd{stdout: &bytes.Buffer{}, openBrowser: func(string) error { return nil }}
	err := cmd.runWithStack(consoleStack(t, be, false), "")
	require.ErrorContains(t, err, "only supported for remote config stacks")
}

func TestConfigEnvConsoleConfigFileRejected(t *testing.T) {
	t.Parallel()
	// An explicit --config-file forces the local store even on a linked remote stack, so the remote
	// console must be rejected just like a plain local stack.
	be := &fakeConsoleBackend{base: "https://app.pulumi.com"}
	cmd := &configEnvConsoleCmd{stdout: &bytes.Buffer{}, openBrowser: func(string) error { return nil }}
	err := cmd.runWithStack(consoleStack(t, be, true), "Pulumi.foo.yaml")
	require.ErrorContains(t, err, "only supported for remote config stacks")
}

func TestConfigEnvConsoleRequiresCloudBackend(t *testing.T) {
	t.Parallel()
	// A plain MockBackend does not implement consoleURLProvider.
	be := &backend.MockBackend{}
	cmd := &configEnvConsoleCmd{stdout: &bytes.Buffer{}, openBrowser: func(string) error { return nil }}
	err := cmd.runWithStack(consoleStack(t, be, true), "")
	require.ErrorContains(t, err, "requires a Pulumi Cloud backend")
}

func TestConfigEnvConsoleURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		base string
	}{
		{"default cloud", "https://app.pulumi.com"},
		{"custom cloud", "https://pulumi.example.com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			be := &fakeConsoleBackend{base: c.base}
			var opened string
			var out bytes.Buffer
			cmd := &configEnvConsoleCmd{
				stdout:      &out,
				openBrowser: func(url string) error { opened = url; return nil },
			}
			require.NoError(t, cmd.runWithStack(consoleStack(t, be, true), ""))

			require.Equal(t, []string{"myorg", "esc", "envProject", "envName"}, be.gotPaths)
			want := c.base + "/myorg/esc/envProject/envName"
			require.Equal(t, want, opened)
			require.Contains(t, out.String(), want)
			require.NotContains(t, opened, "token=")
			require.NotContains(t, opened, "access_token=")
		})
	}
}

func TestConfigEnvConsoleBrowserFailurePrintsURL(t *testing.T) {
	t.Parallel()
	be := &fakeConsoleBackend{base: "https://app.pulumi.com"}
	var out bytes.Buffer
	cmd := &configEnvConsoleCmd{
		stdout:      &out,
		openBrowser: func(string) error { return errors.New("browser failed") },
	}
	require.NoError(t, cmd.runWithStack(consoleStack(t, be, true), ""))
	require.Contains(t, out.String(), "https://app.pulumi.com/myorg/esc/envProject/envName")
	require.Contains(t, out.String(), "open the URL above manually")
}
