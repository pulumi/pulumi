package sdkgen

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	javagen "github.com/pulumi/pulumi-java/pkg/codegen/java"
	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func GenSDK(language, out string, pkg *schema.Package, overlays string, local bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current working directory: %w", err)
	}

	writeWrapper := func(
		generatePackage func(string, *schema.Package, map[string][]byte) (map[string][]byte, error),
	) func(string, *schema.Package, map[string][]byte) error {
		return func(directory string, p *schema.Package, extraFiles map[string][]byte) error {
			m, err := generatePackage("pulumi", p, extraFiles)
			if err != nil {
				return err
			}

			err = os.RemoveAll(directory)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			for k, v := range m {
				path := filepath.Join(directory, k)
				err := os.MkdirAll(filepath.Dir(path), 0o700)
				if err != nil {
					return err
				}
				err = os.WriteFile(path, v, 0o600)
				if err != nil {
					return err
				}
			}
			return nil
		}
	}

	var generatePackage func(string, *schema.Package, map[string][]byte) error
	switch language {
	case "dotnet":
		generatePackage = writeWrapper(func(t string, p *schema.Package, e map[string][]byte) (map[string][]byte, error) {
			return dotnet.GeneratePackage(t, p, e, nil)
		})
	case "java":
		generatePackage = writeWrapper(func(t string, p *schema.Package, e map[string][]byte) (map[string][]byte, error) {
			return javagen.GeneratePackage(t, p, e, local)
		})
	default:
		generatePackage = func(directory string, pkg *schema.Package, extraFiles map[string][]byte) error {
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

			pCtx, err := newPluginContext(cwd)
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
			printDiagnostics(pCtx.Diag, diags)
			if diags.HasErrors() {
				// If we've got error diagnostics then package generation failed, we've printed the error above so
				// just return a plain message here.
				return errors.New("generation failed")
			}

			return nil
		}
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

func LinkDependencies() {
	// TODO
}

func newPluginContext(cwd string) (*plugin.Context, error) {
	sink := diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{})
	pluginCtx, err := plugin.NewContext(sink, sink, nil, nil, cwd, nil, true, nil)
	if err != nil {
		return nil, err
	}
	return pluginCtx, nil
}

// printDiagnostics prints the diagnostics to the diagnostic sink
func printDiagnostics(sink diag.Sink, diagnostics hcl.Diagnostics) {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == hcl.DiagError {
			sink.Errorf(diag.Message("", "%s"), diagnostic)
		} else {
			sink.Warningf(diag.Message("", "%s"), diagnostic)
		}
	}
}
