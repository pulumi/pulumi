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

// buildBinaryPublishPack lays out a pack with a source manifest plus pre-built analyzer
// binaries in the conventional bin/ directory: the host platform's binary (built from
// the analyzer-binary fixture) and — since validateBinaryMatrix requires linux-amd64
// regardless of host — a linux-amd64 placeholder (conformance only boots the host
// binary). It returns the pack dir, the source project, and the discovered platforms.
func buildBinaryPublishPack(t *testing.T) (string, *workspace.PolicyPackProject, []string) {
	t.Helper()
	packDir := t.TempDir()

	proj := &workspace.PolicyPackProject{Runtime: workspace.NewProjectRuntimeInfo("python", nil)}
	require.NoError(t, proj.Save(filepath.Join(packDir, "PulumiPolicy.yaml")))

	hostPlatform := workspace.CurrentPlatform()
	hostBin := "pulumi-analyzer-mypack-" + hostPlatform
	if goruntime.GOOS == "windows" {
		hostBin += ".exe"
	}
	buildAnalyzerBinaryFixture(t, filepath.Join(packDir, "bin", hostBin))

	platforms := []string{hostPlatform}
	if hostPlatform != platformLinuxAmd64 {
		require.NoError(t, os.WriteFile( //nolint:gosec // deliberately executable
			filepath.Join(packDir, "bin", "pulumi-analyzer-mypack-linux-amd64"), []byte("placeholder"), 0o755))
		platforms = append(platforms, platformLinuxAmd64)
	}
	slices.Sort(platforms)
	return packDir, proj, platforms
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
// through the source-only path or the per-platform binary path.
type publishMockService struct {
	server *httptest.Server

	createReq      apitype.CreatePolicyPackRequest
	uploads        map[string][]byte
	completeCalled bool
}

// newPublishMockService starts a mock service that accepts a publish of the named pack
// at version 0.0.1. withPlatforms, if non-empty, makes the create response return an
// upload location for each named platform (driving the binary-publish path); otherwise
// it responds as a service that only understands the single-archive publish.
func newPublishMockService(t *testing.T, packName string, withPlatforms []string) *publishMockService {
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
	mux.HandleFunc("/api/orgs/acme/policypacks/"+packName+"/versions/0.0.1/complete",
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

func TestPublishWithDeclaredBinariesUploadsBinaries(t *testing.T) {
	t.Parallel()

	packDir, proj, platforms := buildBinaryPublishPack(t)
	hostPlatform := workspace.CurrentPlatform()
	mock := newPublishMockService(t, "binary-test-pack", platforms)

	pack := newPublishTestPack(client.NewClient(mock.server.URL, "test-token", true, nil))
	op := backend.PublishOperation{
		PlugCtx:    newPublishTestPlugCtx(t, packDir),
		PolicyPack: proj,
	}

	err := pack.Publish(t.Context(), op)
	require.NoError(t, err)

	assert.Equal(t, platforms, mock.createReq.Platforms)
	assert.Equal(t, "python", mock.createReq.Runtime)
	assert.True(t, mock.completeCalled)
	require.Contains(t, mock.uploads, "source")
	require.Contains(t, mock.uploads, hostPlatform)
	// The per-platform artifact is a bare exe — no manifest.
	entries := tarEntries(t, mock.uploads[hostPlatform])
	assert.Contains(t, entries, "package/pulumi-analyzer-binary-test-pack")
	assert.NotContains(t, entries, "package/PulumiPolicy.yaml")
}

// TestPublishWithoutBinariesTakesSingleArchivePath publishes a pack whose manifest
// declares no binaries: the analyzer boots through the (fake) language runtime and
// only the source archive is uploaded.
func TestPublishWithoutBinariesTakesSingleArchivePath(t *testing.T) {
	// Build the fake "test" language runtime from the pkg/host fixture and put it on
	// PATH so the analyzer boots through the language-plugin path.
	langDir := t.TempDir()
	langBin := "pulumi-language-test"
	if goruntime.GOOS == "windows" {
		langBin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", filepath.Join(langDir, langBin), "../../host/testdata/analyzer-language")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	t.Setenv("PATH", langDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	packDir := t.TempDir()
	proj := &workspace.PolicyPackProject{Runtime: workspace.NewProjectRuntimeInfo("test", nil)}
	require.NoError(t, proj.Save(filepath.Join(packDir, "PulumiPolicy.yaml")))

	// The fake language runtime's analyzer reports name "language-test-pack",
	// version "0.0.1".
	mock := newPublishMockService(t, "language-test-pack", nil)

	pack := newPublishTestPack(client.NewClient(mock.server.URL, "test-token", true, nil))
	op := backend.PublishOperation{
		PlugCtx:    newPublishTestPlugCtx(t, packDir),
		PolicyPack: proj,
	}

	err = pack.Publish(t.Context(), op)
	require.NoError(t, err)

	assert.Equal(t, "language-test-pack", mock.createReq.Name)
	assert.Empty(t, mock.createReq.Platforms)
	assert.Equal(t, "test", mock.createReq.Runtime)
	assert.True(t, mock.completeCalled)
	require.Contains(t, mock.uploads, "source")
	assert.NotContains(t, mock.uploads, workspace.CurrentPlatform())
}
