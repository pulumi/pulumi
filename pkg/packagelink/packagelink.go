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

package packagelink

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/gensdk"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// LinkInto links a package into a project or plugin directory by generating an SDK and
// invoking the appropriate language plugin to link the generated SDK into the target
// project or plugin directory.
func LinkInto(
	ctx context.Context,
	pluginOrProject workspace.BaseProject, pluginOrProjectDir string,
	packageSpec *schema.PackageSpec,
	sink diag.Sink, host plugin.Host,
) error {
	pkg, err := bindSpec(*packageSpec)
	if err != nil {
		return fmt.Errorf("failed to bind schema: %w", err)
	}

	language := pluginOrProject.RuntimeInfo().Name()
	sdkDir, err := genSDK(ctx, pluginOrProjectDir, language, pkg, sink, host)
	if err != nil {
		return err
	}

	if language == "yaml" {
		return nil
	}

	return linkPackage(ctx, pluginOrProject, language, sdkDir, pkg, sink, host)
}

func genSDK(
	ctx context.Context,
	pluginOrProjectDir, pluginOrProjectLanguage string,
	pkg *schema.Package,
	sink diag.Sink, host plugin.Host,
) (string, error) {
	tempOut, err := os.MkdirTemp("", "pulumi-package-")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempOut)

	// We _always_ want SupportPack turned on for `package add`, this is an option on schemas because it can change
	// things like module paths for Go and we don't want every user using gen-sdk to be affected by that. But for
	// `package add` we know that this is just a local package and it's ok for module paths and similar to be different.
	pkg.SupportPack = true

	err = gensdk.GenSDK(ctx, pluginOrProjectDir, pluginOrProjectLanguage, tempOut, pkg,
		"",   /*overlays*/
		true, /*local*/
		sink, host)
	if err != nil {
		return "", fmt.Errorf("failed to generate SDK: %w", err)
	}

	out := filepath.Join(pluginOrProjectDir, "sdks")
	fmt.Printf("Successfully generated an SDK for the %s package at %s\n", pkg.Name, out)

	err = os.MkdirAll(out, 0o755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory for SDK: %w", err)
	}

	outName := pkg.Name
	if pkg.Namespace != "" {
		outName = pkg.Namespace + "-" + outName
	}
	out = filepath.Join(out, outName)

	// If directory already exists, remove it completely before copying new files
	if _, err := os.Stat(out); err == nil {
		if err := os.RemoveAll(out); err != nil {
			return "", fmt.Errorf("failed to clean existing SDK directory: %w", err)
		}
	}

	err = copyAll(out, filepath.Join(tempOut, pluginOrProjectLanguage))
	if err != nil {
		return "", fmt.Errorf("failed to move SDK to project: %w", err)
	}
	return out, nil
}

// linkPackage links a locally generated SDK into a project using `Language.Link`.
func linkPackage(
	ctx context.Context,
	pluginOrProject workspace.BaseProject, pluginOrProjectDir string,
	sdkDir string,
	pkgToLink *schema.Package,
	sink diag.Sink, host plugin.Host,
) error {
	root, err := filepath.Abs(pluginOrProjectDir)
	if err != nil {
		return err
	}
	languagePlugin, err := host.LanguageRuntime(pluginOrProject.RuntimeInfo().Name())
	if err != nil {
		return err
	}

	// Pre-load the schemas into the cached loader. This allows the loader to respond to GetSchema requests for file
	// based schemas.
	entries := make(map[string]schema.PackageReference, 1)
	entries[pkgToLink.Identity()] = pkgToLink.Reference()
	loader := schema.NewCachedLoaderWithEntries(schema.NewPluginLoader(host), entries)
	loaderServer := schema.NewLoaderServer(loader)
	pctx, err := plugin.NewContext(
		ctx, sink, sink, host, nil,
		pluginOrProjectDir, pluginOrProject.RuntimeInfo().Options(),
		false, nil)
	if err != nil {
		return err
	}
	grpcServer, err := plugin.NewServer(pctx, schema.LoaderRegistration(loaderServer))
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(grpcServer)

	pkgPath, err := filepath.Rel(root, sdkDir)
	if err != nil {
		return err
	}
	pkgDescriptor, err := pkgToLink.Descriptor(ctx)
	if err != nil {
		return err
	}

	deps := []workspace.LinkablePackageDescriptor{{
		Path:       pkgPath,
		Descriptor: pkgDescriptor,
	}}

	programInfo := plugin.NewProgramInfo(root, root, ".", pluginOrProject.RuntimeInfo().Options())
	instructions, err := languagePlugin.Link(programInfo, deps, grpcServer.Addr())
	if err != nil {
		return fmt.Errorf("linking package: %w", err)
	}

	sink.Infoerrf(&diag.Diag{Message: "%s"}, instructions)
	return nil
}

func bindSpec(spec schema.PackageSpec) (*schema.Package, error) {
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

// CopyAll copies src to dst. If src is a directory, its contents will be copied
// recursively.
func copyAll(dst string, src string) error {
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
			copyerr := copyAll(filepath.Join(dst, name), filepath.Join(src, name))
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
