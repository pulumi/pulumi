// Copyright 2016, Pulumi Corporation.
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
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdDo "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/do"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsLocalVersion(t *testing.T) {
	t.Parallel()

	// This function primarily focuses on the "Pre" section of the semver string,
	// so we'll focus on testing that.
	stableVer, _ := semver.ParseTolerant("1.0.0")
	devVer, _ := semver.ParseTolerant("v1.0.0-dev")
	alphaVer, _ := semver.ParseTolerant("v1.0.0-alpha.1590772212+g4ff08363.dirty")
	betaVer, _ := semver.ParseTolerant("v1.0.0-beta.1590772212")
	rcVer, _ := semver.ParseTolerant("v1.0.0-rc.1")

	assert.False(t, isLocalVersion(stableVer))
	assert.True(t, isLocalVersion(devVer))
	assert.True(t, isLocalVersion(alphaVer))
	assert.True(t, isLocalVersion(betaVer))
	assert.True(t, isLocalVersion(rcVer))
}

func TestIsDevVersion(t *testing.T) {
	t.Parallel()

	stableVer, _ := semver.ParseTolerant("1.0.0")
	devVer, _ := semver.ParseTolerant("v1.0.0-11-g4ff08363")

	assert.False(t, isDevVersion(stableVer))
	assert.True(t, isDevVersion(devVer))
}

func TestHaveNewerDevVersion(t *testing.T) {
	t.Parallel()

	devVer, _ := semver.ParseTolerant("v1.0.0-11-g4ff08363")
	olderCurVer, _ := semver.ParseTolerant("v1.0.0-10-gdeadbeef")
	newerCurVer, _ := semver.ParseTolerant("v1.0.0-12-gdeadbeef")
	newerPatchCurVer, _ := semver.ParseTolerant("v1.0.1-11-gdeadbeef")
	olderMajorCurVer, _ := semver.ParseTolerant("v0.9.9-11-gdeadbeef")

	assert.True(t, haveNewerDevVersion(devVer, olderCurVer))
	assert.True(t, haveNewerDevVersion(devVer, olderMajorCurVer))
	assert.False(t, haveNewerDevVersion(devVer, devVer))
	assert.False(t, haveNewerDevVersion(devVer, newerCurVer))
	assert.False(t, haveNewerDevVersion(devVer, newerPatchCurVer))
}

//nolint:paralleltest // changes environment variables and globals
func TestGetCLIVersionInfo_Simple(t *testing.T) {
	// Arrange.
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cli/version":
			callCount++
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"latestVersion": "v1.2.3",
				"oldestWithoutWarning": "v1.2.0"
			}`))
			require.NoError(t, err)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Act.
	latestVer, oldestAllowedVer, devVer, err := getCLIVersionInfo(ctx, srv.URL, nil)

	// Assert.
	require.NoError(t, err)
	require.Equal(t, 1, callCount, "should have called API once")
	require.Equal(t, "1.2.3", latestVer.String())
	require.Equal(t, "1.2.0", oldestAllowedVer.String())
	require.Equal(t, "0.0.0", devVer.String())
}

//nolint:paralleltest // changes environment variables and globals
func TestGetCLIVersionInfo_TimesOut(t *testing.T) {
	// Arrange.
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cli/version":
			callCount++
			time.Sleep(4 * time.Second)
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"latestVersion": "v1.2.3",
				"oldestWithoutWarning": "v1.2.0"
			}`))
			require.NoError(t, err)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Act.
	_, _, _, err := getCLIVersionInfo(ctx, srv.URL, nil)

	// Assert.
	require.ErrorContains(t, err, "context deadline exceeded")
}

