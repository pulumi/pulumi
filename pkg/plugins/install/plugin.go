// Copyright 2016-2025, Pulumi Corporation.
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

package installer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pulumi/pulumi/pkg/v3/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	diagutil "github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
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

// installLock acquires a file lock used to prevent concurrent installs.
func installLock(spec workspace.PluginSpec) (unlock func(), err error) {
	finalDir, err := spec.DirPath()
	if err != nil {
		return nil, err
	}
	lockFilePath := finalDir + ".lock"

	if err := os.MkdirAll(filepath.Dir(lockFilePath), 0o700); err != nil {
		return nil, fmt.Errorf("creating plugin root: %w", err)
	}

	mutex := fsutil.NewFileMutex(lockFilePath)
	if err := mutex.Lock(); err != nil {
		return nil, err
	}
	return func() {
		contract.IgnoreError(mutex.Unlock())
	}, nil
}

func installDependenciesForPluginSpec(ctx context.Context, spec workspace.PluginSpec, stdout, stderr io.Writer) error {
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

	return errors.Join(installPluginAtPath(pctx, proj, stdout, stderr), pctx.Close())
}

func installPluginAtPath(pctx *plugin.Context, proj *workspace.PluginProject, stdout, stderr io.Writer) error {
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

// installingPluginRegexp matches the name of temporary folders. Previous versions of Pulumi first extracted
// plugins to a temporary folder with a suffix of `.tmpXXXXXX` (where `XXXXXX`) is a random number, from
// os.CreateTemp. We should ignore these folders.
var installingPluginRegexp = regexp.MustCompile(`\.tmp[0-9]+$`)

// cleanupTempDirs cleans up leftover temp dirs from failed installs with previous versions of Pulumi.
func cleanupTempDirs(finalDir string) error {
	dir := filepath.Dir(finalDir)

	infos, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, info := range infos {
		// Temp dirs have a suffix of `.tmpXXXXXX` (where `XXXXXX`) is a random number,
		// from os.CreateTemp.
		if info.IsDir() && installingPluginRegexp.MatchString(info.Name()) {
			path := filepath.Join(dir, info.Name())
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("cleaning up temp dir %s: %w", path, err)
			}
		}
	}

	return nil
}
