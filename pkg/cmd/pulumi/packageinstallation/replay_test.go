// Copyright 2025, Pulumi Corporation.
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

package packageinstallation_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Args to [packageinstallation.InstallPlugin]
type replayInstallPluginArgs struct {
	spec        workspace.PackageSpec
	baseProject workspace.BaseProject
	projectDir  string
	options     packageinstallation.Options
}

// Run [packageinstallation.InstallPlugin] against a known set of steps, with a known outcome.
//
// To change the expected outcome, run with `PULUMI_ACCEPT=1`.
func replayInstallPlugin(t *testing.T, args replayInstallPluginArgs, steps ...replayStep) {
	t.Helper()
	ws := replayWorkspace{
		t:     t,
		steps: steps,
	}
	runPlugin, err := packageinstallation.InstallPlugin(
		t.Context(), args.spec, args.baseProject, args.projectDir,
		args.options, &ws /* registry */, &ws /* workspace */)
	require.NoError(t, err)

	_, err = runPlugin(t.Context(), "/plugin/launch/dir")
	require.NoError(t, err)

	var b bytes.Buffer
	for _, s := range ws.stepsTaken {
		// We do not write line numbers here to ensure that adding or removing a
		// line causes a minimal diff for reviewers.
		b.WriteString(s)
		b.WriteRune('\n')
	}
	f := filepath.Join("testdata", t.Name(), "steps.txt")

	if cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT")) {
		require.NoError(t, os.MkdirAll(filepath.Dir(f), 0o700))

		f, err := os.Create(f)
		require.NoError(t, err)
		defer contract.IgnoreClose(f)
		_, err = io.Copy(f, &b)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(f)
	require.NoError(t, err, "Unable to read golden file, run PULUMI_ACCEPT=1 to overwrite the golden file")

	assert.Equal(t, string(expected), b.String(), "%s did not match test output", f)
}

// Args to [packageinstallation.InstallInProject]
type replayInstallInProjectArgs struct {
	project    workspace.BaseProject
	projectDir string
	options    packageinstallation.Options
	packages   map[string]workspace.PackageSpec
}

// Run [packageinstallation.InstallInProject] against a known set of steps, with a known outcome.
//
// To change the expected outcome, run with `PULUMI_ACCEPT=1`.
func replayInstallInProject(t *testing.T, args replayInstallInProjectArgs, steps ...replayStep) {
	t.Helper()
	ws := replayWorkspace{
		t:     t,
		steps: steps,
	}

	proj := args.project
	for name, spec := range args.packages {
		proj.AddPackage(name, spec)
	}

	err := packageinstallation.InstallInProject(
		t.Context(), proj, args.projectDir,
		args.options, &ws /* registry */, &ws /* workspace */)
	require.NoError(t, err)

	var b bytes.Buffer
	for _, s := range ws.stepsTaken {
		// We do not write line numbers here to ensure that adding or removing a
		// line causes a minimal diff for reviewers.
		b.WriteString(s)
		b.WriteRune('\n')
	}
	f := filepath.Join("testdata", t.Name(), "steps.txt")

	if cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT")) {
		require.NoError(t, os.MkdirAll(filepath.Dir(f), 0o700))

		f, err := os.Create(f)
		require.NoError(t, err)
		defer contract.IgnoreClose(f)
		_, err = io.Copy(f, &b)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(f)
	require.NoError(t, err, "Unable to read golden file, run PULUMI_ACCEPT=1 to overwrite the golden file")

	assert.Equal(t, string(expected), b.String(), "%s did not match test output", f)
}

type replayWorkspace struct {
	steps []replayStep

	t          *testing.T
	stepsTaken []string
}

type replayStep interface {
	isReplayStep()
}

func (ws *replayWorkspace) pop(name string, args []any) (replayStep, int) {
	require.NotEmpty(ws.t, ws.steps, "attempted to run %s with no steps remaining", name)
	step := ws.steps[0]
	ws.steps = ws.steps[1:]
	ws.stepsTaken = append(ws.stepsTaken, fmt.Sprintf("%s%s -> ", name, formatArgs(args)))
	return step, len(ws.stepsTaken) - 1
}

