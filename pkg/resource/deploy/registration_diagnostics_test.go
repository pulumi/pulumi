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

package deploy

import (
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceSourceContext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	file := filepath.Join(root, "main file.py")
	require.NoError(t, os.WriteFile(file, []byte("# source\nsite = Service(port=0)\n"), 0o600))
	position := (&url.URL{Scheme: "project", Path: "/main file.py", Fragment: "2"}).String()
	deployment := &Deployment{source: &evalSource{runinfo: &EvalRunInfo{
		Proj: &workspace.Project{}, ProjectRoot: root,
	}}}
	state := &pkgresource.State{SourcePosition: position}
	context := ResourceSourceContext(deployment, state)
	require.NotNil(t, context)
	assert.Equal(t, position, context.SourcePosition)
	assert.Empty(t, context.StackTrace)
	rendered := colors.Never.Colorize(context.Rendered)
	assert.Contains(t, rendered, "main file.py:2")
	assert.Contains(t, rendered, "site = Service(port=0)")
	assert.Nil(t, ResourceSourceContext(deployment, &pkgresource.State{}))
	withoutRoot := ResourceSourceContext(nil, state)
	require.NotNil(t, withoutRoot)
	assert.Equal(t, position, withoutRoot.SourcePosition)
	assert.Empty(t, withoutRoot.Rendered)
}

func TestRegistrationContextFiltersOnlyKnownOrigins(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	frames := []pkgresource.StackFrame{
		{SourcePosition: "project:///main.py#2"},
		{SourcePosition: "project:///.venv/lib/python/asyncio.py#10", Origin: "runtime"},
		{SourcePosition: "project:///bin/launcher#1", Origin: "pulumi"},
		{SourcePosition: "project:///main.py#20"},
	}
	context := formatRegistrationFrames(root, frames)
	assert.Contains(t, colors.Never.Colorize(context), "main.py:2")
	assert.Contains(t, colors.Never.Colorize(context), "main.py:20")
	assert.NotContains(t, context, ".venv")
	assert.NotContains(t, context, "launcher")
}

func TestRegistrationSourceDoesNotReadSymlinksOutsideSourceRoot(t *testing.T) {
	t.Parallel()
	root, outside := t.TempDir(), t.TempDir()
	file := filepath.Join(outside, "private.py")
	require.NoError(t, os.WriteFile(file, []byte("private source"), 0o600))
	link := filepath.Join(root, "source.py")
	if err := os.Symlink(file, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	assert.Empty(t, registrationSourceLine(root, link, 1))
}

func TestRegistrationColoursRespectColourMode(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.py"), []byte(
		"# Configure a service\nsite = Service(name=\"website\", port=0)\nsite.id\n"), 0o600))
	context := formatRegistrationFrames(root, []pkgresource.StackFrame{
		{SourcePosition: "project:///main.py#2"},
	})
	plain := colors.Never.Colorize(context)
	assert.Contains(t, plain, "main.py:2")
	assert.Contains(t, plain, "  ┌─ main.py:2\n 1│ # Configure a service\n"+
		">2│ site = Service(name=\"website\", port=0)\n 3│ site.id\n  └─")
	assert.NotContains(t, plain, "\x1b")
	assert.NotContains(t, plain, "<{%")
	assert.NotContains(t, context, "\x1b", "diagnostics carry Pulumi markup, not ANSI escapes")
	assert.Contains(t, colors.Always.Colorize(context), "\x1b[", "terminal renderer supplies the ANSI escapes")
}

func TestRegistrationSourceText(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const secret = "12345 + 67890"
	logging.AddGlobalSecretFilter([]string{secret}, "[secret]")
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.py"),
		[]byte("site = Service(port="+secret+")\n"), 0o600))
	const literal = "# literal <{%reset%}> and <{%example%}>"
	require.NoError(t, os.WriteFile(filepath.Join(root, "helper.py"), []byte(literal), 0o600))
	frames := []pkgresource.StackFrame{
		{SourcePosition: "project:///main.py#1"},
		{SourcePosition: "project:///helper.py#1"},
	}
	for range 2 {
		rendered := formatRegistrationFrames(root, frames)
		for _, mode := range []colors.Colorization{colors.Never, colors.Always} {
			text := ansi.Strip(mode.Colorize(rendered))
			assert.Contains(t, text, literal)
			assert.Contains(t, text, "site = Service(port=[secret])")
			assert.NotContains(t, text, secret)
		}
		slices.Reverse(frames) // Exercise both the highlighted excerpt and the single-line caller.
	}
}

