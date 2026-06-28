// Copyright 2024, Pulumi Corporation.
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

package packages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/oci"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pkgCmdUtil "github.com/pulumi/pulumi/pkg/v3/util/cmdutil"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/errutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v2"
)

// BindSpec binds a PackageSpec into a Package, returning any error or error diagnostics encountered.
func BindSpec(spec schema.PackageSpec, loader schema.Loader) (*schema.Package, error) {
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

// InstallPackage installs a package to the project by generating an SDK and linking it.
// It returns the path to the installed package.
func InstallPackage(stdout io.Writer, ws pkgWorkspace.Context, proj workspace.BaseProject, pctx *plugin.Context,
	language, root, schemaSource string, parameters plugin.ParameterizeParameters,
	registry registry.Registry, e env.Env, concurrency int,
) (*schema.Package, *workspace.PackageSpec, hcl.Diagnostics, error) {
	pkgSpec, specOverride, err := SchemaFromSchemaSource(ws, pctx, schemaSource, parameters, registry, e, concurrency)
	if err != nil {
		var diagErr hcl.Diagnostics
		if errors.As(err, &diagErr) {
			return nil, nil, nil, fmt.Errorf("failed to get schema. Diagnostics: %w", errors.Join(diagErr.Errs()...))
		}
		return nil, nil, nil, fmt.Errorf("failed to get schema: %w", err)
	}

	pkg, err := BindSpec(*pkgSpec, schema.NewPluginLoader(pctx))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to bind schema: %w", err)
	}

	tempOut, err := os.MkdirTemp("", "pulumi-package-")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempOut)

	local := true

	// We _always_ want SupportPack turned on for `package add`, this is an option on schemas because it can change
	// things like module paths for Go and we don't want every user using gen-sdk to be affected by that. But for
	// `package add` we know that this is just a local package and it's ok for module paths and similar to be different.
	pkg.SupportPack = true

	diags, err := GenSDK(
		pctx.Request(),
		registry,
		language,
		tempOut,
		pkg,
		"",    /*overlays*/
		local, /*local*/
	)
	if err != nil {
		return nil, nil, diags, fmt.Errorf("failed to generate SDK: %w", err)
	}

	out := filepath.Join(root, "sdks")
	fmt.Fprintf(stdout, "Successfully generated an SDK for the %s package at %s\n", pkg.Name, out)

	err = os.MkdirAll(out, 0o755)
	if err != nil {
		return nil, nil, diags, fmt.Errorf("failed to create directory for SDK: %w", err)
	}

	outName := pkg.Name
	if pkg.Namespace != "" {
		outName = pkg.Namespace + "-" + outName
	}
	out = filepath.Join(out, outName)

	// If directory already exists, remove it completely before copying new files
	if _, err := os.Stat(out); err == nil {
		if err := os.RemoveAll(out); err != nil {
			return nil, nil, diags, fmt.Errorf("failed to clean existing SDK directory: %w", err)
		}
	}

	err = fsutil.CopyFile(out, filepath.Join(tempOut, language), nil)
	if err != nil {
		return nil, nil, diags, fmt.Errorf("failed to move SDK to project: %w", err)
	}

	// Link the package to the project
	if err := LinkPackages(&LinkPackagesContext{
		Writer:        stdout,
		Project:       proj,
		Language:      language,
		Root:          root,
		Packages:      []PackageToLink{{Pkg: pkg, Out: out}},
		PluginContext: pctx,
		Install:       true,
	}); err != nil {
		return nil, nil, diags, err
	}

	return pkg, specOverride, diags, nil
}