func (ws *replayWorkspace) record(step int, result []any) {
	ws.stepsTaken[step] += formatArgs(result)
}

func formatArgs(args []any) string {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		filtered = append(filtered, formatValue(reflect.ValueOf(arg)))
	}
	return fmt.Sprintf("{%s}", strings.Join(filtered, ", "))
}

func formatValue(v reflect.Value) string {
	// Special case: context.Context - just show "ctx"
	if !v.IsValid() {
		return "nil"
	}
	if v.Type().Implements(reflect.TypeFor[context.Context]()) {
		return "ctx"
	}

	// Handle pointers
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return "nil"
		}
		// For pointers, show the type and recurse
		elem := v.Elem()
		if elem.Kind() == reflect.Struct {
			return formatStruct(elem, v.Type().Elem().Name())
		}
		return formatValue(elem)
	}

	// Handle basic types
	//
	//nolint:exhaustive // We have a default
	switch v.Kind() {
	case reflect.String:
		return fmt.Sprintf("%q", v)
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%g", v.Float())
	case reflect.Struct:
		return formatStruct(v, v.Type().Name())
	case reflect.Map:
		if v.IsNil() {
			return "nil"
		}

		// Collect all key-value pairs
		type kvPair struct {
			key   string
			value string
		}
		pairs := make([]kvPair, 0, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			pairs = append(pairs, kvPair{
				key:   formatValue(iter.Key()),
				value: formatValue(iter.Value()),
			})
		}

		// Sort by key for deterministic output
		sort.Slice(pairs, func(i, j int) bool {
			return pairs[i].key < pairs[j].key
		})

		// Format sorted pairs
		parts := make([]string, len(pairs))
		for i, pair := range pairs {
			parts[i] = fmt.Sprintf("%s:%s", pair.key, pair.value)
		}
		return fmt.Sprintf("map[%d]{%s}", v.Len(), strings.Join(parts, ", "))
	case reflect.Slice:
		if v.IsNil() {
			return "nil"
		}

		parts := make([]string, 0, v.Len())
		for i := range v.Len() {
			parts = append(parts, formatValue(v.Index(i)))
		}
		return fmt.Sprintf("[%d]{%s}", v.Len(), strings.Join(parts, ", "))
	case reflect.Interface:
		if v.IsNil() {
			return "nil"
		}
		return formatValue(v.Elem())
	case reflect.Func:
		// Functions (especially iterators) have non-deterministic addresses
		// Just show the type
		return fmt.Sprintf("<%s>", v.Type())
	default:
		// Fallback for unknown types
		return fmt.Sprintf("%v", v)
	}
}

func formatStruct(rv reflect.Value, typeName string) string {
	parts := make([]string, 0, rv.NumField())
	rt := rv.Type()

	for i := range rv.NumField() {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		// Skip zero values
		if fieldValue.IsZero() {
			continue
		}

		// Format the field
		fieldStr := formatValue(fieldValue)
		parts = append(parts, fmt.Sprintf("%s:%s", field.Name, fieldStr))
	}

	return fmt.Sprintf("%s{%s}", typeName, strings.Join(parts, ", "))
}

func (ws *replayWorkspace) HasPlugin(spec workspace.PluginDescriptor) bool {
	next, idx := ws.pop("HasPlugin", []any{spec})
	step, ok := next.(HasPlugin)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result := step(spec)
	ws.record(idx, []any{result})
	return result
}

type HasPlugin func(spec workspace.PluginDescriptor) bool

func (HasPlugin) isReplayStep() {}

func (ws *replayWorkspace) HasPluginGTE(spec workspace.PluginDescriptor) (bool, *semver.Version, error) {
	next, idx := ws.pop("HasPluginGTE", []any{spec})
	step, ok := next.(HasPluginGTE)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result1, result2, result3 := step(spec)
	ws.record(idx, []any{result1, result2, result3})
	return result1, result2, result3
}

type HasPluginGTE func(spec workspace.PluginDescriptor) (bool, *semver.Version, error)

