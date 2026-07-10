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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/sdk/nodejs/cmd/pulumi-language-nodejs/v3/noderesolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveRuntimeBinFallsBackToManagedNode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH fixture is unix-only")
	}
	bin := t.TempDir()
	fakeNode := filepath.Join(bin, "node")
	require.NoError(t, os.WriteFile(fakeNode, []byte("#!/bin/sh\n"), 0o755))
	t.Setenv("PATH", t.TempDir()) // empty PATH

	host := &nodeLanguageHost{
		runtime: "nodejs",
		resolveNode: func(ctx context.Context, out io.Writer) (noderesolver.Result, error) {
			return noderesolver.Result{Node: fakeNode, BinDir: bin, Managed: true}, nil
		},
	}
	got, err := host.resolveRuntimeBin(context.Background(), io.Discard)
	require.NoError(t, err)
	assert.Equal(t, fakeNode, got)
	// The managed bin dir is now on the process PATH for package-manager children.
	resolved, err := exec.LookPath("node")
	require.NoError(t, err)
	assert.Equal(t, fakeNode, resolved)
}

func TestResolveRuntimeBinAmbientDoesNotMutatePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH fixture is unix-only")
	}
	ambient := filepath.Join(t.TempDir(), "node")
	t.Setenv("PATH", t.TempDir()) // ambient bin dir intentionally not on PATH
	host := &nodeLanguageHost{
		runtime: "nodejs",
		resolveNode: func(ctx context.Context, out io.Writer) (noderesolver.Result, error) {
			return noderesolver.Result{Node: ambient, BinDir: filepath.Dir(ambient), Managed: false}, nil
		},
	}
	got, err := host.resolveRuntimeBin(context.Background(), io.Discard)
	require.NoError(t, err)
	assert.Equal(t, ambient, got)
	_, err = exec.LookPath("node")
	require.Error(t, err, "an ambient resolve must not prepend the bin dir to PATH")
}

func TestResolveRuntimeBinBunNoFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH fixture is unix-only")
	}
	t.Setenv("PATH", t.TempDir())
	host := &nodeLanguageHost{
		runtime: "bun",
		resolveNode: func(ctx context.Context, out io.Writer) (noderesolver.Result, error) {
			t.Fatal("resolver must not be called for bun")
			return noderesolver.Result{}, nil
		},
	}
	_, err := host.resolveRuntimeBin(context.Background(), io.Discard)
	require.Error(t, err)
}

func TestResolveRuntimeBinNilResolverFallsBackToLookPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH fixture is unix-only")
	}
	bin := t.TempDir()
	ambient := filepath.Join(bin, "node")
	require.NoError(t, os.WriteFile(ambient, []byte("#!/bin/sh\n"), 0o755))
	t.Setenv("PATH", bin)
	host := &nodeLanguageHost{runtime: "nodejs"}
	got, err := host.resolveRuntimeBin(context.Background(), io.Discard)
	require.NoError(t, err)
	assert.Equal(t, ambient, got)
}
