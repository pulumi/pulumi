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
	"fmt"
	"io"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// InstallPluginError is returned by InstallPlugin if we couldn't install the plugin
type InstallPluginError struct {
	// The name of the provider
	Name string
	// The requested version of the plugin, if any.
	Version *semver.Version
	// The pluginDownloadURL, if any.
	PluginDownloadURL string
	// The underlying error that occurred during the download or install.
	Err error
}

func (err *InstallPluginError) Error() string {
	var server string
	if err.PluginDownloadURL != "" {
		server = fmt.Sprintf(" --server %s", err.PluginDownloadURL)
	}

	if err.Version != nil {
		return fmt.Sprintf("Could not automatically download and install %[1]s plugin 'pulumi-%[1]s-%[2]s'"+
			" at version v%[3]s"+
			", install the plugin using `pulumi plugin install %[1]s %[2]s v%[3]s%[4]s`: %[5]v",
			workspace.ResourcePlugin, err.Name, err.Version, server, err.Err)
	}

	return fmt.Sprintf("Could not automatically download and install %[1]s plugin 'pulumi-%[1]s-%[2]s'"+
		", install the plugin using `pulumi plugin install %[1]s %[2]s%[3]s`: %[4]v",
		workspace.ResourcePlugin, err.Name, server, err.Err)
}

func (err *InstallPluginError) Unwrap() error {
	return err.Err
}

func InstallPlugin(pluginSpec workspace.PluginSpec, log func(sev diag.Severity, msg string)) error {
	if pluginSpec.Version == nil {
		var err error
		pluginSpec.Version, err = pluginSpec.GetLatestVersion()
		if err != nil {
			return fmt.Errorf("could not find latest version for provider %s: %w", pluginSpec.Name, err)
		}
	}

	wrapper := func(stream io.ReadCloser, size int64) io.ReadCloser {
		log(diag.Info, fmt.Sprintf("Downloading provider: %s", pluginSpec.Name))
		return stream
	}

	retry := func(err error, attempt int, limit int, delay time.Duration) {
		log(diag.Warning, fmt.Sprintf("error downloading provider: %s\n"+
			"Will retry in %v [%d/%d]", err, delay, attempt, limit))
	}

	logging.V(1).Infof("Automatically downloading provider %s", pluginSpec.Name)
	downloadedFile, err := workspace.DownloadToFile(pluginSpec, wrapper, retry)
	if err != nil {
		return &InstallPluginError{
			Name:              pluginSpec.Name,
			Version:           pluginSpec.Version,
			PluginDownloadURL: pluginSpec.PluginDownloadURL,
			Err:               fmt.Errorf("error downloading provider %s to file: %w", pluginSpec.Name, err),
		}
	}

	logging.V(1).Infof("Automatically installing provider %s", pluginSpec.Name)
	err = pluginSpec.Install(downloadedFile, false)
	if err != nil {
		return &InstallPluginError{
			Name:              pluginSpec.Name,
			Version:           pluginSpec.Version,
			PluginDownloadURL: pluginSpec.PluginDownloadURL,
			Err:               fmt.Errorf("error installing provider %s: %w", pluginSpec.Name, err),
		}
	}

	return nil
}
