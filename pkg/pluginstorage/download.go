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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pulumi/pulumi/pkg/v3/util/atomicinstall"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// UnpackContents installs a plugin's tarball into the cache. It validates that
// plugin names are in the expected format.
//
// cleanup *must* be called to avoid leaking system level resources.  It should
// be passed `true` if the plugin was successfully installed, and `false`
// otherwise. The plugin is not considered installed until cleanup is called,
// and it is not considered installed successfully until cleanup(true) is
// called.
func UnpackContents(
	ctx context.Context, spec workspace.PluginDescriptor, content Content, reinstall bool,
) (cleanup func(success bool) error, err error) {
	defer contract.IgnoreClose(content)

	pluginDir, err := workspace.GetPluginDir()
	if err != nil {
		return nil, err
	}
	store := atomicinstall.NewStore(pluginDir)
	key := spec.Dir()

	lock, err := store.Mutate(ctx, key)
	if err != nil {
		return nil, err
	}

	if _, ok := lock.Installed(); ok && !reinstall {
		return func(bool) error { return nil }, lock.Close()
	}

	// If we are not passing back a cleaup function, then we need to close the lock.
	defer func() {
		if cleanup == nil {
			contract.IgnoreClose(lock)
		}
	}()

	finalDir, err := lock.Reset()
	if err != nil {
		return nil, err
	}

	// Previous versions of Pulumi extracted the tarball to a temp directory first, and then renamed the temp
	// directory to the final directory. The rename operation fails often enough on Windows due to aggressive
	// virus scanners opening files in the temp directory. To address this, we now extract the tarball directly
	// into the final directory, and use file locks to prevent concurrent installs.
	//
	// We cleanup the old directory format here.
	if err := cleanupTempDirs(finalDir); err != nil {
		// We don't want to fail the installation if there was an error cleaning up these old temp dirs.
		// Instead, log the error and continue on.
		slog.InfoContext(ctx, "Install: Error cleaning up temp dirs", "err", err)
	}

	if err := content.writeToDir(finalDir); err != nil {
		return nil, err
	}

	// Even though we deferred closing the tarball at the beginning of this function, go ahead and explicitly close
	// it now since we're finished extracting it, to prevent subsequent output from being displayed oddly with
	// the progress bar.
	contract.IgnoreClose(content)

	// The download is complete.
	return func(success bool) error {
		var err error
		if success {
			err = lock.Commit()
		}
		return errors.Join(err, lock.Close())
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
