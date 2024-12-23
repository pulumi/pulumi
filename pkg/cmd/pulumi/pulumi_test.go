// Copyright 2016-2020, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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
func TestCheckForUpdate(t *testing.T) {
	realVersion := version.Version
	t.Cleanup(func() {
		version.Version = realVersion
	})
	version.Version = "v1.0.0"

	// Cached version information is stored in PULUMI_HOME.
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	// If the cached version is missing or outdated,
	// the HTTP server receives a request.
	var requestCounter int // number of requests
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cli/version":
			requestCounter++
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"latestVersion": "v1.2.3",
				"oldestWithoutWarning": "v1.2.0"
			}`))
			if !assert.NoError(t, err) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PULUMI_API", srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := checkForUpdate(ctx)
	require.NotNil(t, msg)
	assert.Contains(t, msg.Message, "A new version of Pulumi is available")
	assert.Contains(t, msg.Message, "upgrade from version '1.0.0' to '1.2.3'")
	assert.Equal(t, 1, requestCounter,
		"expected exactly one request to the HTTP server")

	t.Run("cached", func(t *testing.T) {
		// Once we have cached version information,
		// we will not warn the user again until the cache expires.
		requestCounter = 0
		require.Nil(t, checkForUpdate(ctx))
		assert.Equal(t, 0, requestCounter,
			"no requests are expected to the HTTP server")
	})

	t.Run("cache expired", func(t *testing.T) {
		// Expire the cached version information
		// and verify that we query the server again.

		versionCachePath, err := workspace.GetCachedVersionFilePath()
		require.NoError(t, err)

		expiredTime := time.Now().Add(-25 * time.Hour)
		require.NoError(t,
			os.Chtimes(versionCachePath, expiredTime, expiredTime))

		requestCounter = 0
		require.NotNil(t, checkForUpdate(ctx))
		assert.Equal(t, 1, requestCounter)
	})
}

//nolint:paralleltest // changes environment variables and globals
func TestCheckForUpdate_allFail(t *testing.T) {
	// Cached version information is stored in PULUMI_HOME.
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	var requestCounter int // number of requests
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cli/version":
			requestCounter++
			http.Error(w, "great sadness", http.StatusInternalServerError)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PULUMI_API", srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := checkForUpdate(ctx)
	assert.Nil(t, msg)

	// We should not make more than one attempt to get this information.
	assert.Equal(t, 1, requestCounter)
}

//nolint:paralleltest // changes environment variables and globals
func TestCheckForUpdate_devVersion(t *testing.T) {
	realVersion := version.Version
	t.Cleanup(func() {
		version.Version = realVersion
	})
	version.Version = "v1.0.0-11-g4ff08363"
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	// Cached version information is stored in PULUMI_HOME.
	var requestCounter int // number of requests
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cli/version":
			requestCounter++
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"latestVersion": "v1.2.3",
				"oldestWithoutWarning": "v1.2.0",
				"latestDevVersion": "v1.0.0-12-gdeadbeef"
			}`))
			if !assert.NoError(t, err) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PULUMI_API", srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := checkForUpdate(ctx)
	require.NotNil(t, msg)
	assert.Contains(t, msg.Message, "A new version of Pulumi is available")
	assert.Contains(t, msg.Message, "upgrade from version '1.0.0-11-g4ff08363' to '1.0.0-12-gdeadbeef'")
	assert.Equal(t, 1, requestCounter,
		"expected exactly one request to the HTTP server")

	t.Run("cached", func(t *testing.T) {
		// Once we have cached version information,
		// we will not warn the user again until the cache expires.
		requestCounter = 0
		require.Nil(t, checkForUpdate(ctx))
		assert.Equal(t, 0, requestCounter,
			"no requests are expected to the HTTP server")
	})

	t.Run("cache expired", func(t *testing.T) {
		// Expire the cached version information
		// and verify that we query the server again.

		versionCachePath, err := workspace.GetCachedVersionFilePath()
		require.NoError(t, err)

		expiredTime := time.Now().Add(-2 * time.Hour)
		require.NoError(t,
			os.Chtimes(versionCachePath, expiredTime, expiredTime))

		requestCounter = 0
		require.NotNil(t, checkForUpdate(ctx))
		assert.Equal(t, 1, requestCounter)
	})
}
