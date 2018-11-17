// Copyright 2016-2018, Pulumi Corporation.
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
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/blang/semver"
	"github.com/djherbis/times"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

// PluginInfo provides basic information about a plugin.  Each plugin gets installed into a system-wide
// location, by default `~/.pulumi/plugins/<kind>-<name>-<version>/`.  A plugin may contain multiple files,
// however the primary loadable executable must be named `pulumi-<kind>-<name>`.
type PluginInfo struct {
	Name         string          // the simple name of the plugin.
	Path         string          // the path that a plugin was loaded from.
	Kind         PluginKind      // the kind of the plugin (language, resource, etc).
	Version      *semver.Version // the plugin's semantic version, if present.
	Size         int64           // the size of the plugin, in bytes.
	InstallTime  time.Time       // the time the plugin was installed.
	LastUsedTime time.Time       // the last time the plugin was used.
}

// Dir gets the expected plugin directory for this plugin.
func (info PluginInfo) Dir() string {
	dir := fmt.Sprintf("%s-%s", info.Kind, info.Name)
	if info.Version != nil {
		dir = fmt.Sprintf("%s-v%s", dir, info.Version.String())
	}
	return dir
}

// File gets the expected filename for this plugin.
func (info PluginInfo) File() string {
	return info.FilePrefix() + info.FileSuffix()
}

// FilePrefix gets the expected default file prefix for the plugin.
func (info PluginInfo) FilePrefix() string {
	return fmt.Sprintf("pulumi-%s-%s", info.Kind, info.Name)
}

// FileSuffix returns the suffix for the plugin (if any).
func (info PluginInfo) FileSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// DirPath returns the directory where this plugin should be installed.
func (info PluginInfo) DirPath() (string, error) {
	dir, err := GetPluginDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, info.Dir()), nil
}

// FilePath returns the full path where this plugin's primary executable should be installed.
func (info PluginInfo) FilePath() (string, error) {
	dir, err := info.DirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, info.File()), nil
}

