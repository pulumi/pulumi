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

package packagecmd

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
	"slices"
	"strings"

	cmdDiag "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/diag"
	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	go_gen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/python/toolchain"
	"golang.org/x/mod/modfile"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v2"
)

func GenSDK(language, out string, pkg *schema.Package, overlays string, local bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current working directory: %w", err)
	}

	generatePackage := func(directory string, pkg *schema.Package, extraFiles map[string][]byte) error {
		// Ensure the target directory is clean, but created.
		err = os.RemoveAll(directory)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		err := os.MkdirAll(directory, 0o700)
		if err != nil {
			return err
		}

		jsonBytes, err := pkg.MarshalJSON()
		if err != nil {
			return err
		}

		pCtx, err := NewPluginContext(cwd)
		if err != nil {
			return fmt.Errorf("create plugin context: %w", err)
		}
		defer contract.IgnoreClose(pCtx.Host)
		programInfo := plugin.NewProgramInfo(cwd, cwd, ".", nil)
		languagePlugin, err := pCtx.Host.LanguageRuntime(language, programInfo)
		if err != nil {
			return err
		}

		loader := schema.NewPluginLoader(pCtx.Host)
		loaderServer := schema.NewLoaderServer(loader)
		grpcServer, err := plugin.NewServer(pCtx, schema.LoaderRegistration(loaderServer))
		if err != nil {
			return err
		}
		defer contract.IgnoreClose(grpcServer)

		diags, err := languagePlugin.GeneratePackage(directory, string(jsonBytes), extraFiles, grpcServer.Addr(), nil, local)
		if err != nil {
			return err
		}

		// These diagnostics come directly from the converter and so _should_ be user friendly. So we're just
		// going to print them.
		cmdDiag.PrintDiagnostics(pCtx.Diag, diags)
		if diags.HasErrors() {
			// If we've got error diagnostics then package generation failed, we've printed the error above so
			// just return a plain message here.
			return errors.New("generation failed")
		}

		return nil
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
			return fmt.Errorf("read overlay directory %q: %w", overlays, err)
		}
	}

	root := filepath.Join(out, language)
	err = generatePackage(root, pkg, extraFiles)
	if err != nil {
		return err
	}
	return nil
}

// LinkPackage links a locally generated SDK to an existing project.
// Currently Java is not supported and will print instructions for manual linking.
func LinkPackage(
	ws pkgWorkspace.Context, language string, root string, pkg *schema.Package, out string,
) error {
	switch language {
	case "nodejs":
		return linkNodeJsPackage(ws, root, pkg, out)
	case "python":
		return linkPythonPackage(ws, root, pkg, out)
	case "go":
		return linkGoPackage(root, pkg, out)
	case "dotnet":
		return linkDotnetPackage(root, pkg, out)
	case "java":
		return printJavaLinkInstructions(root, pkg, out)
	default:
		break
	}
	return nil
}

func getNodeJSPkgName(pkg *schema.Package) string {
	if info, ok := pkg.Language["nodejs"].(nodejs.NodePackageInfo); ok && info.PackageName != "" {
		return info.PackageName
	}

	if pkg.Namespace != "" {
		return "@" + pkg.Namespace + "/" + pkg.Name
	}
	return "@pulumi/" + pkg.Name
}

// linkNodeJsPackage links a locally generated SDK to an existing Node.js project.
func linkNodeJsPackage(ws pkgWorkspace.Context, root string, pkg *schema.Package, out string) error {
	fmt.Printf("Successfully generated a Nodejs SDK for the %s package at %s\n", pkg.Name, out)
	proj, _, err := ws.ReadProject()
	if err != nil {
		return err
	}
	relOut, err := filepath.Rel(root, out)
	if err != nil {
		return err
	}
	packageSpecifier := fmt.Sprintf("%s@file:%s", getNodeJSPkgName(pkg), relOut)
	var addCmd *exec.Cmd
	options := proj.Runtime.Options()
	if packagemanager, ok := options["packagemanager"]; ok {
		if pm, ok := packagemanager.(string); ok {
			switch pm {
			case "npm":
				fallthrough
			case "yarn":
				fallthrough
			case "pnpm":
				addCmd = exec.Command(pm, "add", packageSpecifier)
			default:
				return fmt.Errorf("unsupported package manager: %s", pm)
			}
		} else {
			return fmt.Errorf("package manager option must be a string: %v", packagemanager)
		}
	} else {
		// Assume npm if no packagemanager is specified
		addCmd = exec.Command("npm", "add", packageSpecifier)
	}

	addCmd.Stdout = os.Stdout
	addCmd.Stderr = os.Stderr
	err = addCmd.Run()
	if err != nil {
		return fmt.Errorf("error executing node package manager command %s: %w", addCmd.String(), err)
	}

	return printNodeJsImportInstructions(os.Stdout, pkg, options)
}

