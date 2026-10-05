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

package credentialhelper

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

const (
	helperEnvVar = "PULUMI_CREDENTIAL_HELPER"
	argsEnvVar   = "PULUMI_CREDENTIAL_HELPER_ARGS"
)

// setEnvironment replaces the helper configuration and PATH for one test.
func setEnvironment(t *testing.T, config map[string]string) {
	t.Helper()
	for _, name := range []string{helperEnvVar, argsEnvVar, "PATH"} {
		t.Setenv(name, config[name])
	}
}

//nolint:paralleltest // Helper configuration and PATH are process-wide environment variables.
func TestResolveHelperPrecedence(t *testing.T) {
	dir := t.TempDir()
	cli := helperExecutable(t, filepath.Join(dir, "cli"), "pulumi")
	adjacent := helperExecutable(t, filepath.Dir(cli), "pulumi-credential-helper")
	onPath := helperExecutable(t, filepath.Join(dir, "bin"), "pulumi-credential-helper")
	configured := helperExecutable(t, filepath.Join(dir, "custom"), "helper")
	saved := &workspace.CredentialHelper{Path: configured, Args: []string{"--saved"}}

	for _, tt := range []struct {
		name     string
		config   map[string]string
		explicit *workspace.CredentialHelper
		saved    *workspace.CredentialHelper
		want     *Resolved
	}{
		{
			name:     "explicit flags override environment disablement and arguments",
			config:   map[string]string{helperEnvVar: "none", argsEnvVar: "invalid JSON"},
			explicit: &workspace.CredentialHelper{Path: configured, Args: []string{"with spaces,commas", "$(literal)", ""}},
			saved:    saved,
			want: &Resolved{
				CredentialHelper: workspace.CredentialHelper{
					Path: configured, Args: []string{"with spaces,commas", "$(literal)", ""},
				},
				Source: SourceExplicit,
			},
		},
		{
			name:     "explicit flags replace environment helper and do not inherit arguments",
			config:   map[string]string{helperEnvVar: onPath, argsEnvVar: `["--environment"]`},
			explicit: &workspace.CredentialHelper{Path: configured},
			want:     &Resolved{CredentialHelper: workspace.CredentialHelper{Path: configured}, Source: SourceExplicit},
		},
		{
			name: "environment replaces saved path and arguments",
			config: map[string]string{
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
			config: map[string]string{helperEnvVar: configured},
			saved:  saved,
			want:   &Resolved{CredentialHelper: workspace.CredentialHelper{Path: configured}, Source: SourceEnvironment},
		},
		{
			name:   "saved overrides discovery and ignores environment arguments alone",
			config: map[string]string{argsEnvVar: "invalid JSON", "PATH": filepath.Dir(onPath)},
			saved:  saved,
			want:   &Resolved{CredentialHelper: *saved, Source: SourceSaved},
		},
		{
			name:   "adjacent overrides PATH",
			config: map[string]string{"PATH": filepath.Dir(onPath)},
			want:   &Resolved{CredentialHelper: workspace.CredentialHelper{Path: adjacent}, Source: SourceExecutable},
		},
		{
			name:   "none disables everything",
			config: map[string]string{helperEnvVar: "none", argsEnvVar: "invalid JSON", "PATH": filepath.Dir(onPath)},
			saved:  &workspace.CredentialHelper{Path: "invalid saved path"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setEnvironment(t, tt.config)
			resolved, err := ResolveHelper(ResolveHelperOptions{Explicit: tt.explicit, Saved: tt.saved, Executable: cli})
			require.NoError(t, err)
			assert.Equal(t, tt.want, resolved)
		})
	}
}

//nolint:paralleltest // Helper configuration and PATH are process-wide environment variables.
func TestResolveHelperPaths(t *testing.T) {
	dir := t.TempDir()
	cli := helperExecutable(t, filepath.Join(dir, "cli"), "pulumi")
	first := helperExecutable(t, filepath.Join(dir, "first"), "pulumi-credential-helper")
	second := helperExecutable(t, filepath.Join(dir, "second"), "pulumi-credential-helper")
	path := strings.Join([]string{filepath.Join(dir, "missing"), filepath.Dir(first), filepath.Dir(second)},
		string(os.PathListSeparator))
	for _, tt := range []struct {
		name   string
		config map[string]string
		want   *Resolved
	}{
		{
			name:   "PATH discovery uses the first match",
			config: map[string]string{"PATH": path, argsEnvVar: `["ignored"]`},
			want:   &Resolved{CredentialHelper: workspace.CredentialHelper{Path: first}, Source: SourcePath},
		},
		{
			name:   "configured bare name uses PATH",
			config: map[string]string{helperEnvVar: "pulumi-credential-helper", "PATH": path},
			want:   &Resolved{CredentialHelper: workspace.CredentialHelper{Path: first}, Source: SourceEnvironment},
		},
		{
			name:   "relative configured path uses working directory",
			config: map[string]string{helperEnvVar: filepath.Join("second", filepath.Base(second))},
			want:   &Resolved{CredentialHelper: workspace.CredentialHelper{Path: second}, Source: SourceEnvironment},
		},
		{
			name:   "missing helper continues normally",
			config: map[string]string{"PATH": filepath.Join(dir, "missing")},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setEnvironment(t, tt.config)
			t.Chdir(dir)
			resolved, err := ResolveHelper(ResolveHelperOptions{Executable: cli})
			require.NoError(t, err)
			assert.Equal(t, tt.want, resolved)
		})
	}
}

