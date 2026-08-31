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

package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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

	require.Contains(t, uncached.Diag.Message, "A new version of Pulumi is available")
	require.Contains(t, uncached.Diag.Message, "upgrade from version '1.0.0' to '1.2.3'")

	require.Contains(t, expired.Diag.Message, "A new version of Pulumi is available")
	require.Contains(t, expired.Diag.Message, "upgrade from version '1.0.0' to '1.2.3'")
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

	require.Contains(t, uncached.Diag.Message, "A new version of Pulumi is available")
	require.Contains(t, uncached.Diag.Message, "upgrade from version '1.0.0-11-g4ff08363' to '1.0.0-12-gdeadbeef'")

	require.Contains(t, expired.Diag.Message, "A new version of Pulumi is available")
	require.Contains(t, expired.Diag.Message, "upgrade from version '1.0.0-11-g4ff08363' to '1.0.0-12-gdeadbeef'")
}

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

	require.Contains(t, uncached.Diag.Message, "A new version of Pulumi is available")
	require.Contains(t, uncached.Diag.Message, "upgrade from version '1.0.0' to '2.0.3'")

	require.Contains(t, expired.Diag.Message, "A new version of Pulumi is available")
	require.Contains(t, expired.Diag.Message, "upgrade from version '1.0.0' to '2.0.3'")
}

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

	require.Contains(t, uncached.Diag.Message, "You are running a very old version of Pulumi")
	require.Contains(t, uncached.Diag.Message, "upgrade from version '1.0.0' to '1.40.3'")

	require.Nil(t, cached)
	require.Nil(t, cachedAgain)

	require.Contains(t, expired.Diag.Message, "You are running a very old version of Pulumi")
	require.Contains(t, expired.Diag.Message, "upgrade from version '1.0.0' to '1.40.3'")
}

func TestStartHonorsSkipUpdateCheck(t *testing.T) {
	t.Setenv("PULUMI_SKIP_UPDATE_CHECK", "true")

	require.Nil(t, Start(t.Context(), "https://api.pulumi.com", nil))
}

func TestFinish(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())

	// A nil channel means the check never started.
	require.Nil(t, Finish(nil))

	// A check that has not completed yet is not waited for.
	pending := make(chan *CheckResult, 1)
	require.Nil(t, Finish(pending))

	// A completed check is returned and its version information cached.
	result := &CheckResult{
		Diag: diag.RawMessage("", "a new version is available"),
		versionInfo: cachedVersionInfo{
			LatestVersion:        "1.2.3",
			OldestWithoutWarning: "1.2.0",
		},
	}
	done := make(chan *CheckResult, 1)
	done <- result
	close(done)
	require.Equal(t, result, Finish(done))

	cached, err := readVersionInfo()
	require.NoError(t, err)
	require.Equal(t, result.versionInfo, cached)

	// A completed check that found nothing yields nothing.
	empty := make(chan *CheckResult, 1)
	empty <- nil
	close(empty)
	require.Nil(t, Finish(empty))
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
