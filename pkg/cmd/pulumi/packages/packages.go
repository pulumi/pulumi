// Copyright 2024-2025, Pulumi Corporation.
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
	"runtime"
	"strings"

	"github.com/hashicorp/hcl/v2"
	cmdDiag "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/diag"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgCmdUtil "github.com/pulumi/pulumi/pkg/v3/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/v3/util/pdag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"gopkg.in/yaml.v2"
)

// BindSpec binds a PackageSpec into a Package, returning any error or error diagnostics encountered.
func BindSpec(spec schema.PackageSpec) (*schema.Package, error) {
	pkg, diags, err := schema.BindSpec(spec, nil, schema.ValidationOptions{
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

func InstallPackageV2(
	ctx context.Context, proj workspace.BaseProject, root string, registry registry.Registry,
	parallelism int,
	stdout, stderr io.Writer,
) error {
	pkgs := proj.GetPackageSpecs()
	if len(pkgs) == 0 {
		return nil
	}

	fmt.Fprintln(stderr, "Installing packages...")

	type node struct {
		name        string
		packageSpec func(context.Context) error
	}

	installPackage := func(
		cwd, name string, proj workspace.BaseProject, packageSpec workspace.PackageSpec,
	) func(context.Context) error {
		return func(ctx context.Context) error {
			fmt.Fprintf(stderr, "Installing package '%s'...\n", name)

			pctx, err := plugin.NewContextWithRoot(ctx,
				cmdutil.Diag(),
				cmdutil.Diag(),
				nil, // host
				cwd, // pwd
				cwd, // root
				proj.RuntimeInfo().Options(),
				false, // disableProviderPreview
				nil,   // tracingSpan
				nil,   // Plugins
				proj.GetPackageSpecs(),
				nil, // config
				nil, // debugging
			)
			if err != nil {
				return err
			}

			installSource := packageSpec.Source
			if !plugin.IsLocalPluginPath(ctx, installSource) && packageSpec.Version != "" {
				installSource = fmt.Sprintf("%s@%s", installSource, packageSpec.Version)
			}

			parameters := &plugin.ParameterizeArgs{Args: packageSpec.Parameters}
			_, _, diags, err := InstallPackage(
				proj, pctx, proj.RuntimeInfo().Name(), pctx.Root, installSource, parameters, registry)
			cmdDiag.PrintDiagnostics(pctx.Diag, diags)
			if err != nil {
				return errors.Join(
					fmt.Errorf("failed to install package '%s': %w", name, err),
					pctx.Close(),
				)
			}

			fmt.Fprintf(stderr, "Package '%s' installed successfully\n", name)
			return pctx.Close()
		}
	}

	installPlugin := func(path string, proj *workspace.PluginProject) func(context.Context) error {
		return func(ctx context.Context) error {
			pctx, err := plugin.NewContextWithRoot(ctx,
				cmdutil.Diag(),
				cmdutil.Diag(),
				nil,  // host
				path, // pwd
				path, // root
				proj.RuntimeInfo().Options(),
				false, // disableProviderPreview
				nil,   // tracingSpan
				nil,   // Plugins
				proj.GetPackageSpecs(),
				nil, // config
				nil, // debugging
			)
			if err != nil {
				return err
			}

			if err := pkgWorkspace.InstallPluginAtPath(pctx, proj, stdout, stderr); err != nil {
				return errors.Join(fmt.Errorf("installing at '%s': %w", pctx.Pwd, err), pctx.Close())
			}
			return pctx.Close()
		}
	}

	var wg pdag.DAG[node]
	seen := map[string]pdag.Node{}
	var findPlugins func(root pdag.Node, cwd string, proj workspace.BaseProject) error
	findPlugins = func(root pdag.Node, cwd string, proj workspace.BaseProject) error {
		for name, packageSpec := range proj.GetPackageSpecs() {
			var pluginInstall *pdag.Node
			if plugin.IsLocalPluginPath(ctx, packageSpec.Source) {
				// If the package is a local spec, then we need to install it and the
				// packages that it depends on.
				pluginYaml := filepath.Join(packageSpec.Source, "PulumiPlugin.yaml")
				pluginProject, err := workspace.LoadPluginProject(pluginYaml)
				if err != nil {
					return fmt.Errorf("Failed to load plugin project '%s': %w", name, err)
				}
				absPluginSource, err := filepath.Abs(packageSpec.Source)
				if err != nil {
					return err
				}

				if n, ok := seen[absPluginSource]; ok {
					pluginInstall = &n
				} else {
					pkg := wg.NewNode(node{name, installPlugin(absPluginSource, pluginProject)})
					if err := wg.NewEdge(pkg, root); err != nil {
						return err
					}
					pluginInstall = &pkg
					seen[absPluginSource] = pkg
					if err := findPlugins(pkg, packageSpec.Source, pluginProject); err != nil {
						return err
					}
				}
			}

			installPkg := wg.NewNode(node{name, installPackage(cwd, name, proj, packageSpec)})
			if pluginInstall != nil {
				if err := wg.NewEdge(*pluginInstall, installPkg); err != nil {
					return err
				}
			}
			// Ensure that we install this package before we install the plugin.
			if err := wg.NewEdge(installPkg, root); err != nil {
				return err
			}
		}
		return nil
	}

	// Search for plugins
	if err := findPlugins(
		wg.NewNode(node{name: "root", packageSpec: func(context.Context) error { return nil }}),
		root,
		proj,
	); err != nil {
		var cycle pdag.ErrorCycle[node]
		if errors.As(err, &cycle) {
			cyclePath := make([]string, len(cycle.Cycle))
			for i, n := range cycle.Cycle {
				cyclePath[i] = n.name
			}
			return fmt.Errorf("Cycle found: %s", strings.Join(cyclePath, " -> "))
		}
		return err
	}

	return wg.Walk(ctx, func(ctx context.Context, f node) error {
		return f.packageSpec(ctx)
	}, pdag.MaxProcs(parallelism))
}

// InstallPackage installs a package to the project by generating an SDK and linking it.
// It returns the path to the installed package.
func InstallPackage(proj workspace.BaseProject, pctx *plugin.Context, language, root,
	schemaSource string, parameters plugin.ParameterizeParameters,
	registry registry.Registry,
) (*schema.Package, *workspace.PackageSpec, hcl.Diagnostics, error) {
	pkgSpec, specOverride, err := SchemaFromSchemaSource(pctx, schemaSource, parameters, registry)
	if err != nil {
		var diagErr hcl.Diagnostics
		if errors.As(err, &diagErr) {
			return nil, nil, nil, fmt.Errorf("failed to get schema. Diagnostics: %w", errors.Join(diagErr.Errs()...))
		}
		return nil, nil, nil, fmt.Errorf("failed to get schema: %w", err)
	}

	pkg, err := BindSpec(*pkgSpec)
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
	fmt.Printf("Successfully generated an SDK for the %s package at %s\n", pkg.Name, out)

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

	err = CopyAll(out, filepath.Join(tempOut, language))
	if err != nil {
		return nil, nil, diags, fmt.Errorf("failed to move SDK to project: %w", err)
	}

	// Link the package to the project
	if err := LinkPackages(&LinkPackagesContext{
		Writer:        os.Stdout,
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

func GenSDK(language, out string, pkg *schema.Package, overlays string, local bool) (hcl.Diagnostics, error) {
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

		pCtx, err := NewPluginContext(cwd)
		if err != nil {
			return nil, fmt.Errorf("create plugin context: %w", err)
		}
		defer contract.IgnoreClose(pCtx)
		languagePlugin, err := pCtx.Host.LanguageRuntime(language)
		if err != nil {
			return nil, err
		}

		loader := schema.NewPluginLoader(pCtx.Host)
		loaderServer := schema.NewLoaderServer(loader)
		grpcServer, err := plugin.NewServer(pCtx, schema.LoaderRegistration(loaderServer))
		if err != nil {
			return nil, err
		}
		defer contract.IgnoreClose(grpcServer)

		diags, err := languagePlugin.GeneratePackage(directory, string(jsonBytes), extraFiles, grpcServer.Addr(), nil, local)
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
	if ctx.Language == "yaml" {
		return nil // Nothing to do for YAML
	}
	return linkPackage(ctx)
}

// linkPackage links a locally generated SDK into a project using `Language.Link`.
func linkPackage(ctx *LinkPackagesContext) error {
	root, err := filepath.Abs(ctx.Root)
	if err != nil {
		return err
	}
	languagePlugin, err := ctx.PluginContext.Host.LanguageRuntime(ctx.Project.RuntimeInfo().Name())
	if err != nil {
		return err
	}

	// Pre-load the schemas into the cached loader. This allows the loader to respond to GetSchema requests for file
	// based schemas.
	entries := make(map[string]schema.PackageReference, len(ctx.Packages))
	for _, pkg := range ctx.Packages {
		entries[pkg.Pkg.Identity()] = pkg.Pkg.Reference()
	}
	loader := schema.NewCachedLoaderWithEntries(schema.NewPluginLoader(ctx.PluginContext.Host), entries)
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
	instructions, err := languagePlugin.Link(programInfo, deps, grpcServer.Addr())
	if err != nil {
		return fmt.Errorf("linking package: %w", err)
	}

	if ctx.Install {
		if err = pkgCmdUtil.InstallDependencies(languagePlugin, plugin.InstallDependenciesRequest{
			Info: programInfo,
		}, ctx.Writer, ctx.Writer); err != nil {
			return fmt.Errorf("installing dependencies: %w", err)
		}
	}

	fmt.Fprintln(ctx.Writer, instructions)
	return nil
}

// CopyAll copies src to dst. If src is a directory, its contents will be copied
// recursively.
func CopyAll(dst string, src string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		// Recursively copy all files in a directory.
		files, err := os.ReadDir(src)
		if err != nil {
			return fmt.Errorf("read dir: %w", err)
		}
		for _, file := range files {
			name := file.Name()
			copyerr := CopyAll(filepath.Join(dst, name), filepath.Join(src, name))
			if copyerr != nil {
				return copyerr
			}
		}
	} else if info.Mode().IsRegular() {
		// Copy files by reading and rewriting their contents.  Skip other special files.
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		dstdir := filepath.Dir(dst)
		if err = os.MkdirAll(dstdir, 0o700); err != nil {
			return err
		}
		if err = os.WriteFile(dst, data, info.Mode()); err != nil {
			return err
		}
	}

	return nil
}

func NewPluginContext(cwd string) (*plugin.Context, error) {
	sink := diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
		Color: cmdutil.GetGlobalColorization(),
	})
	pluginCtx, err := plugin.NewContext(context.TODO(), sink, sink, nil, nil, cwd, nil, true, nil)
	if err != nil {
		return nil, err
	}
	return pluginCtx, nil
}

func setSpecNamespace(spec *schema.PackageSpec, pluginSpec workspace.PluginSpec) {
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
func SchemaFromSchemaSource(
	pctx *plugin.Context, packageSource string, parameters plugin.ParameterizeParameters, registry registry.Registry,
) (*schema.PackageSpec, *workspace.PackageSpec, error) {
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

	p, specOverride, err := ProviderFromSource(pctx, packageSource, registry)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		contract.IgnoreError(pctx.Host.CloseProvider(p.Provider))
	}()

	var request plugin.GetSchemaRequest
	if !parameters.Empty() {
		if p.AlreadyParameterized {
			return nil, nil,
				fmt.Errorf("cannot specify parameters since %s is already parameterized", packageSource)
		}
		resp, err := p.Provider.Parameterize(pctx.Request(), plugin.ParameterizeRequest{
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

	schema, err := p.Provider.GetSchema(pctx.Request(), request)
	if err != nil {
		return nil, nil, err
	}
	err = json.Unmarshal(schema.Schema, &spec)
	if err != nil {
		return nil, nil, err
	}
	pluginSpec, err := workspace.NewPluginSpec(pctx.Request(), packageSource, apitype.ResourcePlugin, nil, "", nil)
	if err != nil {
		return nil, nil, err
	}
	if pluginSpec.PluginDownloadURL != "" {
		spec.PluginDownloadURL = pluginSpec.PluginDownloadURL
	}
	setSpecNamespace(&spec, pluginSpec)
	return &spec, specOverride, nil
}

type Provider struct {
	Provider plugin.Provider

	AlreadyParameterized bool
}

// ProviderFromSource takes a plugin name or path.
//
// PLUGIN[@VERSION] | PATH_TO_PLUGIN
func ProviderFromSource(
	pctx *plugin.Context, packageSource string, reg registry.Registry,
) (Provider, *workspace.PackageSpec, error) {
	pluginSpec, err := workspace.NewPluginSpec(pctx.Request(), packageSource, apitype.ResourcePlugin, nil, "", nil)
	if err != nil {
		return Provider{}, nil, err
	}
	descriptor := workspace.PackageDescriptor{
		PluginSpec: pluginSpec,
	}

	installDescriptor := func(descriptor workspace.PackageDescriptor) (Provider, error) {
		p, err := pctx.Host.Provider(descriptor)
		if err == nil {
			return Provider{
				Provider:             p,
				AlreadyParameterized: descriptor.Parameterization != nil,
			}, nil
		}
		// There is an executable or directory with the same name, so suggest that
		if info, statErr := os.Stat(descriptor.Name); statErr == nil && (isExecutable(info) || info.IsDir()) {
			return Provider{}, fmt.Errorf("could not find installed plugin %s, did you mean ./%[1]s: %w", descriptor.Name, err)
		}

		// TOOD: This block shouldn't be necessary after https://github.com/pulumi/pulumi/pull/21119
		if descriptor.SubDir() != "" {
			path, err := descriptor.DirPath()
			if err != nil {
				return Provider{}, err
			}
			info, statErr := os.Stat(filepath.Join(path, descriptor.SubDir()))
			if statErr == nil && info.IsDir() {
				// The plugin is already installed.  But since it is in a subdirectory, it could be that
				// we previously installed a plugin in a different subdirectory of the same repository.
				// This is why the provider might have failed to start up.  Install the dependencies
				// and try again.
				depErr := pkgWorkspace.InstallDependenciesForPluginSpec(pctx.Base(), descriptor.PluginSpec,
					os.Stderr /* pipe stdout and stderr to stderr */, os.Stderr)
				if depErr != nil {
					return Provider{}, fmt.Errorf("installing plugin dependencies: %w", depErr)
				}
				p, err := pctx.Host.Provider(descriptor)
				if err != nil {
					return Provider{}, err
				}
				return Provider{Provider: p}, nil
			}
		}

		// Try and install the plugin if it was missing and try again, unless auto plugin installs are turned off.
		var missingError *workspace.MissingError
		if !errors.As(err, &missingError) || env.DisableAutomaticPluginAcquisition.Value() {
			return Provider{}, err
		}

		log := func(sev diag.Severity, msg string) {
			pctx.Host.Log(sev, "", msg, 0)
		}

		_, err = pkgWorkspace.InstallPlugin(pctx.Base(), descriptor.PluginSpec, log)
		if err != nil {
			return Provider{}, err
		}

		p, err = pctx.Host.Provider(descriptor)
		if err != nil {
			return Provider{}, err
		}

		return Provider{Provider: p}, nil
	}

	setupProvider := func(
		descriptor workspace.PackageDescriptor, specOverride *workspace.PackageSpec,
	) (Provider, *workspace.PackageSpec, error) {
		p, err := installDescriptor(descriptor)
		if err != nil {
			return Provider{}, nil, err
		}
		if descriptor.Parameterization != nil {
			_, err := p.Provider.Parameterize(pctx.Request(), plugin.ParameterizeRequest{
				Parameters: &plugin.ParameterizeValue{
					Name:    descriptor.Parameterization.Name,
					Version: descriptor.Parameterization.Version,
					Value:   descriptor.Parameterization.Value,
				},
			})
			if err != nil {
				return Provider{}, nil, fmt.Errorf("failed to parameterize %s: %w", p.Provider.Pkg().Name(), err)
			}
		}
		return p, specOverride, nil
	}

	result, err := packageresolution.Resolve(
		pctx.Base(),
		reg,
		packageresolution.DefaultWorkspace(),
		pluginSpec,
		packageresolution.Options{
			DisableRegistryResolve:      env.DisableRegistryResolve.Value(),
			Experimental:                env.Experimental.Value(),
			IncludeInstalledInWorkspace: true,
		},
		pctx.Root,
	)
	if err != nil {
		var packageNotFoundErr *packageresolution.PackageNotFoundError
		if errors.As(err, &packageNotFoundErr) {
			for _, suggested := range packageNotFoundErr.Suggestions() {
				pctx.Diag.Infof(diag.Message("", "%s/%s/%s@%s is a similar package"),
					suggested.Source, suggested.Publisher, suggested.Name, suggested.Version)
			}
		}
		return Provider{}, nil, fmt.Errorf("Unable to resolve package from name: %w", err)
	}

	switch res := result.(type) {
	case packageresolution.LocalPathResult:
		return setupProviderFromPath(res.LocalPluginPathAbs, pctx)
	case packageresolution.ExternalSourceResult, packageresolution.InstalledInWorkspaceResult:
		return setupProvider(descriptor, nil)
	case packageresolution.RegistryResult:
		return setupProviderFromRegistryMeta(res.Metadata, setupProvider)
	default:
		contract.Failf("Unexpected result type: %T", result)
		return Provider{}, nil, nil
	}
}

func setupProviderFromPath(packageSource string, pctx *plugin.Context) (Provider, *workspace.PackageSpec, error) {
	info, err := os.Stat(packageSource)
	if os.IsNotExist(err) {
		return Provider{}, nil, fmt.Errorf("could not find file %s", packageSource)
	} else if err != nil {
		return Provider{}, nil, err
	} else if !info.IsDir() && !isExecutable(info) {
		if p, err := filepath.Abs(packageSource); err == nil {
			packageSource = p
		}
		return Provider{}, nil, fmt.Errorf("plugin at path %q not executable", packageSource)
	}

	p, err := plugin.NewProviderFromPath(pctx.Host, pctx, packageSource)
	if err != nil {
		return Provider{}, nil, err
	}
	return Provider{Provider: p}, nil, nil
}

func isExecutable(info fs.FileInfo) bool {
	// Windows doesn't have executable bits to check
	if runtime.GOOS == "windows" {
		return !info.IsDir()
	}
	return info.Mode()&0o111 != 0 && !info.IsDir()
}

func setupProviderFromRegistryMeta(
	meta apitype.PackageMetadata,
	setupProvider func(workspace.PackageDescriptor, *workspace.PackageSpec) (Provider, *workspace.PackageSpec, error),
) (Provider, *workspace.PackageSpec, error) {
	spec := workspace.PluginSpec{
		Name:              meta.Name,
		Kind:              apitype.ResourcePlugin,
		Version:           &meta.Version,
		PluginDownloadURL: meta.PluginDownloadURL,
	}
	var params *workspace.Parameterization
	if meta.Parameterization != nil {
		spec.Name = meta.Parameterization.BaseProvider.Name
		spec.Version = &meta.Parameterization.BaseProvider.Version
		params = &workspace.Parameterization{
			Name:    meta.Name,
			Version: meta.Version,
			Value:   meta.Parameterization.Parameter,
		}
	}
	return setupProvider(workspace.NewPackageDescriptor(spec, params), &workspace.PackageSpec{
		Source:  meta.Source + "/" + meta.Publisher + "/" + meta.Name,
		Version: meta.Version.String(),
	})
}
