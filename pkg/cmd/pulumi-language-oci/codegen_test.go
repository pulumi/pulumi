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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// fakeDelegate is an in-process LanguageRuntime standing in for a real
// pulumi-language-<lang>. It records the GeneratePackage/Link calls the OCI host
// forwards (and, for GeneratePackage, writes a file into the requested directory) so a
// test can prove the OCI host delegated the call rather than no-op'd it.
type fakeDelegate struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	mu      sync.Mutex
	genReq  *pulumirpc.GeneratePackageRequest
	linkReq *pulumirpc.LinkRequest
}

func (f *fakeDelegate) Handshake(
	context.Context, *pulumirpc.LanguageHandshakeRequest,
) (*pulumirpc.LanguageHandshakeResponse, error) {
	// A non-nil response is required for the host to accept an attached language plugin.
	return &pulumirpc.LanguageHandshakeResponse{}, nil
}

func (f *fakeDelegate) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: "0.0.0"}, nil
}

func (f *fakeDelegate) GeneratePackage(
	_ context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	f.mu.Lock()
	f.genReq = req
	f.mu.Unlock()
	// Emit a real artifact where the host said to, so the test asserts on observable
	// output, not just that a method was entered.
	if err := os.MkdirAll(req.Directory, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(req.Directory, "generated.txt"), []byte(req.Schema), 0o644); err != nil {
		return nil, err
	}
	return &pulumirpc.GeneratePackageResponse{}, nil
}

func (f *fakeDelegate) Link(_ context.Context, req *pulumirpc.LinkRequest) (*pulumirpc.LinkResponse, error) {
	f.mu.Lock()
	f.linkReq = req
	f.mu.Unlock()
	name := ""
	if len(req.Packages) > 0 {
		name = req.Packages[0].GetPackage().GetName()
	}
	return &pulumirpc.LinkResponse{ImportInstructions: "delegate-linked: " + name}, nil
}

// serveFakeDelegate starts the fake delegate, points the host's plugin loader at it via
// PULUMI_DEBUG_LANGUAGES (attach instead of spawn — fully in-process), and returns the
// fake plus the language name to put in runtime.options.language.
func serveFakeDelegate(t *testing.T) (*fakeDelegate, string) {
	t.Helper()
	fake := &fakeDelegate{}
	cancel := make(chan bool)
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, fake)
			return nil
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { close(cancel); <-handle.Done })

	const lang = "delegatetest"
	t.Setenv("PULUMI_DEBUG_LANGUAGES", fmt.Sprintf("%s:%d", lang, handle.Port))
	return fake, lang
}

// ociProjectDir writes a minimal `runtime: oci` project declaring the given dev language
// and chdirs into it (so the host reads runtime.options.language the way it would in a
// real launch, where cwd == the project root).
func ociProjectDir(t *testing.T, lang string) string {
	t.Helper()
	dir := t.TempDir()
	yaml := fmt.Sprintf("name: oci-deleg-test\nruntime:\n  name: oci\n  options:\n    language: %s\n", lang)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte(yaml), 0o644))
	t.Chdir(dir)
	return dir
}

// The OCI host owns the runtime RPCs but delegates SDK generation to the project's dev
// language host. This proves GeneratePackage is forwarded to
// pulumi-language-<runtime.options.language> with the request intact, and its output
// lands where the caller asked — i.e. the OCI host delegated rather than silently
// no-op'd (the failure mode if it inherited UnimplementedLanguageRuntimeServer).
func TestGeneratePackageDelegates(t *testing.T) {
	fake, lang := serveFakeDelegate(t)
	dir := ociProjectDir(t, lang)

	outDir := filepath.Join(dir, "sdks", "probe")
	schema := `{"name":"probe","version":"1.0.0"}`
	h := &ociHost{}
	_, err := h.GeneratePackage(context.Background(), &pulumirpc.GeneratePackageRequest{
		Directory:    outDir,
		Schema:       schema,
		LoaderTarget: "127.0.0.1:1", // unused by the fake; a real host would dial it
		Local:        true,
	})
	require.NoError(t, err)

	// The delegate saw the request verbatim...
	require.NotNil(t, fake.genReq, "delegate GeneratePackage was never called")
	assert.Equal(t, outDir, fake.genReq.Directory)
	assert.Equal(t, schema, fake.genReq.Schema)
	assert.True(t, fake.genReq.Local)

	// ...and its output landed in the requested directory.
	got, err := os.ReadFile(filepath.Join(outDir, "generated.txt"))
	require.NoError(t, err)
	assert.Equal(t, schema, string(got))
}

// Link is the other delegated codegen RPC. This also exercises the wire→interface
// reconstruction (programInfoFromProto / linkDepsFromProto): the dependency the host
// receives round-trips losslessly through to the delegate, and the delegate's
// instructions return to the caller.
func TestLinkDelegates(t *testing.T) {
	fake, lang := serveFakeDelegate(t)
	dir := ociProjectDir(t, lang)

	resp, err := (&ociHost{}).Link(context.Background(), &pulumirpc.LinkRequest{
		Info: &pulumirpc.ProgramInfo{
			RootDirectory:    dir,
			ProgramDirectory: dir,
			EntryPoint:       ".",
		},
		LoaderTarget: "127.0.0.1:1",
		Packages: []*pulumirpc.LinkRequest_LinkDependency{{
			Path: "sdks/probe",
			Package: &pulumirpc.PackageDependency{
				Name:    "probe",
				Version: "1.0.0",
				Kind:    "resource",
			},
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, "delegate-linked: probe", resp.ImportInstructions)

	require.NotNil(t, fake.linkReq, "delegate Link was never called")
	require.Len(t, fake.linkReq.Packages, 1)
	dep := fake.linkReq.Packages[0]
	assert.Equal(t, "sdks/probe", dep.Path)
	assert.Equal(t, "probe", dep.Package.Name)
	assert.Equal(t, "1.0.0", dep.Package.Version)
	assert.Equal(t, "resource", dep.Package.Kind)
}

// A project that declares runtime: oci but no options.language can't have its SDK
// generated — the host has nothing to delegate to. Fail with an actionable message
// rather than spawning a bogus `pulumi-language-` plugin.
func TestDelegateLanguageMissing(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"),
		[]byte("name: oci-deleg-test\nruntime: oci\n"), 0o644))
	t.Chdir(dir)

	_, err := delegateLanguage()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "runtime.options.language")
}