// Delete removes the plugin from the cache.  It also deletes any supporting files in the cache, which includes
// any files that contain the same prefix as the plugin itself.
func (info PluginInfo) Delete() error {
	dir, err := info.DirPath()
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// SetFileMetadata adds extra metadata from the given file, representing this plugin's directory.
func (info *PluginInfo) SetFileMetadata(path string) error {
	// Get the file info.
	file, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Next, get the size from the directory (or, if there is none, just the file).
	size, err := getPluginSize(path)
	if err != nil {
		return errors.Wrapf(err, "getting plugin dir %s size", path)
	}
	info.Size = size

	// Next get the access times from the plugin binary itself.
	tinfo := times.Get(file)

	if tinfo.HasBirthTime() {
		info.InstallTime = tinfo.BirthTime()
	}

	info.LastUsedTime = tinfo.AccessTime()
	return nil
}

// Install installs a plugin's tarball into the cache.  It validates that plugin names are in the expected format.
func (info PluginInfo) Install(tarball io.ReadCloser) error {
	// Fetch the directory into which we will expand this tarball, and create it.
	finalDir, err := info.DirPath()
	if err != nil {
		return err
	}

	// We unpack the tarball into a temporary directory in the plugin cache and after that has completed rename it
	// to prevent posioning the cache if the install is CTRL-C'd,
	tempDir := finalDir + ".tmp"

	// If the temporary folder already exists, try to remove it. Note that os.RemoveAll returns an nil error
	// when the directory does not already exist.
	if err := os.RemoveAll(tempDir); err != nil {
		return errors.Wrapf(err, "deleting temporary directory %s", tempDir)
	}

	if err = os.MkdirAll(tempDir, 0700); err != nil {
		return errors.Wrapf(err, "creating plugin directory %s", tempDir)
	}

	// Unzip and untar the file as we go. We do this inside a function so that the `defer`'s to close files happen
	// before we later try to rename the directory. Otherwise, the open file handles cause issues on Windows.
	err = (func() error {
		defer contract.IgnoreClose(tarball)
		gzr, err := gzip.NewReader(tarball)
		if err != nil {
			return errors.Wrapf(err, "unzipping")
		}
		r := tar.NewReader(gzr)
		for {
			header, err := r.Next()
			if err == io.EOF {
				break
			} else if err != nil {
				return errors.Wrapf(err, "untarring")
			}

			path := filepath.Join(tempDir, header.Name)

			switch header.Typeflag {
			case tar.TypeDir:
				// Create any directories as needed.
				if _, err := os.Stat(path); err != nil {
					if err = os.MkdirAll(path, 0700); err != nil {
						return errors.Wrapf(err, "untarring dir %s", path)
					}
				}
			case tar.TypeReg:
				// Expand files into the target directory.
				dst, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
				if err != nil {
					return errors.Wrapf(err, "opening file %s for untar", path)
				}
				defer contract.IgnoreClose(dst)
				if _, err = io.Copy(dst, r); err != nil {
					return errors.Wrapf(err, "untarring file %s", path)
				}
			default:
				return errors.Errorf("unexpected plugin file type %s (%v)", header.Name, header.Typeflag)
			}
		}

		return nil
	})()
	if err != nil {
		return err
	}

	return os.Rename(tempDir, finalDir)
}

func (info PluginInfo) String() string {
	var version string
	if v := info.Version; v != nil {
		version = fmt.Sprintf("-%s", v)
	}
	return info.Name + version
}

// PluginKind represents a kind of a plugin that may be dynamically loaded and used by Pulumi.
type PluginKind string

const (
	// AnalyzerPlugin is a plugin that can be used as a resource analyzer.
	AnalyzerPlugin PluginKind = "analyzer"
	// LanguagePlugin is a plugin that can be used as a language host.
	LanguagePlugin PluginKind = "language"
	// ResourcePlugin is a plugin that can be used as a resource provider for custom CRUD operations.
	ResourcePlugin PluginKind = "resource"
)

// IsPluginKind returns true if k is a valid plugin kind, and false otherwise.
func IsPluginKind(k string) bool {
	switch PluginKind(k) {
	case AnalyzerPlugin, LanguagePlugin, ResourcePlugin:
		return true
	default:
		return false
	}
}

// HasPlugin returns true if the given plugin exists.
func HasPlugin(plug PluginInfo) bool {
	dir, err := plug.DirPath()
	if err == nil {
		_, err := os.Stat(dir)
		if err == nil {
			return true
		}
	}
	return false
}

// HasPluginGTE returns true if the given plugin exists at the given version number or greater.
func HasPluginGTE(plug PluginInfo) (bool, error) {
	// If an exact match, return true right away.
	if HasPlugin(plug) {
		return true, nil
	}

	// Otherwise, load up the list of plugins and find one with the same name/type and >= version.
	plugs, err := GetPlugins()
	if err != nil {
		return false, err
	}
	for _, p := range plugs {
		if p.Name == plug.Name &&
			p.Kind == plug.Kind &&
			(p.Version != nil && plug.Version != nil && p.Version.GTE(*plug.Version)) {
			return true, nil
		}
	}
	return false, nil
}

// GetPluginDir returns the directory in which plugins on the current machine are managed.
func GetPluginDir() (string, error) {
	u, err := user.Current()
	if u == nil || err != nil {
		return "", errors.Wrapf(err, "getting user home directory")
	}
	return filepath.Join(u.HomeDir, BookkeepingDir, PluginDir), nil
}

// GetPlugins returns a list of installed plugins.
func GetPlugins() ([]PluginInfo, error) {
	// To get the list of plugins, simply scan the directory in the usual place.
	dir, err := GetPluginDir()
	if err != nil {
		return nil, err
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Now read the file infos and create the plugin infos.
	var plugins []PluginInfo
	for _, file := range files {
		// Skip anything that doesn't look like a plugin.
		if kind, name, version, ok := tryPlugin(file); ok {
			plugin := PluginInfo{
				Name:    name,
				Kind:    kind,
				Version: &version,
			}
			if err = plugin.SetFileMetadata(filepath.Join(dir, file.Name())); err != nil {
				return nil, err
			}
			plugins = append(plugins, plugin)
		}
	}
	return plugins, nil
}

// GetPluginPath finds a plugin's path by its kind, name, and optional version.  It will match the latest version that
// is >= the version specified.  If no version is supplied, the latest plugin for that given kind/name pair is loaded,
// using standard semver sorting rules.  A plugin may be overridden entirely by placing it on your $PATH.
func GetPluginPath(kind PluginKind, name string, version *semver.Version) (string, string, error) {
	// If we have a version of the plugin on its $PATH, use it.  This supports development scenarios.
	filename := (&PluginInfo{Kind: kind, Name: name, Version: version}).FilePrefix()
	if path, err := exec.LookPath(filename); err == nil {
		logging.V(6).Infof("GetPluginPath(%s, %s, %v): found on $PATH %s", kind, name, version, path)
		return "", path, nil
	}

	// Otherwise, check the plugin cache.
	plugins, err := GetPlugins()
	if err != nil {
		return "", "", errors.Wrapf(err, "loading plugin list")
	}
	var match *PluginInfo
	for _, cur := range plugins {
		// Since the value of cur changes as we iterate, we can't save a pointer to it. So let's have a local that
		// we can take a pointer to if this plugin is the best match yet.
		plugin := cur
		if plugin.Kind == kind && plugin.Name == name {
			// Always pick the most recent version of the plugin available.  Even if this is an exact match, we
			// keep on searching just in case there's a newer version available.
			var m *PluginInfo
			if match == nil && version == nil {
				m = &plugin // no existing match, no version spec, take it.
			} else if match != nil &&
				(match.Version == nil || (plugin.Version != nil && plugin.Version.GT(*match.Version))) {
				m = &plugin // existing match, but this plugin is newer, prefer it.
			} else if version != nil && plugin.Version != nil && plugin.Version.GTE(*version) {
				m = &plugin // this plugin is >= the version being requested, use it.
			}

			if m != nil {
				match = m
				logging.V(6).Infof("GetPluginPath(%s, %s, %s): found candidate (#%s)",
					kind, name, version, match.Version)
			}
		}
	}

	if match != nil {
		matchDir, err := match.DirPath()
		if err != nil {
			return "", "", err
		}
		matchPath, err := match.FilePath()
		if err != nil {
			return "", "", err
		}

		logging.V(6).Infof("GetPluginPath(%s, %s, %v): found in cache at %s", kind, name, version, matchPath)
		return matchDir, matchPath, nil
	}

	return "", "", nil
}

// pluginRegexp matches plugin filenames: pulumi-KIND-NAME-VERSION[.exe].
var pluginRegexp = regexp.MustCompile(
	"^(?P<Kind>[a-z]+)-" + // KIND
		"(?P<Name>[a-zA-Z0-9-]*[a-zA-Z0-9])-" + // NAME
		"v(?P<Version>[0-9]+.[0-9]+.[0-9]+(-[a-zA-Z0-9-_.]+)?)$") // VERSION

// tryPlugin returns true if a file is a plugin, and extracts information about it.
func tryPlugin(file os.FileInfo) (PluginKind, string, semver.Version, bool) {
	// Only directories contain plugins.
	if !file.IsDir() {
		logging.V(11).Infof("skipping file in plugin directory: %s", file.Name())
		return "", "", semver.Version{}, false
	}

	// Filenames must match the plugin regexp.
	match := pluginRegexp.FindStringSubmatch(file.Name())
	if len(match) != len(pluginRegexp.SubexpNames()) {
		logging.V(11).Infof("skipping plugin %s with missing capture groups: expect=%d, actual=%d",
			file.Name(), len(pluginRegexp.SubexpNames()), len(match))
		return "", "", semver.Version{}, false
	}
	var kind PluginKind
	var name string
	var version *semver.Version
	for i, group := range pluginRegexp.SubexpNames() {
		v := match[i]
		switch group {
		case "Kind":
			// Skip invalid kinds.
			if IsPluginKind(v) {
				kind = PluginKind(v)
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

// getPluginSize recursively computes how much space is devoted to a given plugin.
func getPluginSize(path string) (int64, error) {
	file, err := os.Stat(path)
	if err != nil {
		return 0, nil
	}

	size := int64(0)
	if file.IsDir() {
		subs, err := ioutil.ReadDir(path)
		if err != nil {
			return 0, err
		}
		for _, child := range subs {
			add, err := getPluginSize(filepath.Join(path, child.Name()))
			if err != nil {
				return 0, err
			}
			size += add
		}
	} else {
		size += file.Size()
	}
	return size, nil
}