//nolint:paralleltest // changes environment variables and globals
func TestGetCLIVersionInfo_SendsMetadataToPulumiCloud(t *testing.T) {
	// Arrange.
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	metadata := map[string]string{
		"Command": "test-command",
		"Flags":   "--foo",
	}

	token := time.Now().String()

	called := false
	authHeader := ""
	commandHeader := ""
	flagsHeader := ""

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cli/version":
			called = true

			authHeader = r.Header.Get("Authorization")
			commandHeader = r.Header.Get("X-Pulumi-Command")
			flagsHeader = r.Header.Get("X-Pulumi-Flags")

			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"latestVersion": "v1.2.3",
				"oldestWithoutWarning": "v1.2.0"
			}`))
			require.NoError(t, err)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	err := workspace.StoreCredentials(workspace.Credentials{
		Current: srv.URL,
		Accounts: map[string]workspace.Account{
			srv.URL: {
				AccessToken: token,
			},
		},
		AccessTokens: map[string]string{
			srv.URL: token,
		},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Act.
	_, _, _, err = getCLIVersionInfo(ctx, srv.URL, metadata)

	// Assert.
	require.NoError(t, err)
	require.True(t, called, "should have called API")
	require.Equal(t, "token "+token, authHeader)
	require.Equal(t, metadata["Command"], commandHeader)
	require.Equal(t, metadata["Flags"], flagsHeader)
}

//nolint:paralleltest // changes environment variables and globals
func TestGetCLIVersionInfo_DoesNotSendMetadataToOtherBackends(t *testing.T) {
	// Arrange.
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	metadata := map[string]string{
		"Command": "test-command",
		"Flags":   "--foo",
	}

	token := time.Now().String()

	called := false

	authHeader := ""
	commandHeader := ""
	flagsHeader := ""

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cli/version":
			called = true

			authHeader = r.Header.Get("Authorization")
			commandHeader = r.Header.Get("X-Pulumi-Command")
			flagsHeader = r.Header.Get("X-Pulumi-Flags")

			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"latestVersion": "v1.2.3",
				"oldestWithoutWarning": "v1.2.0"
			}`))
			require.NoError(t, err)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	err := workspace.StoreCredentials(workspace.Credentials{
		Current: "https://example.com",
		Accounts: map[string]workspace.Account{
			srv.URL: {
				AccessToken: token,
			},
		},
		AccessTokens: map[string]string{
			srv.URL: token,
		},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Act.
	_, _, _, err = getCLIVersionInfo(ctx, srv.URL, metadata)

	// Assert.
	require.NoError(t, err)
	require.True(t, called, "should have called API")
	require.Empty(t, authHeader)
	require.Empty(t, commandHeader)
	require.Empty(t, flagsHeader)
}

func TestGetCLIMetadata(t *testing.T) {
	t.Parallel()

	// Arrange.
	cases := []struct {
		name     string
		cmd      *cobra.Command
		environ  []string
		args     []string
		metadata map[string]string
	}{
		{
			name:     "nil",
			cmd:      nil,
			metadata: nil,
			environ:  nil,
		},
		{
			name: "no set flags",
			cmd: (func() *cobra.Command {
				cmd := &cobra.Command{Use: "no-set"}
				cmd.Flags().Bool("bool", false, "bool flag")
				cmd.Flags().String("string", "", "string flag")
				return cmd
			})(),
			environ: []string{},
			metadata: map[string]string{
				"Command":     "no-set",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "one set bool flag",
			cmd: (func() *cobra.Command {
				cmd := &cobra.Command{Use: "one-set"}
				cmd.Flags().Bool("bool", false, "bool flag")
				cmd.Flags().String("string", "", "string flag")

				cmd.SetArgs([]string{"--bool"})

				err := cmd.Execute()
				require.NoError(t, err)

				return cmd
			})(),
			metadata: map[string]string{
				"Command":     "one-set",
				"Flags":       "--bool",
				"Environment": "",
			},
		},
		{
			name: "one set string flag",
			cmd: (func() *cobra.Command {
				cmd := &cobra.Command{Use: "one-set"}
				cmd.Flags().Bool("bool", false, "bool flag")
				cmd.Flags().String("string", "", "string flag")

				cmd.SetArgs([]string{"--string=value"})

				err := cmd.Execute()
				require.NoError(t, err)

				return cmd
			})(),
			metadata: map[string]string{
				"Command":     "one-set",
				"Flags":       "--string",
				"Environment": "",
			},
		},
		{
			name: "multiple set flags",
			cmd: (func() *cobra.Command {
				cmd := &cobra.Command{Use: "multiple-set"}
				cmd.Flags().Bool("bool", false, "bool flag")
				cmd.Flags().String("string", "", "string flag")

				cmd.SetArgs([]string{"--string=value", "--bool"})

				err := cmd.Execute()
				require.NoError(t, err)

				return cmd
			})(),
			metadata: map[string]string{
				"Command":     "multiple-set",
				"Flags":       "--bool --string",
				"Environment": "",
			},
		},
		{
			name: "longer command path",
			cmd: (func() *cobra.Command {
				parent := &cobra.Command{Use: "parent"}
				err := parent.Execute()
				require.NoError(t, err)

				cmd := &cobra.Command{Use: "multiple-set"}
				parent.AddCommand(cmd)

				return cmd
			})(),
			metadata: map[string]string{
				"Command":     "parent multiple-set",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "no valid PULUMI_ env variables",
			cmd: (func() *cobra.Command {
				cmd := &cobra.Command{Use: "version"}
				err := cmd.Execute()
				require.NoError(t, err)
				return cmd
			})(),
			environ: []string{"PULUMICOPILOT=true", "OTHER_FLAG=true", "PULUMI_NO_EQUALS_SIGN"},
			metadata: map[string]string{
				"Command":     "version",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "has valid PULUMI_ env variables",
			cmd: (func() *cobra.Command {
				cmd := &cobra.Command{Use: "version"}
				err := cmd.Execute()
				require.NoError(t, err)
				return cmd
			})(),
			environ: []string{"PULUMI_EXPERIMENTAL=true", "PULUMI_COPILOT=true"},
			metadata: map[string]string{
				"Command":     "version",
				"Flags":       "",
				"Environment": "PULUMI_EXPERIMENTAL PULUMI_COPILOT",
			},
		},
		{
			name: "do with token and operation",
			cmd:  newDoTestCmd(),
			args: []string{"aws:s3:Bucket", "list"},
			metadata: map[string]string{
				"Command":     "pulumi do aws:s3:Bucket list",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "do with flags mixed in",
			cmd:  newDoTestCmd(),
			args: []string{"--dry-run", "aws:s3:Bucket", "create", "--some-unknown-flag", "secret-value"},
			metadata: map[string]string{
				"Command":     "pulumi do aws:s3:Bucket create",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "do with package flag",
			cmd:  newDoTestCmd(),
			args: []string{"--package", "aws", "aws:s3:Bucket", "list"},
			metadata: map[string]string{
				"Command":     "pulumi do aws:s3:Bucket list",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "do drops extra positionals after verb",
			cmd:  newDoTestCmd(),
			args: []string{"aws:s3:Bucket", "read", "some-resource-id"},
			metadata: map[string]string{
				"Command":     "pulumi do aws:s3:Bucket read",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "do drops unknown verb",
			cmd:  newDoTestCmd(),
			args: []string{"aws:lambda:Function", "someUnknownVerb"},
			metadata: map[string]string{
				"Command":     "pulumi do aws:lambda:Function",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "do with no args",
			cmd:  newDoTestCmd(),
			args: []string{},
			metadata: map[string]string{
				"Command":     "pulumi do",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "plugin run with argument",
			cmd: (func() *cobra.Command {
				cmd := &cobra.Command{Use: "pulumi"}
				pluginCmd := &cobra.Command{Use: "plugin"}
				cmd.AddCommand(pluginCmd)
				pluginRunCmd := &cobra.Command{Use: "run", Args: cmdutil.MinimumNArgs(1)}
				pluginCmd.AddCommand(pluginRunCmd)
				err := pluginRunCmd.Execute()
				require.NoError(t, err)
				return pluginRunCmd
			})(),
			environ: []string{"PULUMI_EXPERIMENTAL=true", "PULUMI_COPILOT=true"},
			args:    []string{"my-plugin"},
			metadata: map[string]string{
				"Command":     "pulumi plugin run my-plugin",
				"Flags":       "",
				"Environment": "PULUMI_EXPERIMENTAL PULUMI_COPILOT",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			// Act.
			metadata := getCLIMetadata(c.cmd, c.environ, c.args)

			// Assert.
			require.Equal(t, c.metadata, metadata)
		})
	}
}

func newDoTestCmd() *cobra.Command {
	root := &cobra.Command{Use: "pulumi"}
	doCmd := cmdDo.NewDoCmd(nil, nil, nil, nil, nil, nil)
	root.AddCommand(doCmd)
	return doCmd
}

//nolint:paralleltest // changes environment variables and globals
func TestCheckForUpdate_AlwaysChecksVersion(t *testing.T) {
	// Arrange.
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cli/version":
			callCount++
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"latestVersion": "v1.2.3",
				"oldestWithoutWarning": "v1.2.0"
			}`))
			require.NoError(t, err)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Act.
	checkForUpdate(ctx, srv.URL, nil)
	checkForUpdate(ctx, srv.URL, nil)
	checkForUpdate(ctx, srv.URL, nil)

	// Assert.
	require.Equal(t, 3, callCount, "should call API every time")
}

//nolint:paralleltest // changes environment variables and globals
func TestCheckForUpdate_CachesPrompts(t *testing.T) {
	realVersion := version.Version
	t.Cleanup(func() {
		version.Version = realVersion
	})
	version.Version = "v1.0.0"

	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cli/version":
			callCount++
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"latestVersion": "v1.2.3",
				"oldestWithoutWarning": "v1.2.0"
			}`))
			require.NoError(t, err)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	uncached := checkForUpdate(ctx, srv.URL, nil)
	require.NotNil(t, uncached)
	require.NoError(t, cacheVersionInfo(uncached.versionInfo))

	cached := checkForUpdate(ctx, srv.URL, nil)
	require.Nil(t, cached)
	cachedAgain := checkForUpdate(ctx, srv.URL, nil)
	require.Nil(t, cachedAgain)

	// Store an expired last prompt timestamp
	expiredTime := time.Now().Add(-25 * time.Hour)
	info, err := readVersionInfo()
	require.NoError(t, err)
	info.LastPromptTimeStampMS = expiredTime.UnixMilli()
	require.NoError(t, cacheVersionInfo(info))

	expired := checkForUpdate(ctx, srv.URL, nil)

	require.Equal(t, 4, callCount, "should call API every time")

	require.Contains(t, uncached.diag.Message, "A new version of Pulumi is available")
	require.Contains(t, uncached.diag.Message, "upgrade from version '1.0.0' to '1.2.3'")

	require.Contains(t, expired.diag.Message, "A new version of Pulumi is available")
	require.Contains(t, expired.diag.Message, "upgrade from version '1.0.0' to '1.2.3'")
}

func TestCheckForUpdate_HandlesAPIFailures(t *testing.T) {
	// Arrange.
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cli/version":
			callCount++
			http.NotFound(w, r)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Act.
	first := checkForUpdate(ctx, srv.URL, nil)
	second := checkForUpdate(ctx, srv.URL, nil)

	// Assert.
	require.Equal(t, 2, callCount, "should call API every time")
	require.Nil(t, first)
	require.Nil(t, second)
}

//nolint:paralleltest // changes environment variables and globals
func TestCheckForUpdate_WorksCorrectlyWithDevVersions(t *testing.T) {
	realVersion := version.Version
	t.Cleanup(func() {
		version.Version = realVersion
	})
	version.Version = "v1.0.0-11-g4ff08363"

	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cli/version":
			callCount++
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"latestVersion": "v1.2.3",
				"oldestWithoutWarning": "v1.2.0",
				"latestDevVersion": "v1.0.0-12-gdeadbeef"
			}`))
			require.NoError(t, err)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	uncached := checkForUpdate(ctx, srv.URL, nil)
	require.NotNil(t, uncached)
	require.NoError(t, cacheVersionInfo(uncached.versionInfo))

	cached := checkForUpdate(ctx, srv.URL, nil)
	require.Nil(t, cached)

	cachedAgain := checkForUpdate(ctx, srv.URL, nil)
	require.Nil(t, cachedAgain)

	// Store an expired last prompt timesamp
	expiredTime := time.Now().Add(-2 * time.Hour)
	info, err := readVersionInfo()
	require.NoError(t, err)
	info.LastPromptTimeStampMS = expiredTime.UnixMilli()
	require.NoError(t, cacheVersionInfo(info))

	expired := checkForUpdate(ctx, srv.URL, nil)
	require.NotNil(t, expired)

	require.Equal(t, 4, callCount, "should call API every time")

	require.Contains(t, uncached.diag.Message, "A new version of Pulumi is available")
	require.Contains(t, uncached.diag.Message, "upgrade from version '1.0.0-11-g4ff08363' to '1.0.0-12-gdeadbeef'")

	require.Contains(t, expired.diag.Message, "A new version of Pulumi is available")
	require.Contains(t, expired.diag.Message, "upgrade from version '1.0.0-11-g4ff08363' to '1.0.0-12-gdeadbeef'")
}

//nolint:paralleltest // changes environment variables and globals
func TestCheckForUpdate_WorksCorrectlyWithLocalVersions(t *testing.T) {
	// Arrange.
	realVersion := version.Version
	t.Cleanup(func() {
		version.Version = realVersion
	})
	version.Version = "v1.0.0-beta.1590772212"

	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cli/version":
			callCount++
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"latestVersion": "v1.2.3",
				"oldestWithoutWarning": "v1.2.0",
				"latestDevVersion": "v1.0.0-12-gdeadbeef"
			}`))
			require.NoError(t, err)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Act.
	nilDiag := checkForUpdate(ctx, srv.URL, nil)
	stillNilDiag := checkForUpdate(ctx, srv.URL, nil)
	alwaysNilDiag := checkForUpdate(ctx, srv.URL, nil)

	// Assert.
	require.Equal(t, 0, callCount, "local versions don't trigger API calls")
	require.Nil(t, nilDiag)
	require.Nil(t, stillNilDiag)
	require.Nil(t, alwaysNilDiag)
}

//nolint:paralleltest // changes environment variables and globals
func TestCheckForUpdate_WorksCorrectlyWithDifferentMajorVersions(t *testing.T) {
	realVersion := version.Version
	t.Cleanup(func() {
		version.Version = realVersion
	})
	version.Version = "v1.0.0"

	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cli/version":
			callCount++
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"latestVersion": "v2.0.3",
				"oldestWithoutWarning": "v2.2.0",
				"latestDevVersion": "v2.0.0-12-gdeadbeef"
			}`))
			require.NoError(t, err)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	uncached := checkForUpdate(ctx, srv.URL, nil)
	require.NotNil(t, uncached)
	require.NoError(t, cacheVersionInfo(uncached.versionInfo))

	cached := checkForUpdate(ctx, srv.URL, nil)
	require.Nil(t, cached)

	cachedAgain := checkForUpdate(ctx, srv.URL, nil)
	require.Nil(t, cachedAgain)

	// Store an expired last prompt timesamp
	expiredTime := time.Now().Add(-25 * time.Hour)
	info, err := readVersionInfo()
	require.NoError(t, err)
	info.LastPromptTimeStampMS = expiredTime.UnixMilli()
	require.NoError(t, cacheVersionInfo(info))

	expired := checkForUpdate(ctx, srv.URL, nil)
	require.NotNil(t, expired)

	require.Equal(t, 4, callCount, "should call API every time")

	require.Contains(t, uncached.diag.Message, "A new version of Pulumi is available")
	require.Contains(t, uncached.diag.Message, "upgrade from version '1.0.0' to '2.0.3'")

	require.Contains(t, expired.diag.Message, "A new version of Pulumi is available")
	require.Contains(t, expired.diag.Message, "upgrade from version '1.0.0' to '2.0.3'")
}

//nolint:paralleltest // changes environment variables and globals
func TestCheckForUpdate_WorksCorrectlyWithVeryOldMinorVersions(t *testing.T) {
	realVersion := version.Version
	t.Cleanup(func() {
		version.Version = realVersion
	})
	version.Version = "v1.0.0"

	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cli/version":
			callCount++
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"latestVersion": "v1.40.3",
				"oldestWithoutWarning": "v1.40.0",
				"latestDevVersion": "v1.40.0-12-gdeadbeef"
			}`))
			require.NoError(t, err)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	uncached := checkForUpdate(ctx, srv.URL, nil)
	require.NotNil(t, uncached)
	require.NoError(t, cacheVersionInfo(uncached.versionInfo))

	cached := checkForUpdate(ctx, srv.URL, nil)
	require.Nil(t, cached)

	cachedAgain := checkForUpdate(ctx, srv.URL, nil)
	require.Nil(t, cachedAgain)

	// Store an expired last prompt timesamp
	expiredTime := time.Now().Add(-25 * time.Hour)
	info, err := readVersionInfo()
	require.NoError(t, err)
	info.LastPromptTimeStampMS = expiredTime.UnixMilli()
	require.NoError(t, cacheVersionInfo(info))

	expired := checkForUpdate(ctx, srv.URL, nil)
	require.NotNil(t, expired)

	require.Equal(t, 4, callCount, "should call API every time")

	require.Contains(t, uncached.diag.Message, "You are running a very old version of Pulumi")
	require.Contains(t, uncached.diag.Message, "upgrade from version '1.0.0' to '1.40.3'")

	require.Nil(t, cached)
	require.Nil(t, cachedAgain)

	require.Contains(t, expired.diag.Message, "You are running a very old version of Pulumi")
	require.Contains(t, expired.diag.Message, "upgrade from version '1.0.0' to '1.40.3'")
}

func TestDiffVersions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		v1        string
		v2        string
		minorDiff int64
	}{
		{
			v1:        "1.0.0",
			v2:        "1.0.0",
			minorDiff: 0,
		},
		{
			v1:        "1.0.0",
			v2:        "1.0.1",
			minorDiff: 0,
		},
		{
			v1:        "1.0.0",
			v2:        "1.1.0",
			minorDiff: 1,
		},
		{
			v1:        "1.0.0",
			v2:        "1.20.0",
			minorDiff: 20,
		},
		{
			v1:        "1.10.0",
			v2:        "1.20.0",
			minorDiff: 10,
		},
		{
			v1:        "1.0.0",
			v2:        "2.0.0",
			minorDiff: 0,
		},
		{
			v1:        "3.0.0",
			v2:        "2.0.0",
			minorDiff: 0,
		},
		{
			v1:        "1.0.0",
			v2:        "0.9.9",
			minorDiff: 0,
		},
		{
			v1:        "1.0.0",
			v2:        "1.0.0-rc.1",
			minorDiff: 0,
		},
		{
			v1:        "1.40.0",
			v2:        "1.20.0",
			minorDiff: -20,
		},
	}

	for _, c := range cases {
		t.Run(c.v1+" vs "+c.v2, func(t *testing.T) {
			t.Parallel()

			v1, err := semver.ParseTolerant(c.v1)
			require.NoError(t, err)

			v2, err := semver.ParseTolerant(c.v2)
			require.NoError(t, err)

			minorDiff := diffMinorVersions(v1, v2)

			require.Equal(t, c.minorDiff, minorDiff)
		})
	}
}