// printNodeJsImportInstructions prints instructions for importing the NodeJS SDK to the specified writer.
func printNodeJsImportInstructions(w io.Writer, pkg *schema.Package, options map[string]interface{}) error {
	importName := cgstrings.Camel(pkg.Name)

	useTypescript := true
	if typescript, ok := options["typescript"]; ok {
		if val, ok := typescript.(bool); ok {
			useTypescript = val
		}
	}
	if useTypescript {
		fmt.Fprintln(w, "You can then import the SDK in your TypeScript code with:")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  import * as %s from \"%s\";\n", importName, getNodeJSPkgName(pkg))
	} else {
		fmt.Fprintln(w, "You can then import the SDK in your Javascript code with:")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  const %s = require(\"%s\");\n", importName, getNodeJSPkgName(pkg))
	}
	fmt.Fprintln(w)
	return nil
}

// linkPythonPackage links a locally generated SDK to an existing Python project.
func linkPythonPackage(ws pkgWorkspace.Context, root string, pkg *schema.Package, out string) error {
	fmt.Printf("Successfully generated a Python SDK for the %s package at %s\n", pkg.Name, out)
	fmt.Println()
	proj, _, err := ws.ReadProject()
	if err != nil {
		return err
	}
	packageSpecifier, err := filepath.Rel(root, out)
	if err != nil {
		return err
	}

	modifyRequirements := func(virtualenv string) error {
		fPath := filepath.Join(root, "requirements.txt")
		fBytes, err := os.ReadFile(fPath)
		if err != nil {
			return fmt.Errorf("error opening requirments.txt: %w", err)
		}

		lines := regexp.MustCompile("\r?\n").Split(string(fBytes), -1)
		if !slices.Contains(lines, packageSpecifier) {
			// Match the file's line endings when adding the package specifier.
			usesCRLF := strings.Contains(string(fBytes), "\r\n")
			lineEnding := "\n"
			if usesCRLF {
				lineEnding = "\r\n"
			}
			fBytes = []byte(packageSpecifier + lineEnding + string(fBytes))
			err = os.WriteFile(fPath, fBytes, 0o600)
			if err != nil {
				return fmt.Errorf("could not write requirements.txt: %w", err)
			}
		}

		tc, err := toolchain.ResolveToolchain(toolchain.PythonOptions{
			Root:       root,
			Virtualenv: virtualenv,
		})
		if err != nil {
			return fmt.Errorf("error resolving toolchain: %w", err)
		}
		if virtualenv != "" {
			if err := tc.EnsureVenv(context.TODO(), root, false, /* useLanguageVersionTools */
				true /*showOutput*/, os.Stdout, os.Stderr); err != nil {
				return fmt.Errorf("error ensuring virtualenv is setup: %w", err)
			}
		}
		cmd, err := tc.ModuleCommand(context.TODO(), "pip", "install", "-r", fPath)
		if err != nil {
			return fmt.Errorf("error preparing pip install command: %w", err)
		}
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("error running %s: %w", cmd.String(), err)
		}

		return nil
	}
	options := proj.Runtime.Options()
	if toolchain, ok := options["toolchain"]; ok {
		if tc, ok := toolchain.(string); ok {
			var depAddCmd *exec.Cmd
			switch tc {
			case "pip":
				virtualenv, ok := options["virtualenv"]
				if !ok {
					return errors.New("virtualenv option is required")
				}
				if virtualenv, ok := virtualenv.(string); ok {
					if err := modifyRequirements(virtualenv); err != nil {
						return err
					}
				} else {
					return errors.New("virtualenv option must be a string")
				}
			case "poetry":
				depAddCmd = exec.Command("poetry", "add", packageSpecifier)
			case "uv":
				depAddCmd = exec.Command("uv", "add", packageSpecifier)
			default:
				return fmt.Errorf("unsupported package manager: %s", tc)
			}

			if depAddCmd != nil {
				depAddCmd.Stderr = os.Stderr
				depAddCmd.Stdout = os.Stdout
				err = depAddCmd.Run()
				if err != nil {
					return fmt.Errorf("error running %s: %w", depAddCmd.String(), err)
				}
			}
		} else {
			return fmt.Errorf("packagemanager option must be a string: %v", toolchain)
		}
	} else {
		// Assume pip if no packagemanager is specified
		if err := modifyRequirements(""); err != nil {
			return err
		}
	}

	pyInfo, ok := pkg.Language["python"].(python.PackageInfo)
	var importName string
	var packageName string
	if ok && pyInfo.PackageName != "" {
		importName = pyInfo.PackageName
		packageName = pyInfo.PackageName
	} else {
		importName = strings.ReplaceAll(pkg.Name, "-", "_")
	}

	if packageName == "" {
		packageName = python.PyPack(pkg.Namespace, pkg.Name)
	}

	fmt.Println()
	fmt.Println("You can then import the SDK in your Python code with:")
	fmt.Println()
	fmt.Printf("  import %s as %s\n", packageName, importName)
	fmt.Println()
	return nil
}