func (HasPluginGTE) isReplayStep() {}

func (ws *replayWorkspace) GetPluginPath(ctx context.Context, spec workspace.PluginDescriptor) (string, error) {
	next, idx := ws.pop("GetPluginPath", []any{spec})
	step, ok := next.(GetPluginPath)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result1, result2 := step(ctx, spec)
	ws.record(idx, []any{result1, result2})
	return result1, result2
}

type GetPluginPath func(ctx context.Context, spec workspace.PluginDescriptor) (string, error)

func (GetPluginPath) isReplayStep() {}

func (ws *replayWorkspace) GetLatestVersion(
	ctx context.Context, spec workspace.PluginDescriptor,
) (*semver.Version, error) {
	next, idx := ws.pop("GetLatestVersion", []any{spec})
	step, ok := next.(GetLatestVersion)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result1, result2 := step(ctx, spec)
	ws.record(idx, []any{result1, result2})
	return result1, result2
}

type GetLatestVersion func(ctx context.Context, spec workspace.PluginDescriptor) (*semver.Version, error)

func (GetLatestVersion) isReplayStep() {}

func (ws *replayWorkspace) IsExternalURL(source string) bool {
	next, idx := ws.pop("IsExternalURL", []any{source})
	step, ok := next.(IsExternalURL)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result := step(source)
	ws.record(idx, []any{result})
	return result
}

type IsExternalURL func(source string) bool

func (IsExternalURL) isReplayStep() {}

func (ws *replayWorkspace) InstallPluginAt(
	ctx context.Context, dirPath string, project *workspace.PluginProject,
) error {
	next, idx := ws.pop("InstallPluginAt", []any{dirPath, project})
	step, ok := next.(InstallPluginAt)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result := step(ctx, dirPath, project)
	ws.record(idx, []any{result})
	return result
}

type InstallPluginAt func(ctx context.Context, dirPath string, project *workspace.PluginProject) error

func (InstallPluginAt) isReplayStep() {}

func (ws *replayWorkspace) IsExecutable(ctx context.Context, binaryPath string) (bool, error) {
	next, idx := ws.pop("IsExecutable", []any{binaryPath})
	step, ok := next.(IsExecutable)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result1, result2 := step(ctx, binaryPath)
	ws.record(idx, []any{result1, result2})
	return result1, result2
}

type IsExecutable func(ctx context.Context, binaryPath string) (bool, error)

func (IsExecutable) isReplayStep() {}

func (ws *replayWorkspace) LoadPluginProject(ctx context.Context, path string) (*workspace.PluginProject, error) {
	next, idx := ws.pop("LoadPluginProject", []any{path})
	step, ok := next.(LoadPluginProject)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result1, result2 := step(ctx, path)
	ws.record(idx, []any{result1, result2})
	return result1, result2
}

type LoadPluginProject func(ctx context.Context, path string) (*workspace.PluginProject, error)

func (LoadPluginProject) isReplayStep() {}

func (ws *replayWorkspace) DownloadPlugin(
	ctx context.Context, plugin workspace.PluginDescriptor,
) (string, packageinstallation.MarkInstallationDone, error) {
	next, idx := ws.pop("DownloadPlugin", []any{plugin})
	step, ok := next.(DownloadPlugin)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result1, result2, result3 := step(ctx, plugin)
	ws.record(idx, []any{result1, result2, result3})

	wrappedCleanup := func(success bool) {
		ws.stepsTaken = append(ws.stepsTaken, "MarkInstallationDone"+formatArgs([]any{plugin, success}))
		result2(success)
	}

	return result1, wrappedCleanup, result3
}

type DownloadPlugin func(
	ctx context.Context, plugin workspace.PluginDescriptor,
) (string, packageinstallation.MarkInstallationDone, error)

func (DownloadPlugin) isReplayStep() {}

func (ws *replayWorkspace) DetectPluginPathAt(ctx context.Context, path string) (string, error) {
	next, idx := ws.pop("DetectPluginPathAt", []any{path})
	step, ok := next.(DetectPluginPathAt)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result1, result2 := step(ctx, path)
	ws.record(idx, []any{result1, result2})
	return result1, result2
}

