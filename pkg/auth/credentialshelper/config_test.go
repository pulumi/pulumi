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

package credentialshelper

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func helperExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, name)
	// Discovery must accept this file without trying to execute its contents.
	require.NoError(t, os.WriteFile(path, []byte("not a program"), 0o700)) //nolint:gosec // Executable discovery fixture.
	path, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	return path
}

func TestResolvePrecedence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cli := helperExecutable(t, filepath.Join(dir, "cli"), "pulumi")
	adjacent := helperExecutable(t, filepath.Dir(cli), "pulumi-credentials-helper")
	onPath := helperExecutable(t, filepath.Join(dir, "bin"), "pulumi-credentials-helper")
	configured := helperExecutable(t, filepath.Join(dir, "custom"), "helper")
	saved := &workspace.CredentialHelper{Path: configured, Args: []string{"--saved"}}

	for _, tt := range []struct {
		name   string
		config env.MapStore
		saved  *workspace.CredentialHelper
		want   *Resolved
	}{
		{
			name: "environment replaces saved path and arguments",
			config: env.MapStore{
				helperEnvVar: onPath,
				argsEnvVar:   `["--profile","with spaces","$(literal)",""]`,
			},
			saved: saved,
			want: &Resolved{
				CredentialHelper: workspace.CredentialHelper{
					Path: onPath, Args: []string{"--profile", "with spaces", "$(literal)", ""},
				},
				Source: SourceEnvironment,
			},
		},
		{
			name:   "environment does not inherit saved arguments",
			config: env.MapStore{helperEnvVar: configured},
			saved:  saved,
			want:   &Resolved{CredentialHelper: workspace.CredentialHelper{Path: configured}, Source: SourceEnvironment},
		},
		{
			name:   "saved overrides discovery and ignores environment arguments alone",
			config: env.MapStore{argsEnvVar: "invalid JSON", "PATH": filepath.Dir(onPath)},
			saved:  saved,
			want:   &Resolved{CredentialHelper: *saved, Source: SourceSaved},
		},
		{
			name:   "adjacent overrides PATH",
			config: env.MapStore{"PATH": filepath.Dir(onPath)},
			want:   &Resolved{CredentialHelper: workspace.CredentialHelper{Path: adjacent}, Source: SourceExecutable},
		},
		{
			name:   "none disables everything",
			config: env.MapStore{helperEnvVar: "none", argsEnvVar: "invalid JSON", "PATH": filepath.Dir(onPath)},
			saved:  &workspace.CredentialHelper{Path: "invalid saved path"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resolved, err := Resolve(ResolveOptions{
				Environment: env.NewEnv(tt.config), Saved: tt.saved, Executable: cli, WorkingDirectory: dir,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.want, resolved)
		})
	}
}

func TestResolvePaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cli := helperExecutable(t, filepath.Join(dir, "cli"), "pulumi")
	first := helperExecutable(t, filepath.Join(dir, "first"), "pulumi-credentials-helper")
	second := helperExecutable(t, filepath.Join(dir, "second"), "pulumi-credentials-helper")
	path := strings.Join([]string{filepath.Join(dir, "missing"), filepath.Dir(first), filepath.Dir(second)},
		string(os.PathListSeparator))
	for _, tt := range []struct {
		name   string
		config env.MapStore
		want   *Resolved
	}{
		{
			name:   "PATH discovery uses the first match",
			config: env.MapStore{"PATH": path, argsEnvVar: `["ignored"]`},
			want:   &Resolved{CredentialHelper: workspace.CredentialHelper{Path: first}, Source: SourcePath},
		},
		{
			name:   "configured bare name uses PATH",
			config: env.MapStore{helperEnvVar: "pulumi-credentials-helper", "PATH": path},
			want:   &Resolved{CredentialHelper: workspace.CredentialHelper{Path: first}, Source: SourceEnvironment},
		},
		{
			name:   "relative configured path uses working directory",
			config: env.MapStore{helperEnvVar: filepath.Join("second", filepath.Base(second))},
			want:   &Resolved{CredentialHelper: workspace.CredentialHelper{Path: second}, Source: SourceEnvironment},
		},
		{
			name:   "missing helper continues normally",
			config: env.MapStore{"PATH": filepath.Join(dir, "missing")},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resolved, err := Resolve(ResolveOptions{
				Environment: env.NewEnv(tt.config), Executable: cli, WorkingDirectory: dir,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.want, resolved)
		})
	}
}