// linkGoPackage links a locally generated SDK to an existing Go project.
func linkGoPackage(root string, pkg *schema.Package, out string) error {
	fmt.Printf("Successfully generated a Go SDK for the %s package at %s\n", pkg.Name, out)

	// All go code is placed under a relative package root so it is nested one
	// more directory deep equal to the package name.  This extra path is equal
	// to the paramaterization name if it is parameterized, else it is the same
	// as the base package name.
	//
	// (see pulumi-language-go  GeneratePackage for the pathPrefix).
	relOut, err := filepath.Rel(root, out)
	if err != nil {
		return err
	}
	if pkg.Parameterization == nil {
		// Go SDK Gen replaces all "-" in the name.  See pkg/codegen/gen.go:goPackage
		name := strings.ReplaceAll(pkg.Name, "-", "")
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

	if err := pkg.ImportLanguages(map[string]schema.Language{"go": go_gen.Importer}); err != nil {
		return err
	}
	goInfo, ok := pkg.Language["go"].(go_gen.GoPackageInfo)
	if !ok {
		return errors.New("failed to import go language info")
	}

	gomodFilepath := filepath.Join(root, "go.mod")
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
			modulePath = extractModulePath(pkg.Reference())
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

	fmt.Printf("Go mod file updated to use local sdk for %s\n", pkg.Name)
	// TODO: Also generate instructions using the default import path in cases where ImportBasePath is empty.
	// See https://github.com/pulumi/pulumi/issues/18410
	if goInfo.ImportBasePath != "" {
		fmt.Printf("To use this package, import %s\n", goInfo.ImportBasePath)
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
func linkDotnetPackage(root string, pkg *schema.Package, out string) error {
	fmt.Printf("Successfully generated a .NET SDK for the %s package at %s\n", pkg.Name, out)
	fmt.Println()

	relOut, err := filepath.Rel(root, out)
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
	if pkg.Namespace != "" {
		namespace = pkg.Namespace
	}

	fmt.Printf("You also need to add the following to your .csproj file of the program:\n")
	fmt.Println()
	fmt.Println("  <DefaultItemExcludes>$(DefaultItemExcludes);sdks/**/*.cs</DefaultItemExcludes>")
	fmt.Println()
	fmt.Println("You can then use the SDK in your .NET code with:")
	fmt.Println()
	fmt.Printf("  using %s.%s;\n", csharpPackageName(namespace), csharpPackageName(pkg.Name))
	fmt.Println()
	return nil
}

// Prints instructions for linking a locally generated SDK to an existing Java
// project, in the absence of us attempting to perform this linking automatically.
func printJavaLinkInstructions(root string, pkg *schema.Package, out string) error {
	fmt.Printf("Successfully generated a Java SDK for the %s package at %s\n", pkg.Name, out)
	fmt.Println()
	fmt.Println("To use this SDK in your Java project, complete the following steps:")
	fmt.Println()
	fmt.Println("1. Copy the contents of the generated SDK to your Java project:")
	fmt.Printf("     cp -r %s/src/* %s/src\n", out, root)
	fmt.Println()
	fmt.Println("2. Add the SDK's dependencies to your Java project's build configuration.")
	fmt.Println("   If you are using Maven, add the following dependencies to your pom.xml:")
	fmt.Println()
	fmt.Println("     <dependencies>")
	fmt.Println("         <dependency>")
	fmt.Println("             <groupId>com.google.code.findbugs</groupId>")
	fmt.Println("             <artifactId>jsr305</artifactId>")
	fmt.Println("             <version>3.0.2</version>")
	fmt.Println("         </dependency>")
	fmt.Println("         <dependency>")
	fmt.Println("             <groupId>com.google.code.gson</groupId>")
	fmt.Println("             <artifactId>gson</artifactId>")
	fmt.Println("             <version>2.8.9</version>")
	fmt.Println("         </dependency>")
	fmt.Println("     </dependencies>")
	fmt.Println()
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
	pluginCtx, err := plugin.NewContext(sink, sink, nil, nil, cwd, nil, true, nil)
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
			spec.Namespace = matches[1]
		}
	}
}

// SchemaFromSchemaSource takes a schema source and returns its associated schema. A
// schema source is either a file (ending with .[json|y[a]ml]) or a plugin with an
// optional version:
//
//	FILE.[json|y[a]ml] | PLUGIN[@VERSION] | PATH_TO_PLUGIN
func SchemaFromSchemaSource(pctx *plugin.Context, packageSource string, args []string) (*schema.Package, error) {
	var spec schema.PackageSpec
	bind := func(spec schema.PackageSpec) (*schema.Package, error) {
		pkg, diags, err := schema.BindSpec(spec, nil, false)
		if err != nil {
			return nil, err
		}
		if diags.HasErrors() {
			return nil, diags
		}
		return pkg, nil
	}
	if ext := filepath.Ext(packageSource); ext == ".yaml" || ext == ".yml" {
		if len(args) > 0 {
			return nil, errors.New("parameterization arguments are not supported for yaml files")
		}
		f, err := os.ReadFile(packageSource)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(f, &spec)
		if err != nil {
			return nil, err
		}
		return bind(spec)
	} else if ext == ".json" {
		if len(args) > 0 {
			return nil, errors.New("parameterization arguments are not supported for json files")
		}

		f, err := os.ReadFile(packageSource)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(f, &spec)
		if err != nil {
			return nil, err
		}
		return bind(spec)
	}

	p, err := ProviderFromSource(pctx, packageSource)
	if err != nil {
		return nil, err
	}
	defer p.Close()

	var request plugin.GetSchemaRequest
	if len(args) > 0 {
		resp, err := p.Parameterize(pctx.Request(), plugin.ParameterizeRequest{
			Parameters: &plugin.ParameterizeArgs{Args: args},
		})
		if err != nil {
			return nil, fmt.Errorf("parameterize: %w", err)
		}

		request = plugin.GetSchemaRequest{
			SubpackageName:    resp.Name,
			SubpackageVersion: &resp.Version,
		}
	}

	schema, err := p.GetSchema(pctx.Request(), request)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(schema.Schema, &spec)
	if err != nil {
		return nil, err
	}
	pluginSpec, err := workspace.NewPluginSpec(packageSource, apitype.ResourcePlugin, nil, "", nil)
	if err != nil {
		return nil, err
	}
	if pluginSpec.PluginDownloadURL != "" {
		spec.PluginDownloadURL = pluginSpec.PluginDownloadURL
	}
	setSpecNamespace(&spec, pluginSpec)
	return bind(spec)
}

func SchemaFromSchemaSourceValueArgs(
	pctx *plugin.Context,
	packageSource string,
	parameterizationValue []byte,
) (*schema.Package, error) {
	var spec schema.PackageSpec
	bind := func(spec schema.PackageSpec) (*schema.Package, error) {
		pkg, diags, err := schema.BindSpec(spec, nil, false)
		if err != nil {
			return nil, err
		}
		if diags.HasErrors() {
			return nil, diags
		}
		return pkg, nil
	}

	p, err := ProviderFromSource(pctx, packageSource)
	if err != nil {
		return nil, err
	}
	defer p.Close()

	var request plugin.GetSchemaRequest
	if parameterizationValue != nil {
		resp, err := p.Parameterize(pctx.Request(), plugin.ParameterizeRequest{
			Parameters: &plugin.ParameterizeValue{Value: parameterizationValue},
		})
		if err != nil {
			return nil, fmt.Errorf("parameterize: %w", err)
		}

		request = plugin.GetSchemaRequest{
			SubpackageName:    resp.Name,
			SubpackageVersion: &resp.Version,
		}
	}

	schema, err := p.GetSchema(pctx.Request(), request)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(schema.Schema, &spec)
	if err != nil {
		return nil, err
	}
	return bind(spec)
}

// ProviderFromSource takes a plugin name or path.
//
// PLUGIN[@VERSION] | PATH_TO_PLUGIN
func ProviderFromSource(pctx *plugin.Context, packageSource string) (plugin.Provider, error) {
	pluginSpec, err := workspace.NewPluginSpec(packageSource, apitype.ResourcePlugin, nil, "", nil)
	if err != nil {
		return nil, err
	}
	descriptor := workspace.PackageDescriptor{
		PluginSpec: pluginSpec,
	}

	isExecutable := func(info fs.FileInfo) bool {
		// Windows doesn't have executable bits to check
		if runtime.GOOS == "windows" {
			return !info.IsDir()
		}
		return info.Mode()&0o111 != 0 && !info.IsDir()
	}

	// For all plugins except for file paths, we try to load it from the provider host.
	//
	// Note that if a local folder has a name that could match an downloadable plugin, we
	// prefer the downloadable plugin.  The user can disambiguate by prepending './' to the
	// name.
	if !plugin.IsLocalPluginPath(packageSource) {
		host, err := plugin.NewDefaultHost(pctx, nil, false, nil, nil, nil, nil, "")
		if err != nil {
			return nil, err
		}
		// We assume this was a plugin and not a path, so load the plugin.
		provider, err := host.Provider(descriptor)
		if err != nil {
			// There is an executable or directory with the same name, so suggest that
			if info, statErr := os.Stat(descriptor.Name); statErr == nil && (isExecutable(info) || info.IsDir()) {
				return nil, fmt.Errorf("could not find installed plugin %s, did you mean ./%[1]s: %w", descriptor.Name, err)
			}

			if descriptor.SubDir() != "" {
				path, err := descriptor.DirPath()
				if err != nil {
					return nil, err
				}
				info, statErr := os.Stat(filepath.Join(path, descriptor.SubDir()))
				if statErr == nil && info.IsDir() {
					// The plugin is already installed.  But since it is in a subdirectory, it could be that
					// we previously installed a plugin in a different subdirectory of the same repository.
					// This is why the provider might have failed to start up.  Install the dependencies
					// and try again.
					depErr := descriptor.PluginSpec.InstallDependencies(pctx.Base())
					if depErr != nil {
						return nil, fmt.Errorf("installing plugin dependencies: %w", depErr)
					}
					return host.Provider(descriptor)
				}
			}

			// Try and install the plugin if it was missing and try again, unless auto plugin installs are turned off.
			var missingError *workspace.MissingError
			if !errors.As(err, &missingError) || env.DisableAutomaticPluginAcquisition.Value() {
				return nil, err
			}

			log := func(sev diag.Severity, msg string) {
				host.Log(sev, "", msg, 0)
			}

			_, err = pkgWorkspace.InstallPlugin(pctx.Base(), descriptor.PluginSpec, log)
			if err != nil {
				return nil, err
			}

			p, err := host.Provider(descriptor)
			if err != nil {
				return nil, err
			}

			return p, nil
		}
		return provider, nil
	}

	// We were given a path to a binary or folder, so invoke that.
	info, err := os.Stat(packageSource)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("could not find file %s", packageSource)
	} else if err != nil {
		return nil, err
	} else if info.IsDir() {
		// If it's a directory we need to add a fake provider binary to the path because that's what NewProviderFromPath
		// expects.
		packageSource = filepath.Join(packageSource, "pulumi-resource-"+info.Name())
	} else {
		if !isExecutable(info) {
			if p, err := filepath.Abs(packageSource); err == nil {
				packageSource = p
			}
			return nil, fmt.Errorf("plugin at path %q not executable", packageSource)
		}
	}

	p, err := plugin.NewProviderFromPath(pctx.Host, pctx, packageSource)
	if err != nil {
		return nil, err
	}
	return p, nil
}