func TestRegistrationFramesFollowCallOrderWithinEachProgram(t *testing.T) {
	t.Parallel()
	project, provider := t.TempDir(), t.TempDir()
	position := func(filename string, line string) pkgresource.StackFrame {
		return pkgresource.StackFrame{SourcePosition: (&url.URL{
			Scheme: "file", Path: filepath.ToSlash(filename), Fragment: line,
		}).String()}
	}
	providerBoundary := position(filepath.Join(provider, "main.py"), "74")
	providerBoundary.Boundary = &apitype.StackTraceBoundaryV1{Kind: apitype.StackTraceBoundaryKindConstruct}
	frames := []pkgresource.StackFrame{
		position(filepath.Join(project, "main.py"), "2"),
		position(filepath.Join(project, "main.py"), "1"),
		{SourcePosition: "project:///.venv/runtime.py#10", Origin: "runtime"},
		providerBoundary,
		position(filepath.Join(provider, "main.py"), "91"),
		position(filepath.Join(provider, ".venv", "runtime.py"), "20"),
		position(filepath.Join(provider, "main.py"), "120"),
	}
	output := colors.Never.Colorize(formatRegistrationFrames(project, frames))
	outer := strings.Index(output, "main.py:1\n")
	inner := strings.Index(output, "main.py:2")
	require.NotEqual(t, -1, outer)
	require.NotEqual(t, -1, inner)
	assert.Less(t, outer, inner)
	assert.NotContains(t, output, "main.py:74")
	assert.NotContains(t, output, "main.py:91")
	assert.NotContains(t, output, "Registered by provider")
	assert.NotContains(t, output, ".venv")
	assert.NotContains(t, output, "main.py:120")
	assert.True(t, strings.HasSuffix(frames[0].SourcePosition, "#2"), "preserve captured frame order")
}

func TestRegistrationExpandsInnermostApplicationFrame(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "__main__.py"),
		[]byte(strings.Repeat("\n", 5)+"site = build_site()\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "components.py"),
		[]byte(strings.Repeat("\n", 15)+"app = Component()\n"), 0o600))
	output := colors.Never.Colorize(formatRegistrationFrames(root,
		[]pkgresource.StackFrame{
			{SourcePosition: "project:///components.py#23"},
			{SourcePosition: "project:///components.py#16"},
			{SourcePosition: "project:///__main__.py#6"},
		}))
	assert.True(t, strings.HasPrefix(output, "      __main__.py:6     site = build_site()\n"+
		"      components.py:16  app = Component()\n   ┌─ components.py:23"), output)
	assert.NotContains(t, output, "Registration call stack")
	assert.NotContains(t, output, "Registered by provider")

	single := colors.Never.Colorize(formatRegistrationFrames(root,
		[]pkgresource.StackFrame{{SourcePosition: "project:///__main__.py#6"}}))
	assert.True(t, strings.HasPrefix(single, "  ┌─ __main__.py:6"), single)
	assert.NotContains(t, single, "Registration call stack")
}

func TestRegistrationMetadataFiltersFullCaptures(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	frames := []pkgresource.StackFrame{
		{SourcePosition: "project:///sdk.py#1", Origin: "pulumi", Boundary: &apitype.StackTraceBoundaryV1{
			Kind: apitype.StackTraceBoundaryKindProgram, CallSite: "project:///main.py#2",
		}},
		{SourcePosition: "project:///resource_constructor.py#1"},
		{SourcePosition: "project:///main.py#2", Module: "__main__"},
		{SourcePosition: "project:///bin/launcher#1", Origin: "pulumi"},
		{SourcePosition: "project:///asyncio.py#1", Origin: "runtime"},
		{SourcePosition: "project:///provider.py#1", Boundary: &apitype.StackTraceBoundaryV1{
			Kind: apitype.StackTraceBoundaryKindConstruct,
		}},
	}
	context := newSourceContext(root, "project:///provider.py#1", frames)
	assert.Equal(t, "__main__", context.StackTrace[2].Module)
	assert.Equal(t, apitype.StackTraceBoundaryKindProgram, context.StackTrace[0].Boundary.Kind)
	plain := colors.Never.Colorize(context.Rendered)
	assert.Contains(t, plain, "main.py:2")
	for _, hidden := range []string{"sdk.py", "resource_constructor.py", "launcher", "asyncio.py", "provider.py"} {
		assert.NotContains(t, plain, hidden)
	}
}

