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
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/blang/semver"
	"github.com/djherbis/times"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func NewFs(root string) Store {
	return fsStore{root: root}
}

var (
	globalFsMutexLock sync.Mutex
	globalFsMutexes   map[string]*fsutil.FileMutex = map[string]*fsutil.FileMutex{}
)

type fsStore struct {
	// The root of the file system used.
	root string
}

func (fs fsStore) LockSpec(ctx context.Context, spec workspace.PluginSpec) (Lock, error) {
	finalDir := filepath.Join(fs.root, spec.Dir())
	lockFilePath := finalDir + ".lock"

	if err := os.MkdirAll(filepath.Dir(lockFilePath), 0o700); err != nil {
		return nil, fmt.Errorf("creating plugin root: %w", err)
	}

	if !filepath.IsAbs(lockFilePath) {
		var err error
		lockFilePath, err = filepath.Abs(lockFilePath)
		if err != nil {
			return nil, fmt.Errorf("unable to acquire absolute file name of %s: %w",
				lockFilePath, err)
		}
	}

	globalFsMutexLock.Lock()
	mutex, ok := globalFsMutexes[lockFilePath]
	if !ok {
		mutex = fsutil.NewFileMutex(lockFilePath)
		globalFsMutexes[lockFilePath] = mutex
	}
	globalFsMutexLock.Unlock()

	if err := mutex.Lock(); err != nil {
		return nil, err
	}

	// Cleanup any temp dirs from failed installations of this plugin from previous versions of Pulumi.
	if err := cleanupTempDirs(finalDir); err != nil {
		// We don't want to fail the installation if there was an error cleaning up these old temp dirs.
		// Instead, log the error and continue on.
		logging.V(5).Infof("Install: Error cleaning up temp dirs: %s", err)
	}

	return fsLock{mutex, finalDir}, nil
}

type fsLock struct {
	*fsutil.FileMutex
	dir string // The dir that the lock is locking
}

func (m fsLock) isLock() {}

func (fs fsStore) partialPath(spec workspace.PluginSpec) string {
	return filepath.Join(fs.root, spec.Dir()+".partial")
}

func (fs fsStore) SetPartial(ctx context.Context, spec workspace.PluginSpec) error {
	return os.WriteFile(fs.partialPath(spec), nil, 0o600)
}

func (fs fsStore) RemovePartial(ctx context.Context, spec workspace.PluginSpec) error {
	return os.Remove(fs.partialPath(spec))
}

func (fs fsStore) IsPartial(ctx context.Context, spec workspace.PluginSpec) (bool, error) {
	partialFilePath := fs.partialPath(spec)
	_, err := os.Stat(partialFilePath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (fs fsStore) List(ctx context.Context) ([]workspace.PluginInfo, error) {
	files, err := os.ReadDir(fs.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Now read the file infos and create the plugin infos.
	var plugins []workspace.PluginInfo
	for _, file := range files {
		// Skip anything that doesn't look like a plugin.
		if kind, name, version, ok := tryPlugin(file); ok {
			path := filepath.Join(fs.root, file.Name())
			plugin := workspace.PluginInfo{
				Name:    name,
				Kind:    kind,
				Version: &version,
				Path:    path,
			}
			if _, err := os.Stat(path + ".partial"); err == nil {
				// Skip it if the partial file exists, meaning the plugin is not fully installed.
				continue
			} else if !os.IsNotExist(err) {
				return nil, err
			}
			if err = setFileMetadata(&plugin, path); err != nil {
				return nil, err
			}
			plugins = append(plugins, plugin)
		}
	}
	return plugins, nil
}

func (f fsStore) Dir(ctx context.Context, spec workspace.PluginSpec) (string, error) {
	dir := filepath.Join(f.root, spec.Dir(), spec.SubDir())
	stat, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !stat.IsDir() {
		return "", fmt.Errorf("invalid installation: expected %q to be a directory", dir)
	}
	return filepath.Abs(dir)
}

func (fs fsStore) Write(ctx context.Context, spec workspace.PluginSpec, content Content) error {
	finalDir := filepath.Join(fs.root, spec.Dir())
	// Create the final directory.
	if err := os.MkdirAll(finalDir, 0o700); err != nil {
		return err
	}

	return errors.Join(
		content.writeToDir(finalDir),
		content.Close(),
	)
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

// pluginRegexp matches plugin directory names: pulumi-KIND-NAME-VERSION.
var pluginRegexp = regexp.MustCompile(
	"^(?P<Kind>[a-z]+)-" + // KIND
		"(?P<Name>[a-zA-Z0-9-][a-zA-Z0-9-_.]*[a-zA-Z0-9])-" + // NAME
		"v(?P<Version>.*)$") // VERSION

// tryPlugin returns true if a file is a plugin, and extracts information about it.
func tryPlugin(file os.DirEntry) (apitype.PluginKind, string, semver.Version, bool) {
	// Only directories contain plugins.
	if !file.IsDir() {
		logging.V(11).Infof("skipping file in plugin directory: %s", file.Name())
		return "", "", semver.Version{}, false
	}

	// Ignore plugins which are being installed
	if installingPluginRegexp.MatchString(file.Name()) {
		logging.V(11).Infof("skipping plugin %s which is being installed", file.Name())
		return "", "", semver.Version{}, false
	}

	// Filenames must match the plugin regexp.
	match := pluginRegexp.FindStringSubmatch(file.Name())
	if len(match) != len(pluginRegexp.SubexpNames()) {
		logging.V(11).Infof("skipping plugin %s with missing capture groups: expect=%d, actual=%d",
			file.Name(), len(pluginRegexp.SubexpNames()), len(match))
		return "", "", semver.Version{}, false
	}
	var kind apitype.PluginKind
	var name string
	var version *semver.Version
	for i, group := range pluginRegexp.SubexpNames() {
		v := match[i]
		switch group {
		case "Kind":
			// Skip invalid kinds.
			if apitype.IsPluginKind(v) {
				kind = apitype.PluginKind(v)
			} else {
				logging.V(11).Infof("skipping invalid plugin kind: %s", v)
			}
		case "Name":
			name = v
		case "Version":
			// Skip invalid versions.
			ver, err := semver.ParseTolerant(v)
			if err == nil {
				version = &ver
			} else {
				logging.V(11).Infof("skipping invalid plugin version: %s", v)
			}
		}
	}

	// If anything was missing or invalid, skip this plugin.
	if kind == "" || name == "" || version == nil {
		logging.V(11).Infof("skipping plugin with missing information: kind=%s, name=%s, version=%v",
			kind, name, version)
		return "", "", semver.Version{}, false
	}

	return kind, name, *version, true
}

// setFileMetadata adds extra metadata from the given file, representing this plugin's directory.
func setFileMetadata(info *workspace.PluginInfo, path string) error {
	// Get the file info.
	file, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Next get the access times from the plugin folder.
	tinfo := times.Get(file)

	if tinfo.HasChangeTime() {
		info.InstallTime = tinfo.ChangeTime()
	} else {
		info.InstallTime = tinfo.ModTime()
	}

	info.LastUsedTime = tinfo.AccessTime()

	if info.Kind == apitype.ResourcePlugin {
		var v string
		if info.Version != nil {
			v = "-" + info.Version.String() + "-"
		}
		info.SchemaPath = filepath.Join(filepath.Dir(path), "schema-"+info.Name+v+".json")
		info.SchemaTime = tinfo.ModTime()
	}

	return nil
}
