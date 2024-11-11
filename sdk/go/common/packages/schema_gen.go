package packages

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/blang/semver"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// ProviderFromSource takes a plugin name or path.
//
// PLUGIN[@VERSION] | PATH_TO_PLUGIN
func ProviderFromSource(packageSource string) (plugin.Provider, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	sink := cmdutil.Diag()
	pCtx, err := plugin.NewContext(sink, sink, nil, nil, wd, nil, false, nil)
	if err != nil {
		return nil, err
	}

	descriptor := workspace.PackageDescriptor{
		PluginSpec: workspace.PluginSpec{
			Kind: apitype.ResourcePlugin,
			Name: packageSource,
		},
	}

	if s := strings.SplitN(packageSource, "@", 2); len(s) == 2 {
		descriptor.Name = s[0]
		v, err := semver.ParseTolerant(s[1])
		if err != nil {
			return nil, fmt.Errorf("VERSION must be valid semver: %w", err)
		}
		descriptor.Version = &v
	}

	isExecutable := func(info fs.FileInfo) bool {
		// Windows doesn't have executable bits to check
		if runtime.GOOS == "windows" {
			return !info.IsDir()
		}
		return info.Mode()&0o111 != 0 && !info.IsDir()
	}

	// No file separators, so we try to look up the schema
	// On unix, these checks are identical. On windows, filepath.Separator is '\\'
	if !strings.ContainsRune(descriptor.Name, filepath.Separator) && !strings.ContainsRune(descriptor.Name, '/') {
		host, err := plugin.NewDefaultHost(pCtx, nil, false, nil, nil, nil, "")
		if err != nil {
			return nil, err
		}
		// We assume this was a plugin and not a path, so load the plugin.
		provider, err := host.Provider(descriptor)
		if err != nil {
			// There is an executable with the same name, so suggest that
			if info, statErr := os.Stat(descriptor.Name); statErr == nil && isExecutable(info) {
				return nil, fmt.Errorf("could not find installed plugin %s, did you mean ./%[1]s: %w", descriptor.Name, err)
			}

			// Try and install the plugin if it was missing and try again, unless auto plugin installs are turned off.
			var missingError *workspace.MissingError
			if !errors.As(err, &missingError) || env.DisableAutomaticPluginAcquisition.Value() {
				return nil, err
			}

			log := func(sev diag.Severity, msg string) {
				host.Log(sev, "", msg, 0)
			}

			_, err = pkgWorkspace.InstallPlugin(descriptor.PluginSpec, log)
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

	host, err := plugin.NewDefaultHost(pCtx, nil, false, nil, nil, nil, "")
	if err != nil {
		return nil, err
	}

	p, err := plugin.NewProviderFromPath(host, pCtx, packageSource)
	if err != nil {
		return nil, err
	}
	return p, nil
}
