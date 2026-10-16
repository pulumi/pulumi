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

package plugin

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloudCredentialEnvArgument(t *testing.T) {
	t.Setenv("PULUMI_BACKEND_URL", "https://ambient.example.com")
	t.Setenv("PULUMI_ACCESS_TOKEN", "ambient-token")
	constructors := []struct {
		name       string
		newContext func(map[string]string) (*Context, error)
	}{
		{"NewContext", func(credentials map[string]string) (*Context, error) {
			return NewContext(t.Context(), nil, nil, &MockHost{}, nil, "", nil, false, nil, credentials)
		}},
		{"NewContextWithRoot", func(credentials map[string]string) (*Context, error) {
			return NewContextWithRoot(t.Context(), nil, nil, &MockHost{}, "", "", nil, false, nil, nil, nil, nil, credentials)
		}},
		{"NewContextWithHost", func(credentials map[string]string) (*Context, error) {
			return NewContextWithHost(t.Context(), nil, nil, &MockHost{}, "", "", nil, credentials)
		}},
	}
	//nolint:paralleltest // Subtests share process credentials configured by the parent.
	for _, constructor := range constructors {
		t.Run(constructor.name, func(t *testing.T) {
			for _, test := range []struct {
				name        string
				credentials map[string]string
			}{
				{"nil", nil},
				{"token", map[string]string{"PULUMI_API": "https://helper.example.com", "PULUMI_ACCESS_TOKEN": "helper-token"}},
				{"header only", map[string]string{"PULUMI_API": "https://helper.example.com", "PULUMI_ACCESS_TOKEN": ""}},
				{"empty", map[string]string{}},
			} {
				t.Run(test.name, func(t *testing.T) {
					ctx, err := constructor.newContext(test.credentials)
					require.NoError(t, err)
					defer ctx.Close()
					expected := test.credentials
					if expected == nil && constructor.name != "NewContextWithHost" {
						expected = map[string]string{"PULUMI_API": "https://ambient.example.com", "PULUMI_ACCESS_TOKEN": "ambient-token"}
					}
					assert.Equal(t, expected, ctx.CloudCredentialEnv)
				})
			}
		})
	}
}

func TestContextRequest_race(t *testing.T) {
	t.Parallel()

	ctx, err := NewContext(
		t.Context(),
		diagtest.LogSink(t), // The diagnostics sink to use for messages.
		diagtest.LogSink(t), // The diagnostics sink to use for status messages.
		&MockHost{},         // the host that can be used to fetch providers; unused by this test.
		nil,                 // configSource
		t.TempDir(),         // the working directory to spawn all plugins in.
		nil,                 // runtimeOptions
		false,               // disableProviderPreview
		mocktracer.New().StartSpan("root"),
		nil,
	)
	require.NoError(t, err)

	// Run 10 goroutines that all call context.Request() concurrently to trigger the race detector.
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			ctx.Request()
		})
	}
	wg.Wait()
}

func TestWithoutProviderDebugging(t *testing.T) {
	t.Parallel()

	pwd := t.TempDir()
	ctx, err := NewContext(
		t.Context(),
		diagtest.LogSink(t),
		diagtest.LogSink(t),
		&MockHost{},
		nil,
		pwd,
		nil,
		false,
		mocktracer.New().StartSpan("root"),
		nil,
	)
	require.NoError(t, err)

	require.False(t, ctx.DisableProviderDebugging())
	require.Same(t, ctx, ctx.LifetimeContext())

	view := ctx.WithoutProviderDebugging()
	require.True(t, view.DisableProviderDebugging())
	require.False(t, ctx.DisableProviderDebugging(), "origin context must be unaffected")
	require.NotSame(t, ctx, view)

	require.Same(t, ctx, view.LifetimeContext())
	require.Equal(t, ctx.Pwd, view.Pwd)
	require.Equal(t, ctx.Host, view.Host)

	require.Same(t, view, view.WithoutProviderDebugging())
}

// A project's package paths are relative to the project directory. A plugin
// that starts in a subdirectory of the project must resolve them from there,
// not from its own working directory.
func TestNewContextRootsAtDetectedProject(t *testing.T) { //nolint:paralleltest // t.Chdir forbids t.Parallel
	projectDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	componentDir := filepath.Join(projectDir, "component")
	require.NoError(t, os.Mkdir(componentDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "Pulumi.yaml"),
		[]byte("name: project\nruntime: yaml\npackages:\n  component: ./component\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(componentDir, "PulumiPlugin.yaml"),
		[]byte("name: component\nruntime: yaml\n"), 0o600))
	t.Chdir(componentDir)

	ctx, err := NewContext(
		t.Context(),
		diagtest.LogSink(t),
		diagtest.LogSink(t),
		&MockHost{},
		nil,
		componentDir,
		nil,
		false,
		mocktracer.New().StartSpan("root"),
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, projectDir, ctx.Root)
	require.Equal(t, componentDir, ctx.Pwd)
}
