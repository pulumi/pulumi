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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/util"
	"github.com/pulumi/pulumi/pkg/v3/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type Options struct {
	UseLanguageVersionTools bool
}

// New creates a new workspace.
//
// The returned workspace must be closed after use.
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

func (w Workspace) Close() error { return w.host.Close() }

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

	if !filepath.IsAbs(dirPath) {
		dirPath, err = filepath.Abs(dirPath)
		if err != nil {
			return err
		}
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
func (w Workspace) DownloadPlugin(
	ctx context.Context, pluginSpec workspace.PluginSpec,
) (string, func(done bool), error) {
	util.SetKnownPluginDownloadURL(&pluginSpec)
	util.SetKnownPluginVersion(&pluginSpec)
	if pluginSpec.Version == nil {
		var err error
		pluginSpec.Version, err = pluginSpec.GetLatestVersion(ctx)
		if err != nil {
			return "", nil, fmt.Errorf("could not find latest version for provider %s: %w", pluginSpec.Name, err)
		}
	}

	wrapper := func(stream io.ReadCloser, size int64) io.ReadCloser {
		// Log at info but to stderr so we don't pollute stdout for commands like `package get-schema`
		w.statusSink.Infoerrf(&diag.Diag{Message: "Downloading provider: %s"}, pluginSpec.Name)
		return stream
	}

	retry := func(err error, attempt int, limit int, delay time.Duration) {
		w.statusSink.Warningf(&diag.Diag{Message: "error downloading provider: %s\n" +
			"Will retry in %v [%d/%d]"}, err, delay, attempt, limit)
	}

	logging.V(1).Infof("downloading provider %s", pluginSpec.Name)
	downloadedFile, err := workspace.DownloadToFile(ctx, pluginSpec, wrapper, retry)
	if err != nil {
		return "", nil, err
	}

	logging.V(1).Infof("unpacking provider %s", pluginSpec.Name)
	cleanup, err := workspace.DownloadPluginContent(
		ctx, pluginSpec, workspace.TarPlugin(downloadedFile), true, /* reinstall */
	)
	if err != nil {
		return "", nil, err
	}
	outDir, err := pluginSpec.DirPath()
	if err != nil {
		cleanup(false)
		return "", nil, err
	}
	return filepath.Join(outDir, pluginSpec.SubDir()), cleanup, nil
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
	runtimeInfo *workspace.ProjectRuntimeInfo, projectDir string, packageName string,
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

	tmpDir, servers, err := w.genSDK(ctx, runtimeInfo.Name(), boundSchema)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(servers)

	pkgName := boundSchema.Name
	if boundSchema.Namespace != "" {
		pkgName = boundSchema.Namespace + "-" + pkgName
	}
	out := filepath.Join(projectDir, "sdks", pkgName)

	// Make sure the out directory doesn't exist anymore.
	//
	// [os.RemoveAll] handles the case  where out doesn't exist.
	if err := os.RemoveAll(out); err != nil {
		return err
	}

	// Now move the temp directory to it's final home.
	if err := os.Rename(tmpDir, out); err != nil {
		// If this failed, we still need to clean up tmpDir.
		return errors.Join(err, os.RemoveAll(tmpDir))
	}

	// We have now generated a SDK, the only thing left to do is link it into the existing project.

	// TODO: Copied from [packages.LinkPackage]. This might be true, but we should
	// still call into the YAML language host (which can then do nothing). Languages
	// should not be special here.
	if runtimeInfo.Name() == "yaml" {
		return nil // Nothing to do for YAML
	}

	sdkPath, err := filepath.Rel(projectDir, out)
	if err != nil {
		return err
	}
	descriptor, err := boundSchema.Descriptor(ctx)
	if err != nil {
		return err
	}
	instructions, err := servers.lang.Link(
		plugin.NewProgramInfo(projectDir, projectDir, ".", runtimeInfo.Options()),
		[]workspace.LinkablePackageDescriptor{{
			Path:       sdkPath,
			Descriptor: descriptor,
		}},
		servers.grpc.Addr(),
	)
	if err != nil {
		return fmt.Errorf("linking package: %w", err)
	}
	fmt.Fprintln(w.stderr, instructions)
	return nil
}

type servers struct {
	pctx *plugin.Context
	lang plugin.LanguageRuntime
	grpc *plugin.GrpcServer
}

func (s servers) Close() error { return errors.Join(s.lang.Close(), s.grpc.Close()) }

func (w Workspace) servers(ctx context.Context, language string, dir string) (servers, error) {
	languageRuntime, err := w.host.LanguageRuntime(language)
	if err != nil {
		return servers{}, err
	}

	pctx := plugin.NewContextWithHost(ctx, w.sink, w.statusSink, noCloseHost{w.host}, dir, dir, w.parentSpan)
	loader := schema.NewPluginLoader(pctx.Host)
	loaderServer := schema.NewLoaderServer(loader)
	grpcServer, err := plugin.NewServer(pctx, schema.LoaderRegistration(loaderServer))
	if err != nil {
		return servers{}, errors.Join(err, languageRuntime.Close(), pctx.Close())
	}
	return servers{
		pctx: pctx,
		lang: languageRuntime,
		grpc: grpcServer,
	}, nil
}

func (w Workspace) genSDK(ctx context.Context, language string, pkg *schema.Package) (string, servers, error) {
	jsonBytes, err := pkg.MarshalJSON()
	if err != nil {
		return "", servers{}, err
	}
	tmpDir, err := os.MkdirTemp("", "pulumi-package-")
	if err != nil {
		return "", servers{}, fmt.Errorf("unable to make temp dir: %w", err)
	}
	s, err := w.servers(ctx, language, tmpDir)
	if err != nil {
		return "", servers{}, errors.Join(err, os.RemoveAll(tmpDir))
	}

	diags, err := s.lang.GeneratePackage(tmpDir, string(jsonBytes), nil, s.grpc.Addr(), nil, true /* local */)
	if err != nil {
		return "", servers{}, errors.Join(err, os.RemoveAll(tmpDir), s.Close())
	}

	if diags.HasErrors() {
		return "", servers{}, errors.Join(fmt.Errorf("generation failed: %w", diags), os.RemoveAll(tmpDir), s.Close())
	}

	return tmpDir, s, nil
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
