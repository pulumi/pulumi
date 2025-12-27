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
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var _ packageinstallation.Workspace = &recordingWorkspace{}

func (w *recordingWorkspace) save(t *testing.T) {
	t.Helper()

	if t.Failed() {
		t.Log("STEPS:")
		for _, s := range w.steps {
			t.Log(s)
		}
		return
	}

	var b bytes.Buffer
	for _, s := range w.steps {
		// Replace \\ with / to account for [filepath]'s windows specific features.
		// Note: formatValue uses %q which escapes backslashes, so Windows paths have \\ that need to become /
		s = strings.ReplaceAll(s, "\\\\", "/")
		// Strip .exe extensions to keep golden files platform-neutral
		s = strings.ReplaceAll(s, ".exe", "")
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

type recordingWorkspace struct {
	w     packageinstallation.Workspace
	steps []string
}

func (w *recordingWorkspace) start(fName string, args ...any) {
	w.steps = append(w.steps, fmt.Sprintf("%s%s -> ", fName, formatArgs(args)))
}

func (w *recordingWorkspace) finish(args ...any) {
	w.steps[len(w.steps)-1] += formatArgs(args)
}

func (w *recordingWorkspace) HasPlugin(spec workspace.PluginDescriptor) bool {
	w.start("HasPlugin", spec)
	result := w.w.HasPlugin(spec)
	w.finish(result)
	return result
}

func (w *recordingWorkspace) HasPluginGTE(spec workspace.PluginDescriptor) (bool, *semver.Version, error) {
	w.start("HasPluginGTE", spec)
	ok, version, err := w.w.HasPluginGTE(spec)
	w.finish(ok, version, err)
	return ok, version, err
}

func (w *recordingWorkspace) IsExternalURL(source string) bool {
	w.start("IsExternalURL", source)
	result := w.w.IsExternalURL(source)
	w.finish(result)
	return result
}

func (w *recordingWorkspace) GetLatestVersion(
	ctx context.Context, spec workspace.PluginDescriptor,
) (*semver.Version, error) {
	w.start("GetLatestVersion", ctx, spec)
	version, err := w.w.GetLatestVersion(ctx, spec)
	w.finish(version, err)
	return version, err
}

func (w *recordingWorkspace) GetPluginPath(ctx context.Context, plugin workspace.PluginDescriptor) (string, error) {
	w.start("GetPluginPath", ctx, plugin)
	path, err := w.w.GetPluginPath(ctx, plugin)
	w.finish(path, err)
	return path, err
}

func (w *recordingWorkspace) InstallPluginAt(
	ctx context.Context, dirPath string, project *workspace.PluginProject,
) error {
	w.start("InstallPluginAt", ctx, dirPath, project)
	err := w.w.InstallPluginAt(ctx, dirPath, project)
	w.finish(err)
	return err
}

func (w *recordingWorkspace) IsExecutable(ctx context.Context, binaryPath string) (bool, error) {
	w.start("IsExecutable", ctx, binaryPath)
	result, err := w.w.IsExecutable(ctx, binaryPath)
	w.finish(result, err)
	return result, err
}

func (w *recordingWorkspace) LoadPluginProject(ctx context.Context, path string) (*workspace.PluginProject, error) {
	w.start("LoadPluginProject", ctx, path)
	project, err := w.w.LoadPluginProject(ctx, path)
	w.finish(project, err)
	return project, err
}

func (w *recordingWorkspace) DownloadPlugin(
	ctx context.Context, plugin workspace.PluginDescriptor,
) (string, packageinstallation.MarkInstallationDone, error) {
	w.start("DownloadPlugin", ctx, plugin)
	path, markDone, err := w.w.DownloadPlugin(ctx, plugin)
	w.finish(path, markDone, err)
	return path, func(success bool) {
		w.start("DownloadPlugin.MarkInstallationDone", plugin, success)
		markDone(success)
		w.finish()
	}, err
}

func (w *recordingWorkspace) DetectPluginPathAt(ctx context.Context, path string) (string, error) {
	w.start("DetectPluginPathAt", ctx, path)
	pluginPath, err := w.w.DetectPluginPathAt(ctx, path)
	w.finish(pluginPath, err)
	return pluginPath, err
}

func (w *recordingWorkspace) LinkPackage(
	ctx context.Context,
	project *workspace.ProjectRuntimeInfo, projectDir string, packageName tokens.Package,
	pluginPath string, params plugin.ParameterizeParameters,
	originalSpec workspace.PackageSpec,
) error {
	w.start("LinkPackage", ctx, project, projectDir, packageName, pluginPath, params, originalSpec)
	err := w.w.LinkPackage(ctx, project, projectDir, packageName, pluginPath, params, originalSpec)
	w.finish(err)
	return err
}

func (w *recordingWorkspace) RunPackage(
	ctx context.Context,
	rootDir, pluginPath string, pkgName tokens.Package, params plugin.ParameterizeParameters,
) (plugin.Provider, error) {
	w.start("RunPackage", ctx, rootDir, pluginPath, pkgName, params)
	provider, err := w.w.RunPackage(ctx, rootDir, pluginPath, pkgName, params)
	w.finish(provider, err)
	return provider, err
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
