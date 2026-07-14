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

package httpstate

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildAnalyzerBinaryFixture compiles the pkg/host analyzer-binary test fixture (which
// reports analyzer name "binary-test-pack", version "0.0.1") to dst.
func buildAnalyzerBinaryFixture(t *testing.T, dst string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0o755))
	cmd := exec.Command("go", "build", "-o", dst, "../../host/testdata/analyzer-binary")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

// buildBinaryPublishPack lays out a pack with a bin/ convention binary for the host
// platform, along with the platform-to-path map that would be passed via explicit
// --binary flags. validateBinaryMatrix also requires a linux-amd64 binary regardless
// of host platform, so one is added too — as a placeholder file, since conformance
// only boots the host platform's binary.
func buildBinaryPublishPack(t *testing.T) (string, map[string]string) {
	t.Helper()
	packDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "PulumiPolicy.yaml"),
		[]byte("runtime: nodejs\n"), 0o600))

	hostPlatform := workspace.CurrentPlatform()
	binName := "pulumi-analyzer-mypack-" + hostPlatform
	if goruntime.GOOS == "windows" {
		binName += ".exe"
	}
	hostRel := filepath.Join("bin", binName)
	buildAnalyzerBinaryFixture(t, filepath.Join(packDir, hostRel))

	binaries := map[string]string{hostPlatform: hostRel}
	if hostPlatform != workspace.PlatformLinuxAmd64 {
		linuxRel := filepath.Join("bin", "pulumi-analyzer-mypack-linux-amd64")
		require.NoError(t, os.WriteFile(filepath.Join(packDir, linuxRel), []byte("placeholder"), 0o755)) //nolint:gosec
		binaries[workspace.PlatformLinuxAmd64] = linuxRel
	}
	return packDir, binaries
}

// buildLegacyPublishPack lays out a pack that ships its analyzer as a single binary at
// the pack root using the pre-existing back-compat convention (pulumi-analyzer-<name>,
// no platform suffix — see the "policy-opa" style analyzers referenced in
// resource/plugin/analyzer_plugin.go). Since binary publishing only happens when
// op.Binaries is explicitly set, this pack takes the legacy, single-archive path even
// though the analyzer still boots as a plugin binary.
func buildLegacyPublishPack(t *testing.T) string {
	t.Helper()
	packDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "PulumiPolicy.yaml"),
		[]byte("runtime: nodejs\n"), 0o600))

	binName := "pulumi-analyzer-binary-test-pack"
	if goruntime.GOOS == "windows" {
		binName += ".exe"
	}
	buildAnalyzerBinaryFixture(t, filepath.Join(packDir, binName))
	return packDir
}

func newPublishTestPlugCtx(t *testing.T, pwd string) *plugin.Context {
	t.Helper()
	d := diagtest.LogSink(t)
	h, err := pkghost.New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, h.Close()) })
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, pwd, pwd, nil)
	require.NoError(t, err)
	return ctx
}

// publishMockService is a mock Pulumi service for the /policypacks create, upload, and
// complete steps that (*cloudPolicyPack).Publish drives, regardless of whether it goes
// through the legacy single-archive path or the per-platform binary path.
type publishMockService struct {
	server *httptest.Server

	createReq      apitype.CreatePolicyPackRequest
	uploads        map[string][]byte
	completeCalled bool
}

// newPublishMockService starts a mock service. withPlatforms, if non-empty, makes the
// create response return an upload location for each named platform (driving the
// binary-publish path); otherwise it responds as a service that only understands the
// legacy single-archive publish.
func newPublishMockService(t *testing.T, withPlatforms []string) *publishMockService {
	t.Helper()
	m := &publishMockService{uploads: map[string][]byte{}}

	mux := http.NewServeMux()
	m.server = httptest.NewServer(mux)
	t.Cleanup(m.server.Close)

	mux.HandleFunc("/api/orgs/acme/policypacks", func(rw http.ResponseWriter, req *http.Request) {
		require.NoError(t, json.NewDecoder(req.Body).Decode(&m.createReq))
		resp := apitype.CreatePolicyPackResponse{
			Version:   1,
			UploadURI: m.server.URL + "/upload/source",
		}
		if len(withPlatforms) > 0 {
			resp.PlatformUploadURIs = map[string]apitype.PolicyPackUpload{}
			for _, platform := range withPlatforms {
				resp.PlatformUploadURIs[platform] = apitype.PolicyPackUpload{
					UploadURI: m.server.URL + "/upload/" + platform,
				}
			}
		}
		require.NoError(t, json.NewEncoder(rw).Encode(resp))
	})
	mux.HandleFunc("/upload/", func(rw http.ResponseWriter, req *http.Request) {
		key := strings.TrimPrefix(req.URL.Path, "/upload/")
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		m.uploads[key] = body
	})
	mux.HandleFunc("/api/orgs/acme/policypacks/binary-test-pack/versions/0.0.1/complete",
		func(rw http.ResponseWriter, req *http.Request) {
			m.completeCalled = true
		})

	return m
}

