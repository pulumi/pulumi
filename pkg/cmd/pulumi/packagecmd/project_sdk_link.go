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

	"github.com/blang/semver"
	cmdDiag "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/diag"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	go_gen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
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

type LinkPackageContext struct {
	// The workspace context for the project to which the SDK package is being linked.
	Workspace pkgWorkspace.Context
	// The programming language of the SDK package being linked.
	Language string
	// The root directory of the project to which the SDK package is being linked.
	Root string
	// The schema of the Pulumi package from which the SDK being linked was generated.
	Pkg *schema.Package
	// The output directory where the SDK package to be linked is located.
	Out string
	// True if the linked SDK package should be installed into the project it is being added to. If this is false, the
	// package will be linked (e.g. an entry added to package.json), but not installed (e.g. its contents unpacked into
	// node_modules).
	Install bool
}

// LinkPackage links a locally generated SDK to an existing project.
// Currently Java is not supported and will print instructions for manual linking.
func LinkPackage(ctx *LinkPackageContext) error {
	switch ctx.Language {
	case "nodejs":
		return linkNodeJsPackage(ctx)
	case "python":
		return linkPythonPackage(ctx)
	case "go":
		return linkGoPackage(ctx)
	case "dotnet":
		return linkDotnetPackage(ctx)
	case "java":
		return printJavaLinkInstructions(ctx)
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
func linkNodeJsPackage(ctx *LinkPackageContext) error {
	fmt.Printf("Successfully generated a Nodejs SDK for the %s package at %s\n", ctx.Pkg.Name, ctx.Out)
	proj, _, err := ctx.Workspace.ReadProject()
	if err != nil {
		return err
	}
	relOut, err := filepath.Rel(ctx.Root, ctx.Out)
	if err != nil {
		return err
	}

	// Depending on whether we want to install the linked package, we have to pick one of two paths:
	//
	// * For cases where we do want to install linked SDKs (where ctx.Install is true), we can use the typical `npm add`,
	//   `pnpm add` commands, etc. These will take care of both modifying the package.json file and installing the SDK
	//   into node_modules.
	// * For cases where we do not want to install linked SDKs (where ctx.Install is false), we only want to modify the
	//   package.json file. In this case, we can use the `pkg set` commands that many package managers support.
	var addCmd *exec.Cmd
	options := proj.Runtime.Options()

	if ctx.Install {
		// Installing -- use the `add` commands.

		packageName := getNodeJSPkgName(ctx.Pkg)
		packageSpecifier := fmt.Sprintf("%s@file:%s", packageName, relOut)
		if packagemanager, ok := options["packagemanager"]; ok {
			if pm, ok := packagemanager.(string); ok {
				switch pm {
				case "bun":
					addCmd = exec.Command(pm, "add", packageSpecifier, "--trust")
				case "npm":
					fallthrough
				case "yarn":
					addCmd = exec.Command(pm, "add", packageSpecifier)
				case "pnpm":
					// pnpm does not run postinstall scripts by default. We need
					// to run the generated postinstall script for the SDK to
					// compile it, See `genPostInstallScript`.
					addCmd = exec.Command(pm, "add", packageSpecifier, "--allow-build="+packageName)
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
	} else {
		// Not installing -- use the `pkg set` commands.

		// `pkg set` lets us directly modify the package.json file. Since we want to set an entry in the `dependencies`
		// section, we'll pass it a string of the form "dependencies.<packageName>=file:<path-to-package>".
		packageSpecifier := fmt.Sprintf("dependencies.%s=file:%s", getNodeJSPkgName(ctx.Pkg), relOut)
		if packagemanager, ok := options["packagemanager"]; ok {
			if pm, ok := packagemanager.(string); ok {
				switch pm {
				case "bun":
					addCmd = exec.Command(pm, "pm", "pkg", "set", packageSpecifier)
				case "npm":
					fallthrough
				case "pnpm":
					addCmd = exec.Command(pm, "pkg", "set", packageSpecifier)
				case "yarn":
					// Yarn doesn't have a `pkg` command. Currently, however, we only support Yarn Classic, for which the
					// recommended install method is through `npm`. Consequently, we can use `npm pkg set` for Yarn as well, since
					// this will only modify the package.json file and not actually perform any dependency management.
					addCmd = exec.Command("npm", "pkg", "set", packageSpecifier)
				default:
					return fmt.Errorf("unsupported package manager: %s", pm)
				}
			} else {
				return fmt.Errorf("package manager option must be a string: %v", packagemanager)
			}
		} else {
			// Assume npm if no packagemanager is specified
			addCmd = exec.Command("npm", "pkg", "set", packageSpecifier)
		}
	}

	addCmd.Stdout = os.Stdout
	addCmd.Stderr = os.Stderr
	err = addCmd.Run()
	if err != nil {
		return fmt.Errorf("error executing node package manager command %s: %w", addCmd.String(), err)
	}

	return printNodeJsImportInstructions(os.Stdout, ctx.Pkg, options)
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
func linkPythonPackage(ctx *LinkPackageContext) error {
	fmt.Printf("Successfully generated a Python SDK for the %s package at %s\n", ctx.Pkg.Name, ctx.Out)
	fmt.Println()
	proj, _, err := ctx.Workspace.ReadProject()
	if err != nil {
		return err
	}
	packageSpecifier, err := filepath.Rel(ctx.Root, ctx.Out)
	if err != nil {
		return err
	}

	modifyRequirements := func(virtualenv string) error {
		fPath := filepath.Join(ctx.Root, "requirements.txt")
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
			Root:       ctx.Root,
			Virtualenv: virtualenv,
		})
		if err != nil {
			return fmt.Errorf("error resolving toolchain: %w", err)
		}
		if virtualenv != "" {
			if err := tc.EnsureVenv(context.TODO(), ctx.Root, false, /* useLanguageVersionTools */
				true /*showOutput*/, os.Stdout, os.Stderr); err != nil {
				return fmt.Errorf("error ensuring virtualenv is setup: %w", err)
			}
		}

		if ctx.Install {
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
		}

		return nil
	}
	options := proj.Runtime.Options()
	if toolchainOpt, ok := options["toolchain"]; ok {
		if tc, ok := toolchainOpt.(string); ok {
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
				args := []string{"add"}
				if !ctx.Install {
					args = append(args, "--lock")
				}

				args = append(args, packageSpecifier)
				depAddCmd = exec.Command("poetry", args...)
			case "uv":
				args := []string{"add"}

				// Starting with version 0.8.0, uv will automatically add
				// packages in subdirectories as workspace members. However the
				// generated SDK might not have a `pyproject.toml`, which is
				// required for uv workspace members. To add the generated SDK
				// as a normal dependency, we can run `uv add --no-workspace`,
				// but this flag is only available on version 0.8.0 and up.
				cmd := exec.Command("uv", "--version")
				versionString, err := cmd.Output()
				if err != nil {
					return fmt.Errorf("failed to get uv version: %w", err)
				}
				version, err := toolchain.ParseUvVersion(string(versionString))
				if err != nil {
					return err
				}
				if version.GE(semver.MustParse("0.8.0")) {
					args = append(args, "--no-workspace")
				}

				if !ctx.Install {
					args = append(args, "--no-sync")
				}

				args = append(args, packageSpecifier)
				depAddCmd = exec.Command("uv", args...)
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
			return fmt.Errorf("packagemanager option must be a string: %v", toolchainOpt)
		}
	} else {
		// Assume pip if no packagemanager is specified
		if err := modifyRequirements(""); err != nil {
			return err
		}
	}

	pyInfo, ok := ctx.Pkg.Language["python"].(python.PackageInfo)
	var importName string
	var packageName string
	if ok && pyInfo.PackageName != "" {
		importName = pyInfo.PackageName
		packageName = pyInfo.PackageName
	} else {
		importName = strings.ReplaceAll(ctx.Pkg.Name, "-", "_")
	}

	if packageName == "" {
		packageName = python.PyPack(ctx.Pkg.Namespace, ctx.Pkg.Name)
	}

	fmt.Println()
	fmt.Println("You can then import the SDK in your Python code with:")
	fmt.Println()
	fmt.Printf("  import %s as %s\n", packageName, importName)
	fmt.Println()
	return nil
}

// linkGoPackage links a locally generated SDK to an existing Go project.
func linkGoPackage(ctx *LinkPackageContext) error {
	fmt.Printf("Successfully generated a Go SDK for the %s package at %s\n", ctx.Pkg.Name, ctx.Out)

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

	fmt.Printf("Go mod file updated to use local sdk for %s\n", ctx.Pkg.Name)
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
func linkDotnetPackage(ctx *LinkPackageContext) error {
	fmt.Printf("Successfully generated a .NET SDK for the %s package at %s\n", ctx.Pkg.Name, ctx.Out)
	fmt.Println()

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

	fmt.Printf("You also need to add the following to your .csproj file of the program:\n")
	fmt.Println()
	fmt.Println("  <DefaultItemExcludes>$(DefaultItemExcludes);sdks/**/*.cs</DefaultItemExcludes>")
	fmt.Println()
	fmt.Println("You can then use the SDK in your .NET code with:")
	fmt.Println()
	fmt.Printf("  using %s.%s;\n", csharpPackageName(namespace), csharpPackageName(ctx.Pkg.Name))
	fmt.Println()
	return nil
}

// Prints instructions for linking a locally generated SDK to an existing Java
// project, in the absence of us attempting to perform this linking automatically.
func printJavaLinkInstructions(ctx *LinkPackageContext) error {
	fmt.Printf("Successfully generated a Java SDK for the %s package at %s\n", ctx.Pkg.Name, ctx.Out)
	fmt.Println()
	fmt.Println("To use this SDK in your Java project, complete the following steps:")
	fmt.Println()
	fmt.Println("1. Copy the contents of the generated SDK to your Java project:")
	fmt.Printf("     cp -r %s/src/* %s/src\n", ctx.Out, ctx.Root)
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
		pluginSpec,
		packageresolution.Options{DisableRegistryResolve: env.DisableRegistryResolve.Value()},
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
	case packageresolution.ExternalSourceResult:
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