func TestRegistrationLegacyCaptureKeepsFrameSelection(t *testing.T) {
	t.Parallel()
	context := formatRegistrationFrames(t.TempDir(), []pkgresource.StackFrame{
		{SourcePosition: "project:///main.go#2", Boundary: &apitype.StackTraceBoundaryV1{
			Kind: apitype.StackTraceBoundaryKindProgram, CallSite: "project:///main.go#99",
		}},
	})
	assert.Contains(t, colors.Never.Colorize(context), "main.go:2")
}

func TestRegistrationRetainsUnknownDependencyFrames(t *testing.T) {
	t.Parallel()
	root, outside := t.TempDir(), t.TempDir()
	private := filepath.Join(outside, "library.py")
	require.NoError(t, os.WriteFile(private, []byte("do_not_read_external_source()"), 0o600))
	external := (&url.URL{Scheme: "file", Path: filepath.ToSlash(private), Fragment: "1"}).String()
	output := colors.Never.Colorize(formatRegistrationFrames(root, []pkgresource.StackFrame{
		{SourcePosition: "project:///.venv/site-packages/library/a.py#2"},
		{SourcePosition: "project:///node_modules/library/a.js#3"},
		{SourcePosition: external, Module: "library"},
		{SourcePosition: "project:///main.py#10"},
	}))
	assert.Contains(t, output, ".venv/site-packages/library/a.py:2")
	assert.Contains(t, output, "node_modules/library/a.js:3")
	assert.Contains(t, output, "library — "+filepath.ToSlash(private)+":1")
	assert.Contains(t, output, "main.py:10")
	assert.NotContains(t, output, "do_not_read_external_source")
	externalOnly := colors.Never.Colorize(formatRegistrationFrames(root,
		[]pkgresource.StackFrame{{SourcePosition: external, Module: "library"}}))
	assert.Contains(t, externalOnly, "library — ")
	assert.NotContains(t, externalOnly, "┌─")
}

func TestRegistrationRetainsRepeatedAndModuleOnlyFrames(t *testing.T) {
	t.Parallel()
	output := colors.Never.Colorize(formatRegistrationFrames(t.TempDir(), []pkgresource.StackFrame{
		{Module: "library.module"},
		{SourcePosition: "project:///recursive.py#2"},
		{SourcePosition: "project:///recursive.py#2"},
	}))
	assert.Equal(t, 2, strings.Count(output, "recursive.py:2"))
	assert.Contains(t, output, "library.module")
	assert.NotContains(t, output, "library.module:0")
}

func TestRegistrationLegacyCaptureUsesAvailableCallSite(t *testing.T) {
	t.Parallel()
	output := colors.Never.Colorize(formatRegistrationFrames(t.TempDir(), []pkgresource.StackFrame{
		{SourcePosition: "project:///wrapper.py#1", Boundary: &apitype.StackTraceBoundaryV1{
			Kind: apitype.StackTraceBoundaryKindProgram, CallSite: "project:///main.py#2",
		}},
		{SourcePosition: "project:///main.py#2"},
		{SourcePosition: "project:///.venv/library.py#3"},
	}))
	assert.NotContains(t, output, "wrapper.py")
	assert.Contains(t, output, "main.py:2")
	assert.Contains(t, output, ".venv/library.py:3")
}

func TestRegistrationProgramViewSupportsBothCaptureOrders(t *testing.T) {
	t.Parallel()
	program := []pkgresource.StackFrame{
		{SourcePosition: "project:///helper.py#2", Boundary: &apitype.StackTraceBoundaryV1{
			Kind: apitype.StackTraceBoundaryKindProgram, CallSite: "project:///helper.py#2",
		}},
		{SourcePosition: "project:///main.py#1"},
	}
	provider := []pkgresource.StackFrame{
		{SourcePosition: "project:///provider.py#3", Boundary: &apitype.StackTraceBoundaryV1{
			Kind: apitype.StackTraceBoundaryKindConstruct, ResourceName: "component",
		}},
		{SourcePosition: "project:///server.py#4", Origin: "pulumi"},
	}
	for name, frames := range map[string][]pkgresource.StackFrame{
		"program first":  append(append([]pkgresource.StackFrame{}, program...), provider...),
		"provider first": append(append([]pkgresource.StackFrame{}, provider...), program...),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			context := newSourceContext(t.TempDir(), "project:///provider.py#3", frames)
			plain := colors.Never.Colorize(context.Rendered)
			assert.Contains(t, plain, "main.py:1")
			assert.Contains(t, plain, "helper.py:2")
			assert.NotContains(t, plain, "provider.py")
			assert.NotContains(t, plain, "server.py")
			require.Len(t, context.StackTrace, len(frames))
			for i, frame := range frames {
				assert.Equal(t, frame.SourcePosition, context.StackTrace[i].SourcePosition)
			}
		})
	}
}