func TestResolveSymlinks(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires elevated privileges on Windows")
	}
	dir := t.TempDir()
	cli := helperExecutable(t, filepath.Join(dir, "installed"), "pulumi")
	helper := helperExecutable(t, filepath.Join(dir, "helpers"), "helper")
	launcher := helperExecutable(t, filepath.Join(dir, "bin"), "unrelated")
	cliLink := filepath.Join(filepath.Dir(launcher), "pulumi")
	require.NoError(t, os.Symlink(cli, cliLink))
	helperLink := filepath.Join(filepath.Dir(cli), "pulumi-credentials-helper")
	require.NoError(t, os.Symlink(helper, helperLink))
	helperExecutable(t, filepath.Dir(launcher), "pulumi-credentials-helper")

	resolved, err := Resolve(ResolveOptions{
		Environment: env.NewEnv(env.MapStore{}), Executable: cliLink, WorkingDirectory: dir,
	})
	require.NoError(t, err)
	assert.Equal(t, &Resolved{
		CredentialHelper: workspace.CredentialHelper{Path: helper}, Source: SourceExecutable,
	}, resolved)

	saved := &workspace.CredentialHelper{Path: helperLink, Args: []string{"--saved"}}
	resolved, err = Resolve(ResolveOptions{Environment: env.NewEnv(env.MapStore{}), Saved: saved})
	require.NoError(t, err)
	assert.Equal(t, helper, resolved.Path)
	assert.Equal(t, SourceSaved, resolved.Source)
	resolved.Args[0] = "changed"
	assert.Equal(t, []string{"--saved"}, saved.Args)
}

func TestResolveInvalidConfiguration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	helper := helperExecutable(t, dir, "helper")
	for _, tt := range []struct {
		name   string
		config env.MapStore
		saved  *workspace.CredentialHelper
	}{
		{name: "missing configured command", config: env.MapStore{helperEnvVar: "missing"}},
		{name: "missing saved file", saved: &workspace.CredentialHelper{Path: filepath.Join(dir, "missing")}},
		{name: "relative saved path", saved: &workspace.CredentialHelper{Path: "./helper"}},
		{name: "directory", config: env.MapStore{helperEnvVar: dir}},
		{name: "NUL saved argument", saved: &workspace.CredentialHelper{Path: helper, Args: []string{"secret\x00"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resolved, err := Resolve(ResolveOptions{Environment: env.NewEnv(tt.config), Saved: tt.saved})
			require.Error(t, err)
			assert.Nil(t, resolved)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
	for _, args := range []string{
		`null`, `{}`, `"secret"`, `["secret",null]`, `["secret",1]`,
		`["secret"] []`, `["secret\u0000"]`, `secret`,
	} {
		t.Run(args, func(t *testing.T) {
			t.Parallel()
			resolved, err := Resolve(ResolveOptions{
				Environment: env.NewEnv(env.MapStore{helperEnvVar: helper, argsEnvVar: args}),
			})
			require.Error(t, err)
			assert.Nil(t, resolved)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
}

func TestResolveUnusableDiscoveredHelper(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("executable permission bits and unprivileged symlinks are not available on Windows")
	}
	for _, besideCLI := range []bool{true, false} {
		for _, brokenSymlink := range []bool{true, false} {
			dir := t.TempDir()
			cli := helperExecutable(t, filepath.Join(dir, "cli"), "pulumi")
			onPath := helperExecutable(t, filepath.Join(dir, "bin"), "pulumi-credentials-helper")
			candidate := onPath
			if besideCLI {
				candidate = helperExecutable(t, filepath.Dir(cli), "pulumi-credentials-helper")
			}
			if brokenSymlink {
				require.NoError(t, os.Remove(candidate))
				require.NoError(t, os.Symlink(filepath.Join(dir, "missing"), candidate))
			} else {
				require.NoError(t, os.Chmod(candidate, 0o600))
			}
			resolved, err := Resolve(ResolveOptions{
				Environment: env.NewEnv(env.MapStore{"PATH": filepath.Dir(onPath)}), Executable: cli,
			})
			require.Error(t, err)
			assert.Nil(t, resolved)
		}
	}
}