// TestParseRootPersistentFlags guards against a --help / -h token causing pflag to drop the root
// flags that follow it (e.g. --otel-traces), which left tracing silently disabled for `pulumi do`.
func TestParseRootPersistentFlags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		args      []string
		wantOtel  string
		wantColor string
	}{
		{
			name:     "otel flag, no help",
			args:     []string{"random:index:RandomString", "create", "--otel-traces", "grpc://localhost:4317"},
			wantOtel: "grpc://localhost:4317",
		},
		{
			name:     "help before otel flag",
			args:     []string{"random:index:RandomString", "create", "--help", "--otel-traces", "grpc://localhost:4317"},
			wantOtel: "grpc://localhost:4317",
		},
		{
			name:     "short help before otel flag",
			args:     []string{"random:index:RandomString", "create", "-h", "--otel-traces", "grpc://localhost:4317"},
			wantOtel: "grpc://localhost:4317",
		},
		{
			name: "help between two root flags, with unknown provider flags",
			args: []string{
				"random:index:RandomString", "create", "--length", "8",
				"--help", "--otel-traces", "file:///tmp/t.json", "--color", "never",
			},
			wantOtel:  "file:///tmp/t.json",
			wantColor: "never",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var otelTraces, color string
			root := pflag.NewFlagSet("pulumi", pflag.ContinueOnError)
			root.StringVar(&otelTraces, "otel-traces", "", "")
			root.StringVar(&color, "color", "", "")

			parseRootPersistentFlags(root, c.args)

			assert.Equal(t, c.wantOtel, otelTraces)
			assert.Equal(t, c.wantColor, color)
		})
	}
}

