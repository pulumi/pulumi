// Copyright 2016-2023, Pulumi Corporation.
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

package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/pkg/v3/util"
	"github.com/pulumi/pulumi/pkg/v3/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	diagutil "github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// InstallPluginError is returned by InstallPlugin if we couldn't install the plugin
type InstallPluginError struct {
	// The specification of the plugin to install
	Spec workspace.PluginDescriptor
	// The underlying error that occurred during the download or install.
	Err error
}

func (err *InstallPluginError) Error() string {
	var server string
	if err.Spec.PluginDownloadURL != "" {
		server = " --server " + err.Spec.PluginDownloadURL
	}

	if err.Spec.Version != nil {
		return fmt.Sprintf("Could not automatically download and install %[1]s plugin 'pulumi-%[1]s-%[2]s'"+
			" at version v%[3]s"+
			", install the plugin using `pulumi plugin install %[1]s %[2]s v%[3]s%[4]s`: %[5]v",
			err.Spec.Kind, err.Spec.Name, err.Spec.Version, server, err.Err)
	}

	return fmt.Sprintf("Could not automatically download and install %[1]s plugin 'pulumi-%[1]s-%[2]s'"+
		", install the plugin using `pulumi plugin install %[1]s %[2]s%[3]s`: %[4]v",
		err.Spec.Kind, err.Spec.Name, server, err.Err)
}

func (err *InstallPluginError) Unwrap() error {
	return err.Err
}

func InstallPlugin(ctx context.Context, pluginSpec workspace.PluginDescriptor,
	log func(sev diag.Severity, msg string),
) (*semver.Version, error) {
	util.SetKnownPluginDownloadURL(&pluginSpec)
	util.SetKnownPluginVersion(&pluginSpec)
	if pluginSpec.Version == nil {
		var err error
		pluginSpec.Version, err = pluginSpec.GetLatestVersion(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not find latest version for provider %s: %w", pluginSpec.Name, err)
		}
	}

	wrapper := func(stream io.ReadCloser, size int64) io.ReadCloser {
		// Log at info but to stderr so we don't pollute stdout for commands like `package get-schema`
		log(diag.Infoerr, "Downloading provider: "+pluginSpec.Name)
		return stream
	}

	retry := func(err error, attempt int, limit int, delay time.Duration) {
		log(diag.Warning, fmt.Sprintf("error downloading provider: %s\n"+
			"Will retry in %v [%d/%d]", err, delay, attempt, limit))
	}

	logging.V(1).Infof("Automatically downloading provider %s", pluginSpec.Name)
	downloadedFile, err := workspace.DownloadToFile(ctx, pluginSpec, wrapper, retry)
	if err != nil {
		return nil, &InstallPluginError{
			Spec: pluginSpec,
			Err:  fmt.Errorf("error downloading provider %s to file: %w", pluginSpec.Name, err),
		}
	}

	logging.V(1).Infof("Automatically installing provider %s", pluginSpec.Name)
	err = InstallPluginContent(ctx, pluginSpec, pluginstorage.TarPlugin(downloadedFile), false)
	if err != nil {
		return nil, &InstallPluginError{
			Spec: pluginSpec,
			Err:  fmt.Errorf("error installing provider %s: %w", pluginSpec.Name, err),
		}
	}

	return pluginSpec.Version, nil
}

// InstallPluginContent installs a plugin's tarball into the cache, then installs it's
// dependencies.
//
// TODO[https://github.com/pulumi/pulumi/issues/21005]: This function is known to be wrong
// when a plugin needs it's dependencies to be installed before it can safely be
// installed.
func InstallPluginContent(
	ctx context.Context, spec workspace.PluginDescriptor, content pluginstorage.Content, reinstall bool,
) (err error) {
	done, err := pluginstorage.UnpackContents(ctx, spec, content, reinstall)
	if err != nil {
		return err
	}
	defer func() { done(err == nil) }()

	return installDependenciesForPluginSpec(ctx, spec, os.Stderr /* redirect stdout to stderr */, os.Stderr)
}

func installDependenciesForPluginSpec(
	ctx context.Context, spec workspace.PluginDescriptor, stdout, stderr io.Writer,
) error {
	dir, err := spec.DirPath()
	if err != nil {
		return err
	}
	subdir := filepath.Join(dir, spec.SubDir())

	// Install dependencies, if needed.
	proj, err := workspace.LoadPluginProject(filepath.Join(subdir, "PulumiPlugin.yaml"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("loading PulumiPlugin.yaml: %w", err)
	}
	if proj == nil {
		return nil
	}

	pctx, err := plugin.NewContextWithRoot(ctx,
		diagutil.Diag(),
		diagutil.Diag(),
		nil,    // host
		subdir, // pwd
		subdir, // root
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

	return errors.Join(InstallPluginAtPath(pctx, proj, stdout, stderr), pctx.Close())
}

func InstallPluginAtPath(pctx *plugin.Context, proj *workspace.PluginProject, stdout, stderr io.Writer) error {
	if err := proj.Validate(); err != nil {
		return err
	}
	runtime, err := pctx.Host.LanguageRuntime(proj.Runtime.Name())
	if err != nil {
		return err
	}
	entryPoint := "." // Plugin's are not able to set a non-standard entry point.
	pInfo := plugin.NewProgramInfo(pctx.Root, pctx.Pwd, entryPoint, proj.Runtime.Options())
	return cmdutil.InstallDependencies(runtime, plugin.InstallDependenciesRequest{
		Info:                    pInfo,
		UseLanguageVersionTools: false,
		IsPlugin:                true,
	}, stdout, stderr)
}
