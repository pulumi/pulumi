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
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	go_gen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgCmdUtil "github.com/pulumi/pulumi/pkg/v3/util/cmdutil"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"golang.org/x/mod/modfile"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v2"
)

// InstallPackage installs a package to the project by generating an SDK and linking it.
// It returns the path to the installed package.
func InstallPackage(proj workspace.BaseProject, pctx *plugin.Context, language, root,
	schemaSource string, parameters plugin.ParameterizeParameters,
	registry registry.Registry,
) (*schema.Package, *workspace.PackageSpec, hcl.Diagnostics, error) {
	pkg, specOverride, err := SchemaFromSchemaSource(pctx, schemaSource, parameters, registry)
	if err != nil {
		var diagErr hcl.Diagnostics
		if errors.As(err, &diagErr) {
			return nil, nil, nil, fmt.Errorf("failed to get schema. Diagnostics: %w", errors.Join(diagErr.Errs()...))
		}
		return nil, nil, nil, fmt.Errorf("failed to get schema: %w", err)
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

	packageDescriptor, err := pkg.Descriptor(pctx.Base())
	if err != nil {
		return nil, nil, diags, err
	}

	// Link the package to the project
	if err := LinkPackage(&LinkPackageContext{
		Writer:            os.Stdout,
		Project:           proj,
		Language:          language,
		Root:              root,
		Pkg:               pkg,
		PluginContext:     pctx,
		PackageDescriptor: packageDescriptor,
		Out:               out,
		Install:           true,
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
		defer contract.IgnoreClose(pCtx.Host)
		programInfo := plugin.NewProgramInfo(cwd, cwd, ".", nil)
		languagePlugin, err := pCtx.Host.LanguageRuntime(language, programInfo)
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

type LinkPackageContext struct {
	// The project into which the SDK package is being linked.
	Project workspace.BaseProject
	// The programming language of the SDK package being linked.
	Language string
	// The root directory of the project to which the SDK package is being linked.
	Root string
	// The schema of the Pulumi package from which the SDK being linked was generated.
	Pkg *schema.Package
	// A plugin context to load languages and providers.
	PluginContext *plugin.Context
	// The descriptor for the package to link.
	PackageDescriptor workspace.PackageDescriptor
	// The output directory where the SDK package to be linked is located.
	Out string
	// True if the linked SDK package should be installed into the project it is being added to. If this is false, the
	// package will be linked (e.g. an entry added to package.json), but not installed (e.g. its contents unpacked into
	// node_modules).
	Install bool
	Writer  io.Writer
}

// LinkPackage links a locally generated SDK to an existing project.
// Currently Java is not supported and will print instructions for manual linking.
func LinkPackage(ctx *LinkPackageContext) error {
	switch ctx.Language {
	case "go":
		return linkGoPackage(ctx)
	case "dotnet":
		return linkDotnetPackage(ctx)
	case "java":
		return printJavaLinkInstructions(ctx)
	case "yaml":
		return nil // Nothing to do for YAML
	default:
		return linkPackage(ctx)
	}
}

// linkPackage links a locally generated SDK into a project using `Language.Link`.
func linkPackage(ctx *LinkPackageContext) error {
	fmt.Fprintf(ctx.Writer, "Successfully generated an SDK for the %s package at %s\n", ctx.Pkg.Name, ctx.Out)
	fmt.Fprintf(ctx.Writer, "\n")
	root, err := filepath.Abs(ctx.Root)
	if err != nil {
		return err
	}
	programInfo := plugin.NewProgramInfo(root, root, ".", ctx.Project.RuntimeInfo().Options())
	languagePlugin, err := ctx.PluginContext.Host.LanguageRuntime(ctx.Project.RuntimeInfo().Name(), programInfo)
	if err != nil {
		return err
	}

	// Pre-load the schema into the cached loader. This allows the loader to respond to GetSchema requests for file
	// based schemas.
	loader := schema.NewCachedLoaderWithEntries(
		schema.NewPluginLoader(ctx.PluginContext.Host),
		map[string]schema.PackageReference{
			ctx.Pkg.Identity(): ctx.Pkg.Reference(),
		})
	loaderServer := schema.NewLoaderServer(loader)
	grpcServer, err := plugin.NewServer(ctx.PluginContext, schema.LoaderRegistration(loaderServer))
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(grpcServer)

	out, err := filepath.Abs(ctx.Out)
	if err != nil {
		return err
	}
	packagePath, err := filepath.Rel(root, out)
	if err != nil {
		return err
	}
	deps := []workspace.LinkablePackageDescriptor{{
		Path:       packagePath,
		Descriptor: ctx.PackageDescriptor,
	}}

	instructions, err := languagePlugin.Link(programInfo, deps, grpcServer.Addr())
	if err != nil {
		return fmt.Errorf("linking package: %w", err)
	}

	if ctx.Install {
		if err = pkgCmdUtil.InstallDependencies(languagePlugin, plugin.InstallDependenciesRequest{
			Info: programInfo,
		}); err != nil {
			return fmt.Errorf("installing dependencies: %w", err)
		}
	}

	fmt.Fprintln(ctx.Writer, instructions)
	return nil
}

// linkGoPackage links a locally generated SDK to an existing Go project.
func linkGoPackage(ctx *LinkPackageContext) error {
	fmt.Fprintf(ctx.Writer, "Successfully generated a Go SDK for the %s package at %s\n", ctx.Pkg.Name, ctx.Out)

	// All go code is placed under a relative package root so it is nested one
	// more directory deep equal to the package name.  This extra path is equal
	// to the paramaterization name if it is parameterized, else it is the same
	// as the base package name.
	//
	// (see pulumi-language-go  GeneratePackage for the pathPrefix).
	relOut, err := filepath.Rel(ctx.Root, ctx.Out)
	if err != nil {
		return err
	}
	if ctx.Pkg.Parameterization == nil {
		// Go SDK Gen replaces all "-" in the name.  See pkg/codegen/gen.go:goPackage
		name := strings.ReplaceAll(ctx.Pkg.Name, "-", "")
		relOut = filepath.Join(relOut, name)
	}
	if runtime.GOOS == "windows" {
		relOut = ".\\" + relOut
	} else {
		relOut = "./" + relOut
	}
	if _, err := os.Stat(relOut); err != nil {
		return fmt.Errorf("could not find sdk path %s: %w", relOut, err)
	}

	if err := ctx.Pkg.ImportLanguages(map[string]schema.Language{"go": go_gen.Importer}); err != nil {
		return err
	}
	goInfo, ok := ctx.Pkg.Language["go"].(go_gen.GoPackageInfo)
	if !ok {
		return errors.New("failed to import go language info")
	}

	gomodFilepath := filepath.Join(ctx.Root, "go.mod")
	gomodFileContent, err := os.ReadFile(gomodFilepath)
	if err != nil {
		return fmt.Errorf("cannot read mod file: %w", err)
	}

	gomod, err := modfile.Parse("go.mod", gomodFileContent, nil)
	if err != nil {
		return fmt.Errorf("mod parse: %w", err)
	}

	modulePath := goInfo.ModulePath
	if modulePath == "" {
		if goInfo.ImportBasePath != "" {
			modulePath = path.Dir(goInfo.ImportBasePath)
		}

		if modulePath == "" {
			modulePath = extractModulePath(ctx.Pkg.Reference())
		}
	}

	err = gomod.AddReplace(modulePath, "", relOut, "")
	if err != nil {
		return fmt.Errorf("could not add replace statement: %w", err)
	}

	b, err := gomod.Format()
	if err != nil {
		return fmt.Errorf("error formatting gomod: %w", err)
	}

	err = os.WriteFile(gomodFilepath, b, 0o600)
	if err != nil {
		return fmt.Errorf("error writing go.mod: %w", err)
	}

	fmt.Fprintf(ctx.Writer, "Go mod file updated to use local sdk for %s\n", ctx.Pkg.Name)
	// TODO: Also generate instructions using the default import path in cases where ImportBasePath is empty.
	// See https://github.com/pulumi/pulumi/issues/18410
	if goInfo.ImportBasePath != "" {
		fmt.Fprintf(ctx.Writer, "To use this package, import %s\n", goInfo.ImportBasePath)
	}

	return nil
}

// csharpPackageName converts a package name to a C#-friendly package name.
// for example "aws-api-gateway" becomes "AwsApiGateway".
func csharpPackageName(pkgName string) string {
	title := cases.Title(language.English)
	parts := strings.Split(pkgName, "-")
	for i, part := range parts {
		parts[i] = title.String(part)
	}
	return strings.Join(parts, "")
}

// linkDotnetPackage links a locally generated SDK to an existing .NET project.
// Also prints instructions for modifying the csproj file for DefaultItemExcludes cleanup.
func linkDotnetPackage(ctx *LinkPackageContext) error {
	fmt.Fprintf(ctx.Writer, "Successfully generated a .NET SDK for the %s package at %s\n", ctx.Pkg.Name, ctx.Out)
	fmt.Fprintf(ctx.Writer, "\n")

	relOut, err := filepath.Rel(ctx.Root, ctx.Out)
	if err != nil {
		return err
	}

	cmd := exec.Command("dotnet", "add", "reference", relOut)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dotnet error: %w", err)
	}

	namespace := "Pulumi"
	if ctx.Pkg.Namespace != "" {
		namespace = ctx.Pkg.Namespace
	}

	fmt.Fprintf(ctx.Writer, "You also need to add the following to your .csproj file of the program under section PropertyGroup:\n")
	fmt.Fprintf(ctx.Writer, "\n")
	fmt.Fprintf(ctx.Writer, "  <DefaultItemExcludes>$(DefaultItemExcludes);sdks/**/*.cs</DefaultItemExcludes>\n")
	fmt.Fprintf(ctx.Writer, "\n")
	fmt.Fprintf(ctx.Writer, "You can then use the SDK in your .NET code with:\n")
	fmt.Fprintf(ctx.Writer, "\n")
	fmt.Fprintf(ctx.Writer, "  using %s.%s;\n", csharpPackageName(namespace), csharpPackageName(ctx.Pkg.Name))
	fmt.Fprintf(ctx.Writer, "\n")
	return nil
}

// Prints instructions for linking a locally generated SDK to an existing Java
// project, in the absence of us attempting to perform this linking automatically.
func printJavaLinkInstructions(ctx *LinkPackageContext) error {
	fmt.Fprintf(ctx.Writer, "Successfully generated a Java SDK for the %s package at %s\n", ctx.Pkg.Name, ctx.Out)
	fmt.Fprintf(ctx.Writer, "\n")
	fmt.Fprintf(ctx.Writer, "To use this SDK in your Java project, complete the following steps:\n")
	fmt.Fprintf(ctx.Writer, "1. Copy the contents of the generated SDK to your Java project:\n")
	fmt.Fprintf(ctx.Writer, "     cp -r %s/src/* %s/src\n", ctx.Out, ctx.Root)
	fmt.Fprintf(ctx.Writer, "\n")
	fmt.Fprintf(ctx.Writer, "2. Add the SDK's dependencies to your Java project's build configuration.\n")
	fmt.Fprintf(ctx.Writer, "   If you are using Maven, add the following dependencies to your pom.xml:\n")
	fmt.Fprintf(ctx.Writer, "\n")
	fmt.Fprintf(ctx.Writer, "     <dependencies>\n")
	fmt.Fprintf(ctx.Writer, "         <dependency>\n")
	fmt.Fprintf(ctx.Writer, "             <groupId>com.google.code.findbugs</groupId>\n")
	fmt.Fprintf(ctx.Writer, "             <artifactId>jsr305</artifactId>\n")
	fmt.Fprintf(ctx.Writer, "             <version>3.0.2</version>\n")
	fmt.Fprintf(ctx.Writer, "         </dependency>\n")
	fmt.Fprintf(ctx.Writer, "         <dependency>\n")
	fmt.Fprintf(ctx.Writer, "             <groupId>com.google.code.gson</groupId>\n")
	fmt.Fprintf(ctx.Writer, "             <artifactId>gson</artifactId>\n")
	fmt.Fprintf(ctx.Writer, "             <version>2.8.9</version>\n")
	fmt.Fprintf(ctx.Writer, "         </dependency>\n")
	fmt.Fprintf(ctx.Writer, "     </dependencies>\n")
	fmt.Fprintf(ctx.Writer, "\n")
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

func extractModulePath(pkgRef schema.PackageReference) string {
	var vPath string
	version := pkgRef.Version()
	name := pkgRef.Name()
	if version != nil && version.Major > 1 {
		vPath = fmt.Sprintf("/v%d", version.Major)
	}

	// Default to example.com/pulumi-pkg if we have no other information.
	root := "example.com/pulumi-" + name
	// But if we have a publisher use that instead, assuming it's from github
	if pkgRef.Publisher() != "" {
		root = fmt.Sprintf("github.com/%s/pulumi-%s", pkgRef.Publisher(), name)
	}
	// And if we have a repository, use that instead of the publisher
	if pkgRef.Repository() != "" {
		url, err := url.Parse(pkgRef.Repository())
		if err == nil {
			// If there's any errors parsing the URL ignore it. Else use the host and path as go doesn't expect http://
			root = url.Host + url.Path
		}
	}

	// Support pack sdks write a go mod inside the go folder. Old legacy sdks would manually write a go.mod in the sdk
	// folder. This happened to mean that sdk/dotnet, sdk/nodejs etc where also considered part of the go sdk module.
	if pkgRef.SupportPack() {
		return fmt.Sprintf("%s/sdk/go%s", root, vPath)
	}

	return fmt.Sprintf("%s/sdk%s", root, vPath)
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
) (*schema.Package, *workspace.PackageSpec, error) {
	var spec schema.PackageSpec
	bind := func(
		spec schema.PackageSpec, specOverride *workspace.PackageSpec,
	) (*schema.Package, *workspace.PackageSpec, error) {
		pkg, diags, err := schema.BindSpec(spec, nil, schema.ValidationOptions{
			AllowDanglingReferences: true,
		})
		if err != nil {
			return nil, nil, err
		}
		if diags.HasErrors() {
			return nil, nil, diags
		}
		return pkg, specOverride, nil
	}
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
		return bind(spec, nil)
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
		return bind(spec, nil)
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
	return bind(spec, specOverride)
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
				depErr := descriptor.InstallDependencies(pctx.Base())
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