func GenSDK(
	ctx context.Context, reg registry.Registry, language, out string, pkg *schema.Package, overlays string, local bool,
) (hcl.Diagnostics, error) {
	tracer := otel.Tracer("pulumi-cli")
	_, span := cmdutil.StartSpan(ctx, tracer, "generate-sdk",
		trace.WithAttributes(
			attribute.String("language", language),
			attribute.String("package", pkg.Name),
		))
	defer span.End()

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get current working directory: %w", err)
	}

	generatePackage := func(directory string, pkg *schema.Package, extraFiles map[string][]byte) (hcl.Diagnostics, error) {
		// Ensure the target directory is clean, but created.
		err = os.RemoveAll(directory)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		err := os.MkdirAll(directory, 0o700)
		if err != nil {
			return nil, err
		}

		jsonBytes, err := pkg.MarshalJSON()
		if err != nil {
			return nil, err
		}

		pCtx, err := NewPluginContext(cwd, reg)
		if err != nil {
			return nil, fmt.Errorf("create plugin context: %w", err)
		}
		defer contract.IgnoreClose(pCtx.Host)
		defer contract.IgnoreClose(pCtx)
		languagePlugin, err := pCtx.Host.LanguageRuntime(pCtx, language)
		if err != nil {
			return nil, err
		}

		loader := schema.NewPluginLoader(pCtx)
		loaderServer := schema.NewLoaderServer(loader)
		grpcServer, err := plugin.NewServer(pCtx, schema.LoaderRegistration(loaderServer))
		if err != nil {
			return nil, err
		}
		defer contract.IgnoreClose(grpcServer)

		diags, err := languagePlugin.GeneratePackage(
			ctx, directory, string(jsonBytes), extraFiles, grpcServer.Addr(), nil, local)
		if err != nil {
			return diags, err
		}

		if diags.HasErrors() {
			return diags, fmt.Errorf("generation failed: %w", diags)
		}

		return diags, nil
	}

	extraFiles := make(map[string][]byte)
	if overlays != "" {
		fsys := os.DirFS(filepath.Join(overlays, language))
		err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}

			contents, err := fs.ReadFile(fsys, path)
			if err != nil {
				return fmt.Errorf("read overlay file %q: %w", path, err)
			}

			extraFiles[path] = contents
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("read overlay directory %q: %w", overlays, err)
		}
	}

	root := filepath.Join(out, language)
	return generatePackage(root, pkg, extraFiles)
}

type PackageToLink struct {
	// The output directory where the SDK package to be linked is located.
	Out string
	Pkg *schema.Package
}

type LinkPackagesContext struct {
	// The project into which the SDK package is being linked.
	Project workspace.BaseProject
	// The programming language of the SDK package being linked.
	Language string
	// The root directory of the project to which the SDK package is being linked.
	Root string
	// The packages to link to the project.
	Packages []PackageToLink
	// A plugin context to load languages and providers.
	PluginContext *plugin.Context
	// True if the linked SDK package should be installed into the project it is being added to. If this is false, the
	// package will be linked (e.g. an entry added to package.json), but not installed (e.g. its contents unpacked into
	// node_modules).
	Install bool
	Writer  io.Writer
}

// LinkPackages links a locally generated SDK to an existing project.
// Currently Java is not supported and will print instructions for manual linking.
func LinkPackages(ctx *LinkPackagesContext) error {
	root, err := filepath.Abs(ctx.Root)
	if err != nil {
		return err
	}
	if ctx.Project.RuntimeInfo().Name() == "" {
		return errors.New("cannot link packages into a project without a runtime")
	}
	languagePlugin, err := ctx.PluginContext.Host.LanguageRuntime(ctx.PluginContext, ctx.Project.RuntimeInfo().Name())
	if err != nil {
		return err
	}

	// Pre-load the schemas into the cached loader. This allows the loader to respond to GetSchema requests for file
	// based schemas.
	entries := make(map[string]schema.PackageReference, len(ctx.Packages))
	for _, pkg := range ctx.Packages {
		entries[pkg.Pkg.Identity()] = pkg.Pkg.Reference()
	}
	loader := schema.NewCachedLoaderWithEntries(schema.NewPluginLoader(ctx.PluginContext), entries)
	loaderServer := schema.NewLoaderServer(loader)
	grpcServer, err := plugin.NewServer(ctx.PluginContext, schema.LoaderRegistration(loaderServer))
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(grpcServer)

	deps := make([]workspace.LinkablePackageDescriptor, 0, len(ctx.Packages))
	for _, packageToLink := range ctx.Packages {
		out, err := filepath.Abs(packageToLink.Out)
		if err != nil {
			return err
		}
		packagePath, err := filepath.Rel(root, out)
		if err != nil {
			return err
		}
		packageDescriptor, err := packageToLink.Pkg.Descriptor(ctx.PluginContext.Base())
		if err != nil {
			return fmt.Errorf("getting package descriptor: %w", err)
		}
		deps = append(deps, workspace.LinkablePackageDescriptor{
			Path:       packagePath,
			Descriptor: packageDescriptor,
		})
	}
	programInfo := plugin.NewProgramInfo(root, root, ".", ctx.Project.RuntimeInfo().Options())
	instructions, err := languagePlugin.Link(ctx.PluginContext.Request(), programInfo, deps, grpcServer.Addr())
	if err != nil {
		return fmt.Errorf("linking package: %w", err)
	}

	if ctx.Install {
		if err = pkgCmdUtil.InstallDependencies(
			ctx.PluginContext.Request(), languagePlugin, plugin.InstallDependenciesRequest{
				Info: programInfo,
			}, ctx.Writer, ctx.Writer); err != nil {
			return errutil.ErrorWithStderr(err, "installing dependencies")
		}
	}

	fmt.Fprintln(ctx.Writer, instructions)
	return nil
}