type DetectPluginPathAt func(ctx context.Context, path string) (string, error)

func (DetectPluginPathAt) isReplayStep() {}

func (ws *replayWorkspace) LinkPackage(
	ctx context.Context,
	project *workspace.ProjectRuntimeInfo, projectDir string,
	packageName string, pluginDir string, params plugin.ParameterizeParameters,
	originalSpec workspace.PackageSpec,
) error {
	next, idx := ws.pop("LinkPackage", []any{project, projectDir, packageName, pluginDir, params, originalSpec})
	step, ok := next.(LinkPackage)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result := step(ctx, project, projectDir, packageName, pluginDir, params, originalSpec)
	ws.record(idx, []any{result})
	return result
}

type LinkPackage func(
	ctx context.Context,
	project *workspace.ProjectRuntimeInfo, projectDir string, packageName string,
	pluginDir string, params plugin.ParameterizeParameters,
	originalSpec workspace.PackageSpec,
) error

func (LinkPackage) isReplayStep() {}

func (ws *replayWorkspace) RunPackage(
	ctx context.Context, rootDir, pluginDir string, params plugin.ParameterizeParameters,
) (plugin.Provider, error) {
	next, idx := ws.pop("RunPackage", []any{rootDir, pluginDir, params})
	step, ok := next.(RunPackage)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result1, result2 := step(ctx, rootDir, pluginDir, params)
	ws.record(idx, []any{result1, result2})
	return result1, result2
}

type RunPackage func(
	ctx context.Context, rootDir, pluginDir string, params plugin.ParameterizeParameters,
) (plugin.Provider, error)

func (RunPackage) isReplayStep() {}

func (ws *replayWorkspace) GetPackage(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.PackageMetadata, error) {
	next, idx := ws.pop("GetPackage", []any{source, publisher, name, version})
	step, ok := next.(GetPackage)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result1, result2 := step(ctx, source, publisher, name, version)
	ws.record(idx, []any{result1, result2})
	return result1, result2
}

type GetPackage func(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.PackageMetadata, error)

func (GetPackage) isReplayStep() {}

func (ws *replayWorkspace) ListPackages(
	ctx context.Context, name *string,
) iter.Seq2[apitype.PackageMetadata, error] {
	next, idx := ws.pop("ListPackages", []any{name})
	step, ok := next.(ListPackages)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result := step(ctx, name)
	ws.record(idx, []any{result})
	return result
}

type ListPackages func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error]

func (ListPackages) isReplayStep() {}

func (ws *replayWorkspace) GetTemplate(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.TemplateMetadata, error) {
	next, idx := ws.pop("GetTemplate", []any{source, publisher, name, version})
	step, ok := next.(GetTemplate)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result1, result2 := step(ctx, source, publisher, name, version)
	ws.record(idx, []any{result1, result2})
	return result1, result2
}

type GetTemplate func(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.TemplateMetadata, error)

func (GetTemplate) isReplayStep() {}

func (ws *replayWorkspace) ListTemplates(
	ctx context.Context, name *string,
) iter.Seq2[apitype.TemplateMetadata, error] {
	next, idx := ws.pop("ListTemplates", []any{name})
	step, ok := next.(ListTemplates)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result := step(ctx, name)
	ws.record(idx, []any{result})
	return result
}

type ListTemplates func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error]

func (ListTemplates) isReplayStep() {}

func (ws *replayWorkspace) DownloadTemplate(ctx context.Context, downloadURL string) (io.ReadCloser, error) {
	next, idx := ws.pop("DownloadTemplate", []any{downloadURL})
	step, ok := next.(DownloadTemplate)
	require.True(ws.t, ok, "%d: Expected step %T but found %T", idx, step, next)
	result1, result2 := step(ctx, downloadURL)
	ws.record(idx, []any{result1, result2})
	return result1, result2
}

type DownloadTemplate func(ctx context.Context, downloadURL string) (io.ReadCloser, error)

func (DownloadTemplate) isReplayStep() {}
