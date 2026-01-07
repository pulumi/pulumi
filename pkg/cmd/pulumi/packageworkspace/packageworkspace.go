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
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/pkg/v3/util"
	"github.com/pulumi/pulumi/pkg/v3/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	diagutils "github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
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
	host plugin.Host, stdout, stderr io.Writer,
	parentSPan opentracing.Span, options Options,
) Workspace {
	return Workspace{packageresolution.DefaultWorkspace(), host, stdout, stderr, options, parentSPan}
}

type Workspace struct {
	packageresolution.PluginWorkspace
	host           plugin.Host
	stdout, stderr io.Writer
	options        Options
	parentSpan     opentracing.Span
}

func (Workspace) GetPluginPath(ctx context.Context, spec workspace.PluginDescriptor) (string, error) {
	path, err := spec.DirPath()
	if err != nil {
		return "", err
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
	ctx context.Context, pluginSpec workspace.PluginDescriptor,
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
		fmt.Fprintf(w.stderr, "Downloading provider: %s\n", pluginSpec.Name)
		return stream
	}

	retry := func(err error, attempt int, limit int, delay time.Duration) {
		fmt.Fprintf(w.stderr, "error downloading provider: %s\n"+
			"Will retry in %v [%d/%d]", err, delay, attempt, limit)
	}

	logging.V(1).Infof("downloading provider %s", pluginSpec.Name)
	downloadedFile, err := workspace.DownloadToFile(ctx, pluginSpec, wrapper, retry)
	if err != nil {
		return "", nil, err
	}

	logging.V(1).Infof("unpacking provider %s", pluginSpec.Name)
	cleanup, err := pluginstorage.UnpackContents(
		ctx, pluginSpec, pluginstorage.TarPlugin(downloadedFile), true, /* reinstall */
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
func (w Workspace) LinkPackage(
	ctx context.Context, runtimeInfo *workspace.ProjectRuntimeInfo,
	projectDir string, provider plugin.Provider,
) error {
	schemaResponse, err := provider.GetSchema(ctx, plugin.GetSchemaRequest{})
	if err != nil {
		return err
	}

	var schemaSpec schema.PackageSpec
	if err := json.Unmarshal(schemaResponse.Schema, &schemaSpec); err != nil {
		return err
	}

	boundSchema, err := bindSpec(schemaSpec, schema.NewPluginLoader(w.host))
	if err != nil {
		return fmt.Errorf("failed to bind schema: %w", err)
	}

	// We _always_ want SupportPack turned on for `package add`, this is an option on schemas because it can change
	// things like module paths for Go and we don't want every user using gen-sdk to be affected by that. But for
	// `package add` we know that this is just a local package and it's ok for module paths and similar to be different.
	boundSchema.SupportPack = true

	tmpDir, servers, err := w.genSDK(ctx, runtimeInfo.Name(), boundSchema)
	if err != nil {
		return fmt.Errorf("failed to generate SDK: %w", err)
	}

	pkgName := boundSchema.Name
	if boundSchema.Namespace != "" {
		pkgName = boundSchema.Namespace + "-" + pkgName
	}

	sdkDir := filepath.Join(projectDir, "sdks")
	out := filepath.Join(sdkDir, pkgName)

	// Make sure the out directory doesn't exist anymore.
	//
	// [os.RemoveAll] handles the case  where out doesn't exist.
	if err := os.RemoveAll(out); err != nil {
		return err
	}

	// Now move the temp directory to it's final home.
	if err := os.Mkdir(sdkDir, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("unable to create %q for generated SDKs: %w", sdkDir, err)
	}
	if err := os.Rename(tmpDir, out); err != nil {
		// If this failed, we still need to clean up tmpDir.
		return errors.Join(err, os.RemoveAll(tmpDir))
	}

	// We have now generated a SDK, the only thing left to do is link it into the existing project.

	// TODO[https://github.com/pulumi/pulumi/issues/21323]: Copied from
	// [packages.LinkPackage]. This might be true, but we should still call into the
	// YAML language host (which can then do nothing). Languages should not be special
	// here.
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

func (w Workspace) servers(ctx context.Context, language string, dir string) (servers, error) {
	languageRuntime, err := w.host.LanguageRuntime(language)
	if err != nil {
		return servers{}, err
	}

	d := diag.DefaultSink(w.stdout, w.stderr, diag.FormatOptions{
		Color: diagutils.GetGlobalColorization(),
	})

	pctx := plugin.NewContextWithHost(ctx, d, d, w.host, dir, dir, w.parentSpan)
	loader := schema.NewPluginLoader(pctx.Host)
	loaderServer := schema.NewLoaderServer(loader)
	grpcServer, err := plugin.NewServer(pctx, schema.LoaderRegistration(loaderServer))
	if err != nil {
		return servers{}, err
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
		return "", servers{}, errors.Join(err, os.RemoveAll(tmpDir))
	}

	if diags.HasErrors() {
		return "", servers{}, errors.Join(fmt.Errorf("generation failed: %w", diags), os.RemoveAll(tmpDir))
	}

	return tmpDir, s, nil
}

// Run a package from a directory, parameterized by params.
func (w Workspace) RunPackage(
	ctx context.Context, rootDir, pluginPath string, pkgName tokens.Package, params plugin.ParameterizeParameters,
	originalSpec workspace.PackageSpec,
) (plugin.Provider, error) {
	d := diag.DefaultSink(w.stdout, w.stderr, diag.FormatOptions{
		Color: diagutils.GetGlobalColorization(),
	})

	pctx := plugin.NewContextWithHost(ctx, d, d, w.host, rootDir, rootDir, w.parentSpan)
	p, err := plugin.NewProviderFromPath(w.host, pctx, pkgName, pluginPath)
	if err != nil {
		return nil, fmt.Errorf("could not run plugin at %q: %w", pluginPath, err)
	}
	var pluginResp *plugin.ParameterizeResponse
	if params != nil && !params.Empty() {
		resp, err := p.Parameterize(ctx, plugin.ParameterizeRequest{
			Parameters: params,
		})
		if err != nil {
			return nil, err
		}
		pluginResp = &resp
	}
	return pluginProvider{p, originalSpec, pluginResp}, nil
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

type pluginProvider struct {
	plugin.Provider

	originalSpec workspace.PackageSpec

	// The response to the parameterization *if* the parameterization was handled within the RunPlugin call that we
	// originally had.
	paramResp *plugin.ParameterizeResponse
}

func (p pluginProvider) GetSchema(
	ctx context.Context, req plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	if p.paramResp != nil {
		if req.SubpackageName != "" {
			return plugin.GetSchemaResponse{}, plugin.ErrDoubleParameterized
		}
		req.SubpackageName = p.paramResp.Name
		req.SubpackageVersion = &p.paramResp.Version
	}
	resp, err := p.Provider.GetSchema(ctx, req)
	if err != nil {
		return plugin.GetSchemaResponse{}, err
	}

	// Git based plugins are allowed to not be self-referential: know their version
	// and pluginDownloadURL. That requires the launching infrastructure to inject
	// that information into the returned schema.
	//
	// TODO[https://github.com/pulumi/pulumi/issues/21258]: Download lock files would
	// allow us to push this deeper through the plugin loading process.

	var pkgSpec schema.PackageSpec
	if json.Unmarshal(resp.Schema, &pkgSpec) != nil {
		// If we can't un-marshal, give up.
		return resp, nil
	}
	source := p.originalSpec.Source
	if p.originalSpec.Version != "" {
		source += "@" + p.originalSpec.Version
	}
	pd, err := workspace.NewPluginDescriptor(ctx, source, apitype.ResourcePlugin, nil, "", nil)
	if err == nil && pd.IsGitPlugin() {
		pkgSpec.PluginDownloadURL = pd.PluginDownloadURL
		if pd.Version != nil {
			pkgSpec.Version = pd.Version.String()
		}

		if pkgSpec.Namespace == "" {
			namespaceRegex := regexp.MustCompile(`git://[^/]+/([^/]+)/`)
			matches := namespaceRegex.FindStringSubmatch(pd.PluginDownloadURL)
			if len(matches) == 2 {
				pkgSpec.Namespace = strings.ToLower(matches[1])
			}
		}
	}
	bytes, err := json.Marshal(pkgSpec)
	contract.AssertNoErrorf(err, "schema.PackageSpec is safe to marshal")
	return plugin.GetSchemaResponse{Schema: bytes}, nil
}

func (p pluginProvider) Parameterize(
	ctx context.Context, req plugin.ParameterizeRequest,
) (plugin.ParameterizeResponse, error) {
	if p.paramResp != nil {
		return plugin.ParameterizeResponse{}, plugin.ErrDoubleParameterized
	}
	return p.Provider.Parameterize(ctx, req)
}
