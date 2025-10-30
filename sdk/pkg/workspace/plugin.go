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
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/sdk/v3/pkg/util"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// InstallPluginError is returned by InstallPlugin if we couldn't install the plugin
type InstallPluginError struct {
	// The specification of the plugin to install
	Spec	workspace.PluginSpec
	// The underlying error that occurred during the download or install.
	Err	error
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
			Spec:	pluginSpec,
			Err:	fmt.Errorf("error downloading provider %s to file: %w", pluginSpec.Name, err),
		}
	}

	logging.V(1).Infof("Automatically installing provider %s", pluginSpec.Name)
	err = InstallPluginContent(context.Background(), pluginSpec, tarPlugin{downloadedFile}, false)
	if err != nil {
		return nil, &InstallPluginError{
			Spec:	pluginSpec,
			Err:	fmt.Errorf("error installing provider %s: %w", pluginSpec.Name, err),
		}
	}

	return pluginSpec.Version, nil
}

type PluginContent interface {
	io.Closer

	writeToDir(pathToDir string) error
}

func SingleFilePlugin(f *os.File, spec workspace.PluginSpec) PluginContent {
	return singleFilePlugin{F: f, Kind: spec.Kind, Name: spec.Name}
}

type singleFilePlugin struct {
	F	*os.File
	Kind	apitype.PluginKind
	Name	string
}

func (p singleFilePlugin) writeToDir(finalDir string) error {
	bytes, err := io.ReadAll(p.F)
	if err != nil {
		return err
	}

	finalPath := filepath.Join(finalDir, fmt.Sprintf("pulumi-%s-%s", p.Kind, p.Name))
	if runtime.GOOS == "windows" {
		finalPath += ".exe"
	}
	// We are writing an executable.
	return os.WriteFile(finalPath, bytes, 0o700)	//nolint:gosec
}

func (p singleFilePlugin) Close() error {
	return p.F.Close()
}

func TarPlugin(tgz io.ReadCloser) PluginContent {
	return tarPlugin{Tgz: tgz}
}

type tarPlugin struct {
	Tgz io.ReadCloser
}

func (p tarPlugin) Close() error {
	return p.Tgz.Close()
}

func (p tarPlugin) writeToDir(finalPath string) error {
	return archive.ExtractTGZ(p.Tgz, finalPath)
}

func DirPlugin(rootPath string) PluginContent {
	return dirPlugin{Root: rootPath}
}

type dirPlugin struct {
	Root string
}

func (p dirPlugin) Close() error {
	return nil
}

func (p dirPlugin) writeToDir(dstRoot string) error {
	return filepath.WalkDir(p.Root, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath := strings.TrimPrefix(srcPath, p.Root)
		dstPath := filepath.Join(dstRoot, relPath)

		if srcPath == p.Root {
			return nil
		}
		if d.IsDir() {
			return os.Mkdir(dstPath, 0o700)
		}

		src, err := os.Open(srcPath)
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		bytes, err := io.ReadAll(src)
		if err != nil {
			return err
		}

		return os.WriteFile(dstPath, bytes, info.Mode())
	})
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

// InstallPluginContent installs a plugin's tarball into the cache. It validates that plugin names are in the expected
// format. Previous versions of Pulumi extracted the tarball to a temp directory first, and then renamed the temp
// directory to the final directory. The rename operation fails often enough on Windows due to aggressive virus scanners
// opening files in the temp directory. To address this, we now extract the tarball directly into the final directory,
// and use file locks to prevent concurrent installs.
//
// Each plugin has its own file lock, with the same name as the plugin directory, with a `.lock` suffix.
// During installation an empty file with a `.partial` suffix is created, indicating that installation is in-progress.
// The `.partial` file is deleted when installation is complete, indicating that the plugin has finished installing.
// If a failure occurs during installation, the `.partial` file will remain, indicating the plugin wasn't fully
// installed. The next time the plugin is installed, the old installation directory will be removed and replaced with
// a fresh installation.
func InstallPluginContent(ctx context.Context, spec workspace.PluginSpec, content PluginContent, reinstall bool) error {
	defer contract.IgnoreClose(content)

	// Fetch the directory into which we will expand this tarball.
	finalDir, err := spec.DirPath()
	if err != nil {
		return err
	}

	// Create a file lock file at <pluginsdir>/<kind>-<name>-<version>.lock.
	unlock, err := installLock(spec)
	if err != nil {
		return err
	}
	defer unlock()

	// Cleanup any temp dirs from failed installations of this plugin from previous versions of Pulumi.
	if err := cleanupTempDirs(finalDir); err != nil {
		// We don't want to fail the installation if there was an error cleaning up these old temp dirs.
		// Instead, log the error and continue on.
		logging.V(5).Infof("Install: Error cleaning up temp dirs: %s", err)
	}

	// Get the partial file path (e.g. <pluginsdir>/<kind>-<name>-<version>.partial).
	partialFilePath, err := spec.PartialFilePath()
	if err != nil {
		return err
	}

	// Check whether the directory exists while we were waiting on the lock.
	_, finalDirStatErr := os.Stat(finalDir)
	if finalDirStatErr == nil {
		_, partialFileStatErr := os.Stat(partialFilePath)
		if partialFileStatErr != nil {
			if !os.IsNotExist(partialFileStatErr) {
				return partialFileStatErr
			}
			if !reinstall {
				// finalDir exists, there's no partial file, and we're not reinstalling, so the plugin is already
				// installed.
				return nil
			}
		}

		// Either the partial file exists--meaning a previous attempt at installing the plugin failed--or we're
		// deliberately reinstalling the plugin. Delete finalDir so we can try installing again. There's no need to
		// delete the partial file since we'd just be recreating it again below anyway.
		if err := os.RemoveAll(finalDir); err != nil {
			return err
		}
	} else if !os.IsNotExist(finalDirStatErr) {
		return finalDirStatErr
	}

	// Create an empty partial file to indicate installation is in-progress.
	if err := os.WriteFile(partialFilePath, nil, 0o600); err != nil {
		return err
	}

	// Create the final directory.
	if err := os.MkdirAll(finalDir, 0o700); err != nil {
		return err
	}

	if err := content.writeToDir(finalDir); err != nil {
		return err
	}

	// Even though we deferred closing the tarball at the beginning of this function, go ahead and explicitly close
	// it now since we're finished extracting it, to prevent subsequent output from being displayed oddly with
	// the progress bar.
	contract.IgnoreClose(content)

	err = spec.InstallDependencies(ctx)
	if err != nil {
		return err
	}

	// Installation is complete. Remove the partial file.
	return os.Remove(partialFilePath)
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
