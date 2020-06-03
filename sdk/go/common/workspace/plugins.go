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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/cheggaaa/pb"
	"github.com/djherbis/times"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v2/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/httputil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v2/go/common/version"
)

const (
	windowsGOOS = "windows"
)

var (
	enableLegacyPluginBehavior = os.Getenv("PULUMI_ENABLE_LEGACY_PLUGIN_SEARCH") != ""
)

// MissingError is returned by functions that attempt to load plugins if a plugin can't be located.
type MissingError struct {
	// Info contains information about the plugin that was not found.
	Info PluginInfo
}

// NewMissingError allocates a new error indicating the given plugin info was not found.
func NewMissingError(info PluginInfo) error {
	return &MissingError{
		Info: info,
	}
}

func (err *MissingError) Error() string {
	if err.Info.Version != nil {
		return fmt.Sprintf("no %[1]s plugin '%[2]s-v%[3]s' found in the workspace or on your $PATH, "+
			"install the plugin using `pulumi plugin install %[1]s %[2]s v%[3]s`",
			err.Info.Kind, err.Info.Name, err.Info.Version)
	}

	return fmt.Sprintf("no %s plugin '%s' found in the workspace or on your $PATH",
		err.Info.Kind, err.Info.String())
}

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
	ServerURL    string          // an optional server to use when downloading this plugin.
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
	if runtime.GOOS == windowsGOOS {
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

// Download fetches an io.ReadCloser for this plugin and also returns the size of the response (if known).
func (info PluginInfo) Download() (io.ReadCloser, int64, error) {
	// Figure out the OS/ARCH pair for the download URL.
	var os string
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		os = runtime.GOOS
	default:
		return nil, -1, errors.Errorf("unsupported plugin OS: %s", runtime.GOOS)
	}
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = runtime.GOARCH
	default:
		return nil, -1, errors.Errorf("unsupported plugin architecture: %s", runtime.GOARCH)
	}

	// If the plugin has a server, associated with it, download from there.  Otherwise use the "default" location, which
	// is hosted by Pulumi.
	serverURL := info.ServerURL
	if serverURL == "" {
		serverURL = "https://get.pulumi.com/releases/plugins"
	}
	serverURL = strings.TrimSuffix(serverURL, "/")

	// URL escape the path value to ensure we have the correct path for S3/CloudFront.
	endpoint := fmt.Sprintf("%s/%s",
		serverURL,
		url.QueryEscape(fmt.Sprintf("pulumi-%s-%s-v%s-%s-%s.tar.gz", info.Kind, info.Name, info.Version, os, arch)))

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, -1, err
	}

	userAgent := fmt.Sprintf("pulumi-cli/1 (%s; %s)", version.Version, runtime.GOOS)
	req.Header.Set("User-Agent", userAgent)

	resp, err := httputil.DoWithRetry(req, http.DefaultClient)
	if err != nil {
		return nil, -1, err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, -1, errors.Errorf("%d HTTP error fetching plugin from %s", resp.StatusCode, endpoint)
	}

	return resp.Body, resp.ContentLength, nil
}