func NewPluginContext(cwd string, reg registry.Registry) (*plugin.Context, error) {
	// Helper used by callers without a *cobra.Command writer; emits to
	// process stderr.
	sink := diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{ //nolint:forbidigo
		Color: cmdutil.GetGlobalColorization(),
	})
	pluginHost, err := pkghost.New(context.TODO(), sink, sink, nil,
		pkgWorkspace.EnsureLanguageInstalled, schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext,
		packageworkspace.NewResolverServer(reg))
	if err != nil {
		return nil, err
	}
	pluginCtx, err := plugin.NewContext(context.TODO(), sink, sink, pluginHost, nil, cwd, nil, true, nil)
	if err != nil {
		return nil, errors.Join(err, pluginHost.Close())
	}
	return pluginCtx, nil
}

func setSpecNamespace(spec *schema.PackageSpec, pluginSpec workspace.PluginDescriptor) {
	if spec.Namespace == "" && pluginSpec.IsGitPlugin() {
		namespaceRegex := regexp.MustCompile(`git://[^/]+/([^/]+)/`)
		matches := namespaceRegex.FindStringSubmatch(pluginSpec.PluginDownloadURL)
		if len(matches) == 2 {
			spec.Namespace = strings.ToLower(matches[1])
		}
	}
}

