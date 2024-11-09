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
	"fmt"
	"io"
	"time"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/util"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// InstallPluginError is returned by InstallPlugin if we couldn't install the plugin
type InstallPluginError struct {
	// The specification of the plugin to install
	Spec workspace.PluginSpec
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

func InstallPlugin(ctx context.Context, pluginSpec workspace.PluginSpec,
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
	err = pluginSpec.Install(downloadedFile, false)
	if err != nil {
		return nil, &InstallPluginError{
			Spec: pluginSpec,
			Err:  fmt.Errorf("error installing provider %s: %w", pluginSpec.Name, err),
		}
	}

	return pluginSpec.Version, nil
}
