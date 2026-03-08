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

package pluginstorage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// UnpackContents installs a plugin's tarball into the cache. It validates that
// plugin names are in the expected format. cleanup *must* be called to avoid leaking
// system level resources. It should be passed `true` if the plugin was successfully
// installed, and `false` otherwise.
//
// Cleanup:
//
// In addition to the downloaded plugin, this file creates 2 empty files on disk:
//
// - "<spec>.lock"
// - "<spec>.partial"
//
// "<spec>.lock" establishes a process level lock on the plugin, preventing some
// concurrent operations on the plugin from multiple versions of Pulumi. It should always
// be removed after the plugin is installed, *or* if the install fails for any reason.
//
// "<spec>.partial" indicates that the plugin is not yet fully installed. "<spec>.partial"
// should be removed after the plugin is *successfully* installed, but left if the install
// fails for any reason.
func UnpackContents(
	ctx context.Context, spec workspace.PluginDescriptor, content Content, reinstall bool,
) (cleanup func(success bool), err error) {
	defer contract.IgnoreClose(content)

	// Fetch the directory into which we will expand this tarball.
	finalDir, err := spec.DirPath()
	if err != nil {
		return nil, err
	}

	// Create a file lock file at <pluginsdir>/<kind>-<name>-<version>.lock.
	unlock, err := lockPluginForInstall(spec)
	if err != nil {
		return nil, err
	}
	// If we are not passing back a cleaup function, then we need to close the lock.
	defer func() {
		if cleanup == nil {
			unlock()
		}
	}()

	// Previous versions of Pulumi extracted the tarball to a temp directory first, and then renamed the temp
	// directory to the final directory. The rename operation fails often enough on Windows due to aggressive
	// virus scanners opening files in the temp directory. To address this, we now extract the tarball directly
	// into the final directory, and use file locks to prevent concurrent installs.
	//
	// We cleanup the old directory format here.
	if err := cleanupTempDirs(finalDir); err != nil {
		// We don't want to fail the installation if there was an error cleaning up these old temp dirs.
		// Instead, log the error and continue on.
		logging.V(5).Infof("Install: Error cleaning up temp dirs: %s", err)
	}

	// Get the partial file path (e.g. <pluginsdir>/<kind>-<name>-<version>.partial).
	partialFilePath, err := spec.PartialFilePath()
	if err != nil {
		return nil, err
	}

	// Check whether the directory exists while we were waiting on the lock.
	_, finalDirStatErr := os.Stat(finalDir)
	if finalDirStatErr == nil {
		_, partialFileStatErr := os.Stat(partialFilePath)
		if partialFileStatErr != nil {
			if !os.IsNotExist(partialFileStatErr) {
				return nil, partialFileStatErr
			}
			if !reinstall {
				// finalDir exists, there's no partial file, and we're not reinstalling, so the plugin is already
				// installed.
				unlock()
				return func(bool) {}, nil
			}
		}

		// Either the partial file exists--meaning a previous attempt at installing the plugin failed--or we're
		// deliberately reinstalling the plugin. Delete finalDir so we can try installing again. There's no need to
		// delete the partial file since we'd just be recreating it again below anyway.
		if err := os.RemoveAll(finalDir); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(finalDirStatErr) {
		return nil, finalDirStatErr
	}

	// Create an empty partial file to indicate installation is in-progress.
	if err := os.WriteFile(partialFilePath, nil, 0o600); err != nil {
		return nil, err
	}

	// Create the final directory.
	if err := os.MkdirAll(finalDir, 0o700); err != nil {
		return nil, err
	}

	if err := content.writeToDir(finalDir); err != nil {
		return nil, err
	}

	// Even though we deferred closing the tarball at the beginning of this function, go ahead and explicitly close
	// it now since we're finished extracting it, to prevent subsequent output from being displayed oddly with
	// the progress bar.
	contract.IgnoreClose(content)

	// The download is complete.
	return func(success bool) {
		if success {
			contract.IgnoreError(os.Remove(partialFilePath))
		}
		unlock()
	}, nil
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

// LockPluginForInstall acquires a file lock used to prevent concurrent installs.
func lockPluginForInstall(spec workspace.PluginDescriptor) (func(), error) {
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
