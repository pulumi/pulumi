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

package gensdk

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	cmdDiag "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/diag"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func GenSDK(
	ctx context.Context,
	wd, language, out string,
	pkg *schema.Package, overlays string, local bool,
	sink diag.Sink, host plugin.Host,
) error {
	generatePackage := func(directory string, pkg *schema.Package, extraFiles map[string][]byte) error {
		// Ensure the target directory is clean, but created.
		err := os.RemoveAll(directory)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		err = os.MkdirAll(directory, 0o700)
		if err != nil {
			return err
		}

		jsonBytes, err := pkg.MarshalJSON()
		if err != nil {
			return err
		}

		pCtx, err := plugin.NewContext(ctx, sink, sink, host, nil, wd,
			nil,   /* runtimeOptions */
			false, /* disableProviderPreview */
			nil,   /* parentSpan */
		)
		if err != nil {
			return fmt.Errorf("create plugin context: %w", err)
		}
		defer contract.IgnoreClose(pCtx)
		languagePlugin, err := pCtx.Host.LanguageRuntime(language)
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
		cmdDiag.PrintDiagnostics(sink, diags)
		if err != nil {
			return err
		}

		if diags.HasErrors() {
			return fmt.Errorf("generation failed: %w", diags)
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
	return generatePackage(root, pkg, extraFiles)
}