// Install installs a plugin's tarball into the cache.  It validates that plugin names are in the expected format.
func (info PluginInfo) Install(tarball io.ReadCloser) error {
	// Fetch the directory into which we will expand this tarball, and create it.
	finalDir, err := info.DirPath()
	if err != nil {
		return err
	}

	// If part of the directory tree is missing, ioutil.TempDir will return an error, so make sure the path we're going
	// to create the temporary folder in actually exists.
	if err := os.MkdirAll(filepath.Dir(finalDir), 0700); err != nil {
		return errors.Wrap(err, "creating plugin root")
	}

	tempDir, err := ioutil.TempDir(filepath.Dir(finalDir), fmt.Sprintf("%s.tmp", filepath.Base(finalDir)))
	if err != nil {
		return errors.Wrapf(err, "creating plugin directory %s", tempDir)
	}

	// If we early out of this function, try to remove the temp folder we created.
	defer func() {
		contract.IgnoreError(os.RemoveAll(tempDir))
	}()

	// Uncompress the plugin. We do this inside a function so that the `defer`'s to close files
	// happen before we later try to rename the directory. Otherwise, the open file handles cause
	// issues on Windows.
	err = (func() error {
		defer contract.IgnoreClose(tarball)
		tarballBytes, err := ioutil.ReadAll(tarball)
		if err != nil {
			return err
		}

		return archive.UnTGZ(tarballBytes, tempDir)
	})()
	if err != nil {
		return err
	}

	// If two calls to `plugin install` for the same plugin are racing, the second one will be unable to rename
	// the directory. That's OK, just ignore the error. The temp directory created as part of the install will be
	// cleaned up when we exit by the defer above.
	fmt.Print("Moving plugin...")
	if err := os.Rename(tempDir, finalDir); err != nil && !os.IsExist(err) {
		switch err.(type) {
		case *os.LinkError:
			// On Windows, an Access Denied error is sometimes thrown when renaming. Work around by trying the
			// second time, which seems to work fine. See https://github.com/pulumi/pulumi/issues/2695
			if err := os.Rename(tempDir, finalDir); err != nil && !os.IsExist(err) {
				return errors.Wrap(err, "moving plugin")
			}
		default:
			return errors.Wrap(err, "moving plugin")
		}
	}
	fmt.Println(" done.")

	return nil
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

	// If we're not doing the legacy plugin behavior and we've been asked for a specific version, do the same plugin
	// search that we'd do at runtime. This ensures that `pulumi plugin install` works the same way that the runtime
	// loader does, to minimize confusion when a user has to install new plugins.
	if !enableLegacyPluginBehavior && plug.Version != nil {
		requestedVersion := semver.MustParseRange(plug.Version.String())
		_, err := SelectCompatiblePlugin(plugs, plug.Kind, plug.Name, requestedVersion)
		return err == nil, err
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

// GetPolicyDir returns the directory in which an organization's Policy Packs on the current machine are managed.
func GetPolicyDir(orgName string) (string, error) {
	return GetPulumiPath(PolicyDir, orgName)
}

// GetPolicyPath finds a PolicyPack by its name version, as well as a bool marked true if the path
// already exists and is a directory.
func GetPolicyPath(orgName, name, version string) (string, bool, error) {
	policiesDir, err := GetPolicyDir(orgName)
	if err != nil {
		return "", false, err
	}

	policyPackPath := path.Join(policiesDir, fmt.Sprintf("pulumi-analyzer-%s-v%s", name, version))

	file, err := os.Stat(policyPackPath)
	if err == nil && file.IsDir() {
		// PolicyPack exists. Return.
		return policyPackPath, true, nil
	} else if err != nil && !os.IsNotExist(err) {
		// Error trying to inspect PolicyPack FS entry. Return error.
		return "", false, err
	}

	// Not found. Return empty path.
	return policyPackPath, false, nil
}

// GetPluginDir returns the directory in which plugins on the current machine are managed.
func GetPluginDir() (string, error) {
	return GetPulumiPath(PluginDir)
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

	// At some point in the future, language plugins will be located in the plugin cache, just like regular plugins
	// (see pulumi/pulumi#956 for some of the reasons why this isn't the case today). For now, they ship next to the
	// `pulumi` binary. While we encourage this folder to be on the $PATH (and so the check above would have found
	// the language plugin) it's possible someone is running `pulumi` with an explicit path on the command line or
	// has done symlink magic such that `pulumi` is on the path, but the language plugins are not. So, if possible,
	// look next to the instance of `pulumi` that is running to find this language plugin.
	if kind == LanguagePlugin {
		exePath, exeErr := os.Executable()
		if exeErr == nil {
			fullPath, fullErr := filepath.EvalSymlinks(exePath)
			if fullErr == nil {
				for _, ext := range getCandidateExtensions() {
					candidate := filepath.Join(filepath.Dir(fullPath), filename+ext)
					// Let's see if the file is executable. On Windows, os.Stat() returns a mode of "-rw-rw-rw" so on
					// on windows we just trust the fact that the .exe can actually be launched.
					if stat, err := os.Stat(candidate); err == nil &&
						(stat.Mode()&0100 != 0 || runtime.GOOS == windowsGOOS) {
						logging.V(6).Infof("GetPluginPath(%s, %s, %v): found next to current executable %s",
							kind, name, version, candidate)

						return "", candidate, nil
					}
				}
			}
		}
	}

	// Otherwise, check the plugin cache.
	plugins, err := GetPlugins()
	if err != nil {
		return "", "", errors.Wrapf(err, "loading plugin list")
	}

	var match *PluginInfo
	if !enableLegacyPluginBehavior && version != nil {
		logging.V(6).Infof("GetPluginPath(%s, %s, %s): enabling new plugin behavior", kind, name, version)
		candidate, err := SelectCompatiblePlugin(plugins, kind, name, semver.MustParseRange(version.String()))
		if err != nil {
			return "", "", NewMissingError(PluginInfo{
				Name:    name,
				Kind:    kind,
				Version: version,
			})
		}
		match = &candidate
	} else {
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

// SortedPluginInfo is a wrapper around PluginInfo that allows for sorting by version.
type SortedPluginInfo []PluginInfo

func (sp SortedPluginInfo) Len() int { return len(sp) }
func (sp SortedPluginInfo) Less(i, j int) bool {
	iVersion := sp[i].Version
	jVersion := sp[j].Version
	switch {
	case iVersion == nil && jVersion == nil:
		return false
	case iVersion == nil:
		return true
	case jVersion == nil:
		return false
	default:
		return iVersion.LT(*jVersion)
	}
}
func (sp SortedPluginInfo) Swap(i, j int) { sp[i], sp[j] = sp[j], sp[i] }

// SelectCompatiblePlugin selects a plugin from the list of plugins with the given kind and name that sastisfies the
// requested semver range. It returns the highest version plugin that satisfies the requested constraints, or an error
// if no such plugin could be found.
//
// If there exist plugins in the plugin list that don't have a version, SelectCompatiblePlugin will select them if there
// are no other compatible plugins available.
func SelectCompatiblePlugin(
	plugins []PluginInfo, kind PluginKind, name string, requested semver.Range) (PluginInfo, error) {
	logging.V(7).Infof("SelectCompatiblePlugin(..., %s): beginning", name)
	var bestMatch PluginInfo
	var hasMatch bool

	// Before iterating over the list of plugins, sort the list of plugins by version in ascending order. This ensures
	// that we can do a single pass over the plugin list, from lowest version to greatest version, and be confident that
	// the best match that we find at the end is the greatest possible compatible version for the requested plugin.
	//
	// Plugins without versions are treated as having the lowest version. Ties between plugins without versions are
	// resolved arbitrarily.
	sort.Sort(SortedPluginInfo(plugins))
	for _, plugin := range plugins {
		switch {
		case plugin.Kind != kind || plugin.Name != name:
			// Not the plugin we're looking for.
		case !hasMatch && plugin.Version == nil:
			// This is the plugin we're looking for, but it doesn't have a version. We haven't seen anything better yet,
			// so take it.
			logging.V(7).Infof(
				"SelectCompatiblePlugin(..., %s): best plugin %s: no version and no other candidates",
				name, plugin.String())
			hasMatch = true
			bestMatch = plugin
		case plugin.Version == nil:
			// This is a rare case - we've already seen a version-less plugin and we're seeing another here. Ignore this
			// one and defer to the one we previously selected.
			logging.V(7).Infof("SelectCompatiblePlugin(..., %s): skipping plugin %s: no version", name, plugin.String())
		case requested(*plugin.Version):
			// This plugin is compatible with the requested semver range. Save it as the best match and continue.
			logging.V(7).Infof("SelectCompatiblePlugin(..., %s): best plugin %s: semver match", name, plugin.String())
			hasMatch = true
			bestMatch = plugin
		default:
			logging.V(7).Infof(
				"SelectCompatiblePlugin(..., %s): skipping plugin %s: semver mismatch", name, plugin.String())
		}
	}

	if !hasMatch {
		logging.V(7).Infof("SelectCompatiblePlugin(..., %s): failed to find match", name)
		return PluginInfo{}, errors.New("failed to locate compatible plugin")
	}
	logging.V(7).Infof("SelectCompatiblePlugin(..., %s): selecting plugin '%s': best match ", name, bestMatch.String())
	return bestMatch, nil
}

// ReadCloserProgressBar displays a progress bar for the given closer and returns a wrapper closer to manipulate it.
func ReadCloserProgressBar(
	closer io.ReadCloser, size int64, message string, colorization colors.Colorization) io.ReadCloser {
	if size == -1 {
		return closer
	}

	// If we know the length of the download, show a progress bar.
	bar := pb.New(int(size))
	bar.Prefix(colorization.Colorize(colors.SpecUnimportant + message + ":"))
	bar.Postfix(colorization.Colorize(colors.Reset))
	bar.SetMaxWidth(80)
	bar.SetUnits(pb.U_BYTES)
	bar.Start()

	return &barCloser{
		bar:        bar,
		readCloser: bar.NewProxyReader(closer),
	}
}

// getCandidateExtensions returns a set of file extensions (including the dot seprator) which should be used when
// probing for an executable file.
func getCandidateExtensions() []string {
	if runtime.GOOS == windowsGOOS {
		return []string{".exe", ".cmd"}
	}

	return []string{""}
}

// pluginRegexp matches plugin directory names: pulumi-KIND-NAME-VERSION.
var pluginRegexp = regexp.MustCompile(
	"^(?P<Kind>[a-z]+)-" + // KIND
		"(?P<Name>[a-zA-Z0-9-]*[a-zA-Z0-9])-" + // NAME
		"v(?P<Version>.*)$") // VERSION

// installingPluginRegexp matches the name of folders for plugins which are being installed. During installation
// we extract plugins to a folder with a suffix of `.tmpXXXXXX` (where `XXXXXX`) is a random number, from
// ioutil.TempFile. We should ignore these plugins as they have not yet been successfully installed.
var installingPluginRegexp = regexp.MustCompile(`\.tmp[0-9]+$`)

// tryPlugin returns true if a file is a plugin, and extracts information about it.
func tryPlugin(file os.FileInfo) (PluginKind, string, semver.Version, bool) {
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

type barCloser struct {
	bar        *pb.ProgressBar
	readCloser io.ReadCloser
}

func (bc *barCloser) Read(dest []byte) (int, error) {
	return bc.readCloser.Read(dest)
}

func (bc *barCloser) Close() error {
	bc.bar.Finish()
	return bc.readCloser.Close()
}