func newPublishTestPack(cl *client.Client) *cloudPolicyPack {
	return &cloudPolicyPack{
		ref: newCloudBackendPolicyPackReference("https://console.example.com", "acme", ""),
		cl:  cl,
	}
}

func TestPublishWithExplicitBinariesUploadsBinaries(t *testing.T) {
	t.Parallel()

	packDir, binaries := buildBinaryPublishPack(t)
	platform := workspace.CurrentPlatform()
	expectedPlatforms := slices.Sorted(maps.Keys(binaries))
	mock := newPublishMockService(t, expectedPlatforms)

	pack := newPublishTestPack(client.NewClient(mock.server.URL, "test-token", true, nil))
	op := backend.PublishOperation{
		PlugCtx:    newPublishTestPlugCtx(t, packDir),
		PolicyPack: &workspace.PolicyPackProject{Runtime: workspace.NewProjectRuntimeInfo("python", nil)},
		Binaries:   binaries,
	}

	err := pack.Publish(t.Context(), op)
	require.NoError(t, err)

	assert.Equal(t, expectedPlatforms, mock.createReq.Platforms)
	assert.Equal(t, "python", mock.createReq.Runtime)
	assert.True(t, mock.completeCalled)
	require.Contains(t, mock.uploads, "source")
	require.Contains(t, mock.uploads, platform)
	entries := tarEntries(t, mock.uploads[platform])
	assert.Contains(t, entries, "package/pulumi-analyzer-binary-test-pack")
}

func TestPublishWithoutBinariesFlagTakesLegacyPathEvenWithConventionFiles(t *testing.T) {
	t.Parallel()

	// Same pack layout as the explicit-binaries test: it has a bin/ convention
	// binary, but with no --binary flags (no op.Binaries), Publish must not discover
	// it and instead take the legacy single-archive path.
	packDir, _ := buildBinaryPublishPack(t)
	mock := newPublishMockService(t, nil)

	pack := newPublishTestPack(client.NewClient(mock.server.URL, "test-token", true, nil))
	op := backend.PublishOperation{
		PlugCtx:    newPublishTestPlugCtx(t, packDir),
		PolicyPack: &workspace.PolicyPackProject{Runtime: workspace.NewProjectRuntimeInfo("python", nil)},
	}

	err := pack.Publish(t.Context(), op)
	require.NoError(t, err)

	assert.Empty(t, mock.createReq.Platforms)
	assert.True(t, mock.completeCalled)
	require.Contains(t, mock.uploads, "source")
	assert.NotContains(t, mock.uploads, workspace.CurrentPlatform())
}

func TestPublishLegacyPackTakesSingleArchivePath(t *testing.T) {
	t.Parallel()

	packDir := buildLegacyPublishPack(t)
	mock := newPublishMockService(t, nil)

	pack := newPublishTestPack(client.NewClient(mock.server.URL, "test-token", true, nil))
	op := backend.PublishOperation{
		PlugCtx:    newPublishTestPlugCtx(t, packDir),
		PolicyPack: &workspace.PolicyPackProject{Runtime: workspace.NewProjectRuntimeInfo("python", nil)},
	}

	err := pack.Publish(t.Context(), op)
	require.NoError(t, err)

	assert.Empty(t, mock.createReq.Platforms)
	assert.Equal(t, "python", mock.createReq.Runtime)
	assert.True(t, mock.completeCalled)
	require.Contains(t, mock.uploads, "source")
}