//nolint:paralleltest // Helper configuration and PATH are process-wide environment variables.
func TestResolveHelperSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires elevated privileges on Windows")
	}
	dir := t.TempDir()
	cli := helperExecutable(t, filepath.Join(dir, "installed"), "pulumi")
	helper := helperExecutable(t, filepath.Join(dir, "helpers"), "helper")
	launcher := helperExecutable(t, filepath.Join(dir, "bin"), "unrelated")
	cliLink := filepath.Join(filepath.Dir(launcher), "pulumi")
	require.NoError(t, os.Symlink(cli, cliLink))
	helperLink := filepath.Join(filepath.Dir(cli), "pulumi-credential-helper")
	require.NoError(t, os.Symlink(helper, helperLink))
	helperExecutable(t, filepath.Dir(launcher), "pulumi-credential-helper")

	setEnvironment(t, nil)
	resolved, err := ResolveHelper(ResolveHelperOptions{Executable: cliLink})
	require.NoError(t, err)
	assert.Equal(t, &Resolved{
		CredentialHelper: workspace.CredentialHelper{Path: helper}, Source: SourceExecutable,
	}, resolved)

	saved := &workspace.CredentialHelper{Path: helperLink, Args: []string{"--saved"}}
	resolved, err = ResolveHelper(ResolveHelperOptions{Saved: saved})
	require.NoError(t, err)
	assert.Equal(t, helper, resolved.Path)
	assert.Equal(t, SourceSaved, resolved.Source)
	resolved.Args[0] = "changed"
	assert.Equal(t, []string{"--saved"}, saved.Args)
}

//nolint:paralleltest // Helper configuration and PATH are process-wide environment variables.
func TestResolveHelperInvalidConfiguration(t *testing.T) {
	dir := t.TempDir()
	helper := helperExecutable(t, dir, "helper")
	for _, tt := range []struct {
		name   string
		config map[string]string
		saved  *workspace.CredentialHelper
	}{
		{name: "missing configured command", config: map[string]string{helperEnvVar: "missing"}},
		{name: "missing saved file", saved: &workspace.CredentialHelper{Path: filepath.Join(dir, "missing")}},
		{name: "relative saved path", saved: &workspace.CredentialHelper{Path: "./helper"}},
		{name: "directory", config: map[string]string{helperEnvVar: dir}},
		{name: "NUL saved argument", saved: &workspace.CredentialHelper{Path: helper, Args: []string{"secret\x00"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setEnvironment(t, tt.config)
			resolved, err := ResolveHelper(ResolveHelperOptions{Saved: tt.saved})
			require.Error(t, err)
			assert.Nil(t, resolved)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
	for _, args := range []string{
		`null`, `{}`, `"secret"`, `["secret",1]`,
		`["secret"] []`, `["secret\u0000"]`, `secret`,
	} {
		t.Run(args, func(t *testing.T) {
			setEnvironment(t, map[string]string{
				helperEnvVar: helper, argsEnvVar: args,
			})
			resolved, err := ResolveHelper(ResolveHelperOptions{})
			require.Error(t, err)
			assert.Nil(t, resolved)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
}

//nolint:paralleltest // Helper configuration and PATH are process-wide environment variables.
func TestResolveHelperUnusableDiscoveredHelper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable permission bits and unprivileged symlinks are not available on Windows")
	}
	for _, besideCLI := range []bool{true, false} {
		for _, brokenSymlink := range []bool{true, false} {
			dir := t.TempDir()
			cli := helperExecutable(t, filepath.Join(dir, "cli"), "pulumi")
			onPath := helperExecutable(t, filepath.Join(dir, "bin"), "pulumi-credential-helper")
			candidate := onPath
			if besideCLI {
				candidate = helperExecutable(t, filepath.Dir(cli), "pulumi-credential-helper")
			}
			if brokenSymlink {
				require.NoError(t, os.Remove(candidate))
				require.NoError(t, os.Symlink(filepath.Join(dir, "missing"), candidate))
			} else {
				require.NoError(t, os.Chmod(candidate, 0o600))
			}
			setEnvironment(t, map[string]string{"PATH": filepath.Dir(onPath)})
			resolved, err := ResolveHelper(ResolveHelperOptions{Executable: cli})
			assert.Nil(t, resolved)
			// Like a shell, PATH lookup skips files that cannot be executed.
			if besideCLI {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		}
	}
}