// SchemaFromSchemaSource takes a schema source and returns its associated schema. A
// schema source is either a file (ending with .[json|y[a]ml]) or a plugin with an
// optional version:
//
//	FILE.[json|y[a]ml] | PLUGIN[@VERSION] | PATH_TO_PLUGIN
//
// The returned workspace.PackageSpec will be non-nil if and only if the schema is sourced
// from a plugin.
func SchemaFromSchemaSource(
	ws pkgWorkspace.Context,
	pctx *plugin.Context, packageSource string, parameters plugin.ParameterizeParameters, registry registry.Registry,
	env env.Env, concurrency int,
) (*schema.PackageSpec, *workspace.PackageSpec, error) {
	// An oci:// source is a pre-built provider *image* (the package model's "everything
	// is a plugin image"): resolve the ref to a running container and read its schema,
	// rather than installing and spawning a binary at a filesystem path. The image ref —
	// not a path — is what lands in Pulumi.yaml, so the engine later consumes only a ref.
	if ref, ok := strings.CutPrefix(packageSource, "oci://"); ok {
		return schemaFromImage(pctx, packageSource, ref, parameters)
	}

	var spec schema.PackageSpec
	if ext := filepath.Ext(packageSource); ext == ".yaml" || ext == ".yml" {
		if !parameters.Empty() {
			return nil, nil, errors.New("parameterization arguments are not supported for yaml files")
		}
		f, err := os.ReadFile(packageSource)
		if err != nil {
			return nil, nil, err
		}
		err = yaml.Unmarshal(f, &spec)
		if err != nil {
			return nil, nil, err
		}
		return &spec, nil, nil
	} else if ext == ".json" {
		if !parameters.Empty() {
			return nil, nil, errors.New("parameterization arguments are not supported for json files")
		}

		f, err := os.ReadFile(packageSource)
		if err != nil {
			return nil, nil, err
		}
		err = json.Unmarshal(f, &spec)
		if err != nil {
			return nil, nil, err
		}
		return &spec, nil, nil
	}

	p, packageSpec, err := ProviderFromSource(ws, pctx, packageSource, registry, env, concurrency)
	if err != nil {
		return nil, nil, err
	}
	defer contract.IgnoreClose(p)

	var request plugin.GetSchemaRequest
	if !parameters.Empty() {
		resp, err := p.Parameterize(pctx.Request(), plugin.ParameterizeRequest{
			Parameters: parameters,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("parameterize: %w", err)
		}

		request = plugin.GetSchemaRequest{
			SubpackageName:    resp.Name,
			SubpackageVersion: &resp.Version,
		}
	}

	tracer := otel.Tracer("pulumi-cli")
	_, schemaSpan := cmdutil.StartSpan(pctx.Request(), tracer, "get-schema",
		trace.WithAttributes(attribute.String("source", packageSource)))
	schema, err := p.GetSchema(pctx.Request(), request)
	schemaSpan.End()
	if err != nil {
		return nil, nil, err
	}
	err = json.Unmarshal(schema.Schema, &spec)
	if err != nil {
		return nil, nil, err
	}
	pluginSpec, err := workspace.NewPluginDescriptor(pctx.Request(), packageSource, apitype.ResourcePlugin, nil, "", nil)
	if err != nil {
		return nil, nil, err
	}
	if pluginSpec.PluginDownloadURL != "" {
		spec.PluginDownloadURL = pluginSpec.PluginDownloadURL
	}
	setSpecNamespace(&spec, pluginSpec)
	return &spec, &packageSpec, nil
}

// schemaFromImage extracts a package schema from a provider *image* (an oci:// source).
// It runs the image as a one-shot pod container, reads its schema over the attached
// connection, and returns a package spec whose Source is the oci:// ref — so the package
// recorded in Pulumi.yaml is an image ref, the form the engine consumes at runtime. This
// is the dev-time analogue of executing a provider binary to read its schema, mirroring
// the existing GetSchema/Parameterize path below but sourced from a container.
func schemaFromImage(
	pctx *plugin.Context, source, ref string, parameters plugin.ParameterizeParameters,
) (*schema.PackageSpec, *workspace.PackageSpec, error) {
	p, stop, err := oci.ProviderFromImage(pctx, ref)
	if err != nil {
		return nil, nil, err
	}
	defer func() { contract.IgnoreError(stop()) }()
	defer contract.IgnoreClose(p)

	var request plugin.GetSchemaRequest
	if !parameters.Empty() {
		resp, err := p.Parameterize(pctx.Request(), plugin.ParameterizeRequest{Parameters: parameters})
		if err != nil {
			return nil, nil, fmt.Errorf("parameterize: %w", err)
		}
		request = plugin.GetSchemaRequest{SubpackageName: resp.Name, SubpackageVersion: &resp.Version}
	}

	schemaResp, err := p.GetSchema(pctx.Request(), request)
	if err != nil {
		return nil, nil, err
	}
	var spec schema.PackageSpec
	if err := json.Unmarshal(schemaResp.Schema, &spec); err != nil {
		return nil, nil, err
	}
	return &spec, &workspace.PackageSpec{Source: source}, nil
}

// ProviderFromSource takes a plugin name or path.
//
// PLUGIN[@VERSION] | PATH_TO_PLUGIN
func ProviderFromSource(
	ws pkgWorkspace.Context,
	pctx *plugin.Context, packageSource string, reg registry.Registry,
	e env.Env, concurrency int,
) (plugin.Provider, workspace.PackageSpec, error) {
	// Helper without a *cobra.Command writer; plumbing the writer into
	// packageworkspace.New would require a much larger API change.
	installCtx := packageworkspace.New(pluginstorage.Instance, ws, pctx, os.Stderr, os.Stderr, //nolint:forbidigo
		nil, packageworkspace.Options{})
	return providerFromSource(pctx, packageSource, reg, e, concurrency, installCtx)
}

// providerFromSource is the injectable core of [ProviderFromSource]. It performs all package
// resolution, installation, and launching through installCtx, allowing tests to substitute a
// mock [packageinstallation.Context] for the real, IO-performing [packageworkspace.Workspace].
func providerFromSource(
	pctx *plugin.Context, packageSource string, reg registry.Registry,
	e env.Env, concurrency int, installCtx packageinstallation.Context,
) (plugin.Provider, workspace.PackageSpec, error) {
	var version string
	if parts := strings.SplitN(packageSource, "@", 2); len(parts) > 1 {
		packageSource = parts[0]
		version = parts[1]
	}
	packageSpec := workspace.PackageSpec{Source: packageSource, Version: version}
	{
		proj, _, err := installCtx.LoadBaseProjectFrom(pctx.Request(), pctx.Pwd)
		if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
			return nil, workspace.PackageSpec{}, fmt.Errorf("error loading Pulumi Project: %w", err)
		}
		if proj != nil {
			if remap, ok := proj.GetPackageSpecs()[packageSource]; ok {
				packageSpec = remap
			}
		}
	}

	f, spec, _, err := packageinstallation.InstallPlugin(pctx.Request(), packageSpec, nil, "", packageinstallation.Options{
		Options: packageresolution.Options{
			ResolveWithRegistry:                        !e.GetBool(env.DisableRegistryResolve),
			ResolveVersionWithLocalWorkspace:           true,
			AllowNonInvertableLocalWorkspaceResolution: true,
		},
		Concurrency: concurrency,
	}, reg, installCtx)
	if err != nil {
		return nil, workspace.PackageSpec{}, fmt.Errorf("unable to install %s: %w", packageSpec, err)
	}
	p, err := f(pctx.Request(), ".")
	if err != nil {
		return nil, workspace.PackageSpec{}, fmt.Errorf("unable to run %s: %w", packageSpec, err)
	}
	return p, spec, nil
}