// Group commands (commands with subcommands but no run function) must fail with a
// non-zero exit code when given an unknown subcommand, rather than printing help
// and exiting 0. See https://github.com/spf13/cobra's execute(): non-runnable
// commands return flag.ErrHelp before args are ever validated.
//
//nolint:paralleltest // NewPulumiCmd registers env vars in a process-wide registry
func TestGroupCommandsRejectUnknownSubcommands(t *testing.T) {
	cases := []struct {
		args    []string
		wantErr string
	}{
		{args: []string{"env", "lisst"}, wantErr: `unknown command "lisst" for "pulumi env"`},
		{args: []string{"env", "bogus"}, wantErr: `unknown command "bogus" for "pulumi env"`},
		{args: []string{"esc", "bogus"}, wantErr: `unknown command "bogus" for "pulumi env"`},
		{args: []string{"env", "provider", "bogus"}, wantErr: "unknown command"},
		{args: []string{"stack", "tag", "bogus"}, wantErr: `unknown command "bogus" for "pulumi stack tag"`},
		{args: []string{"plugin", "bogus"}, wantErr: `unknown command "bogus" for "pulumi plugin"`},
	}

	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			pulumiCmd, cleanup := NewPulumiCmd()
			defer cleanup()

			var stdout, stderr bytes.Buffer
			pulumiCmd.SetOut(&stdout)
			pulumiCmd.SetErr(&stderr)
			pulumiCmd.SetArgs(c.args)

			err := pulumiCmd.Execute()
			require.Error(t, err)
			assert.ErrorContains(t, err, c.wantErr)
			assert.Equal(t, cmd.ExitCodeError, cmd.ExitCodeFor(err))
		})
	}
}

