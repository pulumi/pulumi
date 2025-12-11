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

package packageworkspace

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type Options struct {
	UseLanguageVersionTools bool
}

func New(
	host plugin.Host, stdout, stderr io.Writer, sink, statusSink diag.Sink,
	parentSPan opentracing.Span, options Options,
) Workspace {
	return Workspace{host, stdout, stderr, options, sink, statusSink, parentSPan}
}

type Workspace struct {
	host             plugin.Host
	stdout, stderr   io.Writer
	options          Options
	sink, statusSink diag.Sink
	parentSpan       opentracing.Span
}

func (Workspace) HasPlugin(spec workspace.PluginSpec) bool { return workspace.HasPlugin(spec) }
func (Workspace) HasPluginGTE(spec workspace.PluginSpec) (bool, error) {
	return workspace.HasPluginGTE(spec)
}
func (Workspace) IsExternalURL(source string) bool { return workspace.IsExternalURL(source) }

func (Workspace) GetPluginPath(ctx context.Context, spec workspace.PluginSpec) (string, error) {
	path, err := spec.DirPath()
	if err != nil {
		return "", nil
	}
	// This should be runnable, so we need to include the subdir if any.
	return filepath.Join(path, spec.SubDir()), nil
}

// Install an already downloaded plugin at a specific path.
//
// InstallPlugin should assume that all dependencies of the plugin are already
// installed.
func (w Workspace) InstallPluginAt(ctx context.Context, dirPath string, project *workspace.PluginProject) error {
	lang, err := w.host.LanguageRuntime(project.Runtime.Name())
	if err != nil {
		return err
	}
	info := plugin.NewProgramInfo(dirPath, dirPath, ".", project.Runtime.Options())
	return cmdutil.InstallDependencies(lang, plugin.InstallDependenciesRequest{
		Info:                    info,
		UseLanguageVersionTools: w.options.UseLanguageVersionTools,
		IsPlugin:                true,
	}, w.stdout, w.stderr)
}

// IsExecutable returns if the file at binaryPath can be executed.
//
// If no file is found at binaryPath, then (false, os.ErrNotExist) should be
// returned.
func (Workspace) IsExecutable(ctx context.Context, binaryPath string) (bool, error) {
	info, err := os.Stat(binaryPath)
	if err != nil {
		return false, err
	}
	// Windows doesn't have executable bits to check
	if runtime.GOOS == "windows" {
		return !info.IsDir(), nil
	}
	return info.Mode()&0o111 != 0 && !info.IsDir(), nil
}

func (Workspace) LoadPluginProject(ctx context.Context, path string) (*workspace.PluginProject, error) {
	return workspace.LoadPluginProject(path)
}

// Download a plugin onto disk, returning the path the plugin was downloaded to.
func (Workspace) DownloadPlugin(ctx context.Context, plugin workspace.PluginSpec) (string, error) {
	panic("TODO")
}

func (Workspace) DetectPluginPathAt(ctx context.Context, path string) (string, error) {
	return workspace.DetectPluginPathAt(path)
}

// Link a package into a project, generating an SDK if appropriate.
//
// project and projectDir describe the where the SDK is being generated and linked into.
//
// parameters describes any parameters necessary to convert the plugin into a
// package.
//
// The plugin used to generate the SDK will always be installed already, and
// should be run from pluginDir.
func (w Workspace) LinkPackage(
	ctx context.Context,
	project *workspace.ProjectRuntimeInfo, projectDir string, packageName string,
	pluginDir string, params plugin.ParameterizeParameters,
) error {
	p, paramResp, err := w.runPackage(ctx, projectDir, pluginDir, params)
	if err != nil {
		return err
	}

	var schemaRequest plugin.GetSchemaRequest
	if paramResp != nil {
		schemaRequest.SubpackageName = paramResp.Name
		schemaRequest.SubpackageVersion = &paramResp.Version
	}
	schemaResponse, err := p.GetSchema(ctx, schemaRequest)
	if err != nil {
		return err
	}
	var schemaSpec schema.PackageSpec
	if err := json.Unmarshal(schemaResponse.Schema, &schemaSpec); err != nil {
		return err
	}

	boundSchema, err := bindSpec(schemaSpec, schema.NewPluginLoader(noCloseHost{w.host}))
	if err != nil {
		return err
	}

	// We _always_ want SupportPack turned on for `package add`, this is an option on schemas because it can change
	// things like module paths for Go and we don't want every user using gen-sdk to be affected by that. But for
	// `package add` we know that this is just a local package and it's ok for module paths and similar to be different.
	boundSchema.SupportPack = true

	// TODO: Perf optimization. Given a unique (pluginDir, params) key, we can re-use
	// a generated SDK every time that combination is requested. This will help in
	// cases where the same provider is needed by multiple other plugins.
	panic("TODO: Generate the SDK into a temp directory, if successful, then link it into the current directory.")
}

// Run a package from a directory, parameterized by params.
func (w Workspace) RunPackage(
	ctx context.Context, rootDir, pluginDir string, params plugin.ParameterizeParameters,
) (plugin.Provider, error) {
	p, _, err := w.runPackage(ctx, rootDir, pluginDir, params)
	return p, err
}

func bindSpec(spec schema.PackageSpec, loader schema.Loader) (*schema.Package, error) {
	pkg, diags, err := schema.BindSpec(spec, loader, schema.ValidationOptions{
		AllowDanglingReferences: true,
	})
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, diags
	}
	return pkg, nil
}

// Run a package from a directory, parameterized by params.
func (w Workspace) runPackage(
	ctx context.Context, rootDir, pluginDir string, params plugin.ParameterizeParameters,
) (plugin.Provider, *plugin.ParameterizeResponse, error) {
	pctx := plugin.NewContextWithHost(ctx, w.sink, w.statusSink, noCloseHost{w.host}, rootDir, rootDir, w.parentSpan)
	p, err := plugin.NewProviderFromPath(w.host, pctx, pluginDir)
	if err != nil {
		return nil, nil, err
	}
	p = providerWithEmbeddedContext{p, pctx}
	var pluginResp *plugin.ParameterizeResponse
	if params != nil && !params.Empty() {
		resp, err := p.Parameterize(ctx, plugin.ParameterizeRequest{
			Parameters: params,
		})
		if err != nil {
			return nil, nil, errors.Join(err, p.Close())
		}
		pluginResp = &resp
	}
	return p, pluginResp, nil
}

type providerWithEmbeddedContext struct {
	plugin.Provider
	pctx *plugin.Context
}

func (p providerWithEmbeddedContext) Close() error {
	return errors.Join(
		p.pctx.Host.CloseProvider(p.Provider),
		p.pctx.Close(),
	)
}

type noCloseHost struct {
	plugin.Host
}

func (noCloseHost) Close() error { return nil }
