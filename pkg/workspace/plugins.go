// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package workspace

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/blang/semver"
	"github.com/djherbis/times"
	"github.com/golang/glog"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
)

// PluginInfo provides basic information about a plugin.
type PluginInfo struct {
	Path         string         // the path to the plugin.
	Name         string         // the simple name of the plugin.
	Kind         PluginKind     // the kind of the plugin (language, resource, etc).
	Version      semver.Version // the plugin's semantic version.
	Size         int64          // the size of the plugin, in bytes.
	InstallTime  time.Time      // the time the plugin was installed.
	LastUsedTime time.Time      // the last time the plugin was used.
}

// FilePrefix gets the expected default file prefix for the plugin.
func (info PluginInfo) FilePrefix() string {
	return fmt.Sprintf("pulumi-%s-%s-v%s", info.Kind, info.Name, info.Version.String())
}

// Delete removes the plugin from the cache.  It also deletes any supporting files in the cache, which includes
// any files that contain the same prefix as the plugin itself.
func (info PluginInfo) Delete() error {
	return os.Remove(info.Path)
}

// Install installs a plugin's tarball into the cache.  It validates that plugin names are in the expected format.
func (info PluginInfo) Install(tarball io.ReadCloser) error {
	// Fetch the directory into which we will expand this tarball.
	pluginDir, err := GetPluginDir()
	if err != nil {
		return err
	}

	// Unzip and untar the file as we go.
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
		switch header.Typeflag {
		case tar.TypeReg:
			// Ensure the file has the anticipated prefix.
			if !strings.HasPrefix(header.Name, info.FilePrefix()) {
				return errors.Errorf(
					"plugin file %s doesn't have the expected prefix %s", header.Name, info.FilePrefix())
			}

			// If so, expand it into the plugin home directory.
			dst, err := os.Open(filepath.Join(pluginDir, header.Name))
			if err != nil {
				return err
			}
			defer contract.IgnoreClose(dst)
			if _, err = io.Copy(dst, r); err != nil {
				return err
			}
		case tar.TypeDir:
			return errors.Errorf("unexpected plugin directory %s", header.Name)
		default:
			return errors.Errorf("unexpected plugin file type %s (%v)", header.Name, header.Typeflag)
		}
	}
	return nil
}

func (info PluginInfo) String() string {
	return fmt.Sprintf("%s-%s", info.Name, info.Version.String())
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

// GetPluginDir returns the directory in which plugins on the current machine are managed.
func GetPluginDir() (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, BookkeepingDir, PluginDir), nil
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
		if ok, kind, name, version := isPlugin(file); ok {
			tinfo := times.Get(file)
			plugins = append(plugins, PluginInfo{
				Path:         filepath.Join(dir, file.Name()),
				Name:         name,
				Kind:         kind,
				Version:      version,
				Size:         file.Size(),
				InstallTime:  tinfo.BirthTime(),
				LastUsedTime: tinfo.AccessTime(),
			})
		}
	}
	return plugins, nil
}

// pluginRegexp matches plugin filenames: pulumi-KIND-NAME-VERSION[.exe].
var pluginRegexp = regexp.MustCompile(
	"^pulumi-" + // pulumi prefix
		"(?P<Kind>[a-z]+)-" + // KIND
		"(?P<Name>[a-zA-Z0-9-]*[a-zA-Z0-9])-" + // NAME
		"v(?P<Version>[0-9]+.[0-9]+.[0-9]+(-[a-zA-Z0-9-_.]+)?)" + // VERSION
		"(\\.exe)?$") // optional .exe extension on Windows

// isPlugin returns true if a file is a plugin, and extracts information about it.
func isPlugin(file os.FileInfo) (bool, PluginKind, string, semver.Version) {
	// Only files are plugins.
	if file.IsDir() {
		glog.V(11).Infof("skipping plugin as a directory: %s", file.Name())
		return false, "", "", semver.Version{}
	}

	// Filenames must match the plugin regexp.
	match := pluginRegexp.FindStringSubmatch(file.Name())
	if len(match) != len(pluginRegexp.SubexpNames()) {
		glog.V(11).Infof("skipping plugin %s with missing capture groups: expect=%d, actual=%d",
			file.Name(), len(pluginRegexp.SubexpNames()), len(match))
		return false, "", "", semver.Version{}
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
				glog.V(11).Infof("skipping invalid plugin kind: %s", v)
			}
		case "Name":
			name = v
		case "Version":
			// Skip invalid versions.
			ver, err := semver.ParseTolerant(v)
			if err == nil {
				version = &ver
			} else {
				glog.V(11).Infof("skipping invalid plugin version: %s", v)
			}
		}
	}

	// If anything was missing or invalid, skip this plugin.
	if kind == "" || name == "" || version == nil {
		glog.V(11).Infof("skipping plugin with missing information: kind=%s, name=%s, version=%v",
			kind, name, version)
		return false, "", "", semver.Version{}
	}

	return true, kind, name, *version
}