// A bare group command such as `pulumi env` should still print help, but exit
// non-zero since it did not do anything.
func TestGroupCommandsBareInvocationExitsNonZero(t *testing.T) {
	// A bare group invocation is runnable and so executes the root
	// PersistentPreRunE, which would otherwise fire a background network
	// update check. t.Setenv also rules out t.Parallel here.
	t.Setenv("PULUMI_SKIP_UPDATE_CHECK", "true")

	pulumiCmd, cleanup := NewPulumiCmd()
	defer cleanup()

	var stdout, stderr bytes.Buffer
	pulumiCmd.SetOut(&stdout)
	pulumiCmd.SetErr(&stderr)
	pulumiCmd.SetArgs([]string{"env"})

	err := pulumiCmd.Execute()
	require.Error(t, err)
	assert.True(t, result.IsBail(err), "expected a bail error so no message is printed after the help text")
	assert.Equal(t, cmd.ExitCodeError, cmd.ExitCodeFor(err))
	assert.Contains(t, stdout.String(), "Usage:", "help text should still be printed")
}

// Requesting help explicitly must keep exiting 0.
//
//nolint:paralleltest // NewPulumiCmd registers env vars in a process-wide registry
func TestGroupCommandsHelpFlagSucceeds(t *testing.T) {
	pulumiCmd, cleanup := NewPulumiCmd()
	defer cleanup()

	var stdout, stderr bytes.Buffer
	pulumiCmd.SetOut(&stdout)
	pulumiCmd.SetErr(&stderr)
	pulumiCmd.SetArgs([]string{"env", "--help"})

	err := pulumiCmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Usage:")
}
