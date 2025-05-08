// Copyright 2016-2024, Pulumi Corporation.
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
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/cheggaaa/pb"
	"github.com/djherbis/times"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/retry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/nodejs/npm"
	"github.com/pulumi/pulumi/sdk/v3/python/toolchain"
)

const (
	windowsGOOS = "windows"
)

var enableLegacyPluginBehavior = os.Getenv("PULUMI_ENABLE_LEGACY_PLUGIN_SEARCH") != ""

// pluginDownloadURLOverrides is a variable instead of a constant so it can be set using the `-X` `ldflag` at build
// time, if necessary. When non-empty, it's parsed into `pluginDownloadURLOverridesParsed` in `init()`. The expected
// format is `regexp=URL`, and multiple pairs can be specified separated by commas, e.g. `regexp1=URL1,regexp2=URL2`.
//
// For example, when set to "^https://foo=https://bar,^github://=https://buzz", HTTPS plugin URLs that start with
// "foo" will use https://bar as the download URL and plugins hosted on github will use https://buzz
//
// Note that named regular expression groups can be used to capture parts of URLs and then reused for building
// redirects. For example
// ^github://api.github.com/(?P<org>[^/]+)/(?P<repo>[^/]+)=https://foo.com/downloads/${org}/${repo}
// will capture any GitHub-hosted plugin and redirect to its corresponding folder under https://foo.com/downloads
var pluginDownloadURLOverrides string

// pluginDownloadURLOverridesParsed is the parsed array from `pluginDownloadURLOverrides`.
var pluginDownloadURLOverridesParsed pluginDownloadOverrideArray

// pluginDownloadURLOverride represents a plugin download URL override, parsed from `pluginDownloadURLOverrides`.
type pluginDownloadURLOverride struct {
	reg *regexp.Regexp // The regex used to match against the plugin's name.
	url string         // The URL to use for the matched plugin.
}

// pluginDownloadOverrideArray represents an array of overrides.
type pluginDownloadOverrideArray []pluginDownloadURLOverride

// get returns the URL and true if url matches an override's regular expression,
// otherwise an empty string and false. The input url may contain placeholders
// that match regular expression's named subgroups, the placeholders will be
// replaced by matches values.
func (overrides pluginDownloadOverrideArray) get(url string) (string, bool) {
	for _, override := range overrides {
		if match := override.reg.FindStringSubmatch(url); match != nil {
			result := override.url
			for i, name := range override.reg.SubexpNames() {
				// Replace placeholders that match a group index like $1
				placeholder := fmt.Sprintf("$%d", i)
				result = strings.ReplaceAll(result, placeholder, match[i])
				// Replace placeholders that match a group name like ${org}
				if i != 0 && name != "" {
					placeholder := fmt.Sprintf("${%s}", name)
					result = strings.ReplaceAll(result, placeholder, match[i])
				}
			}
			return result, true
		}
	}
	return "", false
}

func init() {
	// Default to Plugin Download URL overrides passed as compile-time flags (if any).
	overrides := pluginDownloadURLOverrides
	// Environment variable takes precedence over compile-time flags.
	if v := env.PluginDownloadURLOverrides.Value(); v != "" {
		overrides = v
	}
	// Parse overrides into a strongly-typed collection.
	var err error
	if pluginDownloadURLOverridesParsed, err = parsePluginDownloadURLOverrides(overrides); err != nil {
		panic(fmt.Errorf("error parsing `pluginDownloadURLOverrides`: %w", err))
	}
}

// parsePluginDownloadURLOverrides parses an overrides string with the expected format `regexp1=URL1,regexp2=URL2`.
func parsePluginDownloadURLOverrides(overrides string) (pluginDownloadOverrideArray, error) {
	splits := strings.Split(overrides, ",")
	result := make(pluginDownloadOverrideArray, 0, len(splits))
	if overrides == "" {
		return result, nil
	}
	for _, pair := range splits {
		split := strings.Split(pair, "=")
		if len(split) != 2 || split[0] == "" || split[1] == "" {
			return nil, fmt.Errorf("expected format to be \"regexp1=URL1,regexp2=URL2\"; got %q", overrides)
		}
		reg, err := regexp.Compile(split[0])
		if err != nil {
			return nil, err
		}
		result = append(result, pluginDownloadURLOverride{
			reg: reg,
			url: split[1],
		})
	}
	return result, nil
}

// MissingError is returned by functions that attempt to load plugins if a plugin can't be located.
type MissingError struct {
	// PluginSpec of the plugin that couldn't be found.
	spec PluginSpec
	// includeAmbient is true if we search $PATH for this plugin
	includeAmbient bool
}

// NewMissingError allocates a new error indicating the given plugin info was not found.
func NewMissingError(
	spec PluginSpec, includeAmbient bool,
) error {
	return &MissingError{
		spec:           spec,
		includeAmbient: includeAmbient,
	}
}

func (err *MissingError) Error() string {
	includePath := ""
	if err.includeAmbient {
		includePath = " or on your $PATH"
	}

	pluginDownloadURL := ""
	if err.spec.PluginDownloadURL != "" {
		pluginDownloadURL = " from " + err.spec.PluginDownloadURL
	}

	if err.spec.Version != nil {
		return fmt.Sprintf("no %[1]s plugin 'pulumi-%[1]s-%[2]s' found in the workspace at version v%[3]s%[4]s%[5]s",
			err.spec.Kind, err.spec.Name, err.spec.Version, includePath, pluginDownloadURL)
	}

	return fmt.Sprintf("no %[1]s plugin 'pulumi-%[1]s-%[2]s' found in the workspace%[3]s%[4]s",
		err.spec.Kind, err.spec.Name, includePath, pluginDownloadURL)
}

// PluginSource deals with downloading a specific version of a plugin, or looking up the latest version of it.
type PluginSource interface {
	// Download fetches an io.ReadCloser for this plugin and also returns the size of the response (if known).
	// The context supplied enables I/O to be canceled as needed.
	Download(ctx context.Context,
		version semver.Version, opSy string, arch string,
		getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error)) (io.ReadCloser, int64, error)

	// GetLatestVersion tries to find the latest version for this plugin. This is currently only supported for
	// plugins we can get from GitHub releases. The context supplied enables I/O to be canceled as needed.
	GetLatestVersion(ctx context.Context,
		getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error)) (*semver.Version, error)

	// A base URL that can uniquely identify the source. Has the same structure as the PluginDownloadURL
	// schema option. Example: "github://api.github.com/pulumi/pulumi-aws".
	URL() string
}

// standardAssetName returns the standard name for the asset that contains the given plugin.
func standardAssetName(name string, kind apitype.PluginKind, version semver.Version, opSy, arch string) string {
	return fmt.Sprintf("pulumi-%s-%s-v%s-%s-%s.tar.gz", kind, name, version, opSy, arch)
}

// getPulumiSource can download a plugin from get.pulumi.com
type getPulumiSource struct {
	name string
	kind apitype.PluginKind
}

func newGetPulumiSource(name string, kind apitype.PluginKind) *getPulumiSource {
	return &getPulumiSource{name: name, kind: kind}
}

func (source *getPulumiSource) Download(
	ctx context.Context,
	version semver.Version, opSy string, arch string,
	getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error),
) (io.ReadCloser, int64, error) {
	serverURL := "https://get.pulumi.com/releases/plugins"
	logging.V(1).Infof("%s downloading from %s", source.name, serverURL)
	endpoint := fmt.Sprintf("%s/%s",
		serverURL,
		url.QueryEscape(standardAssetName(source.name, source.kind, version, opSy, arch)))

	req, err := buildHTTPRequest(ctx, endpoint, "")
	if err != nil {
		return nil, -1, err
	}
	return getHTTPResponse(req)
}

// gitlabSource can download a plugin from gitlab releases.
type gitlabSource struct {
	host    string
	project string
	name    string
	kind    apitype.PluginKind

	token string
}

// Creates a new GitLab source from a gitlab://<host>/<project_id> url.
// Uses the GITLAB_TOKEN environment variable for authentication if it's set.
func newGitlabSource(url *url.URL, name string, kind apitype.PluginKind) (*gitlabSource, error) {
	contract.Requiref(url.Scheme == "gitlab", "url", `scheme must be "gitlab", was %q`, url.Scheme)

	host := url.Host
	if host == "" {
		return nil, fmt.Errorf("gitlab:// url must have a host part, was: %s", url)
	}

	project := strings.Trim(url.Path, "/")
	if project == "" || strings.Contains(project, "/") {
		return nil, fmt.Errorf(
			"gitlab:// url must have the format <host>/<project>, was: %s",
			url)
	}

	return &gitlabSource{
		host:    host,
		project: project,
		name:    name,
		kind:    kind,

		token: os.Getenv("GITLAB_TOKEN"),
	}, nil
}

func (source *gitlabSource) newHTTPRequest(ctx context.Context, url, accept string) (*http.Request, error) {
	var authorization string
	if source.token != "" {
		authorization = "Bearer " + source.token
	}

	req, err := buildHTTPRequest(ctx, url, authorization)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	return req, nil
}

func (source *gitlabSource) GetLatestVersion(
	ctx context.Context,
	getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error),
) (*semver.Version, error) {
	releaseURL := fmt.Sprintf(
		"https://%s/api/v4/projects/%s/releases/permalink/latest",
		source.host, source.project)
	logging.V(9).Infof("plugin GitLab releases url: %s", releaseURL)
	req, err := source.newHTTPRequest(ctx, releaseURL, "application/json")
	if err != nil {
		return nil, err
	}
	resp, length, err := getHTTPResponse(req)
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(resp)

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err = json.NewDecoder(resp).Decode(&release); err != nil {
		return nil, fmt.Errorf("cannot decode gitlab response len(%d): %w", length, err)
	}

	parsedVersion, err := semver.ParseTolerant(release.TagName)
	if err != nil {
		return nil, fmt.Errorf("invalid plugin version %s: %w", release.TagName, err)
	}
	return &parsedVersion, nil
}

func (source *gitlabSource) Download(
	ctx context.Context,
	version semver.Version, opSy string, arch string,
	getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error),
) (io.ReadCloser, int64, error) {
	assetName := standardAssetName(source.name, source.kind, version, opSy, arch)

	assetURL := fmt.Sprintf(
		"https://%s/api/v4/projects/%s/releases/v%s/downloads/%s",
		source.host, source.project, version, assetName)
	logging.V(1).Infof("%s downloading from %s", source.name, assetURL)

	req, err := source.newHTTPRequest(ctx, assetURL, "application/octet-stream")
	if err != nil {
		return nil, -1, err
	}
	return getHTTPResponse(req)
}

func (source *gitlabSource) URL() string {
	return fmt.Sprintf("gitlab://%s/%s", source.host, source.project)
}

func isPreReleaseVersion(version semver.Version) bool {
	return version.Major == 0 && version.Minor == 0 && version.Patch == 0 && len(version.Pre) > 0
}

// gitSource is used to download a plugin from a git repository.
type gitSource struct {
	// The git url to clone the plugin from.
	url string
	// The path within the repo to the plugin.
	path string

	// function to clone and checkout the repo.  Used so gitutil.GitCloneAndCheckoutCommit can be mocked in tests.
	cloneAndCheckoutRevision func(context.Context, string, plumbing.Revision, string) error
	// function to clone or pull the repo.  Used so gitutil.GitCloneOrPull can be mocked in tests.
	cloneOrPull func(context.Context, string, plumbing.ReferenceName, string, bool) error
}

func newGitHTTPSSource(url *url.URL) (*gitSource, error) {
	url.Scheme = "https"
	u, path, err := gitutil.ParseGitRepoURL(url.String())
	if err != nil {
		return nil, err
	}
	return &gitSource{
		url:                      u,
		path:                     path,
		cloneAndCheckoutRevision: gitutil.GitCloneAndCheckoutRevision,
		cloneOrPull:              gitutil.GitCloneOrPull,
	}, nil
}

func (source *gitSource) GetLatestVersion(
	ctx context.Context, _ func(*http.Request) (io.ReadCloser, int64, error),
) (*semver.Version, error) {
	return gitutil.GetLatestTagOrHash(ctx, source.url)
}

// Downloads a plugin from a git repository.  If the version is a pre-release version, the version is expected to be
// a commit hash, that will be checked out.  Otherwise, the version is expected to be a tag, that will be checked out.
// The tag is expected to be prefixed with a 'v' character.
// If the version is the special sentinel version 0.0.0, we'll use the latest commit on the default branch.
func (source *gitSource) Download(
	ctx context.Context, version semver.Version, _ string, _ string,
	_ func(*http.Request) (io.ReadCloser, int64, error),
) (io.ReadCloser, int64, error) {
	tmpdir, err := os.MkdirTemp("", "pulumi-plugin")
	if err != nil {
		return nil, -1, err
	}
	defer os.RemoveAll(tmpdir)
	if isPreReleaseVersion(version) {
		if len(version.Pre) != 1 {
			return nil, -1, fmt.Errorf("invalid version %s", version)
		}
		// The version string is prefixed with a 'x' character because Pre-versions can't
		// start with a 0. Strip that off to get the actual hash.  Note that we allow short
		// hashes, so we need to create a go-git revision that's then resolved to a full hash.
		revision := plumbing.Revision(version.Pre[0].VersionStr[1:])
		err := source.cloneAndCheckoutRevision(ctx, source.url, revision, tmpdir)
		if err != nil {
			return nil, -1, err
		}
	} else {
		var ref plumbing.ReferenceName
		if version.Major == 0 && version.Minor == 0 && version.Patch == 0 {
			ref = plumbing.HEAD
		} else {
			ref = plumbing.ReferenceName("refs/tags/v" + version.String())
		}
		err := source.cloneOrPull(ctx, source.url, ref, tmpdir, true /* shallow */)
		if err != nil {
			return nil, -1, err
		}
	}

	tarbuf := bytes.NewBuffer([]byte{})
	zip := gzip.NewWriter(tarbuf)

	tw := tar.NewWriter(zip)

	err = tw.AddFS(os.DirFS(tmpdir))
	if err != nil {
		return nil, -1, err
	}

	// These close statements cannot be deferred, because expressions in return statements
	// are executed before defer statements in Go.  Closing the writers in defer statements
	// can lead to the tarbuf.Len() return value to be wrong.
	tw.Close()
	zip.Close()

	return io.NopCloser(tarbuf), int64(tarbuf.Len()), nil
}

func (source *gitSource) URL() string {
	return source.url
}

// githubSource can download a plugin from github releases
type githubSource struct {
	host         string
	organization string
	repository   string
	name         string
	kind         apitype.PluginKind

	token string

	// If true we explicitly disabled the token due to 401 errors and won't try it again
	tokenDisabled bool
}

// Creates a new github source adding authentication data in the environment, if it exists
func newGithubSource(url *url.URL, name string, kind apitype.PluginKind) (*githubSource, error) {
	contract.Requiref(url.Scheme == "github", "url", `scheme must be "github", was %q`, url.Scheme)

	// 14-03-2022 we stopped looking at GITHUB_PERSONAL_ACCESS_TOKEN and sending basic auth for github and
	// instead just look at GITHUB_TOKEN and send in a header. Given GITHUB_PERSONAL_ACCESS_TOKEN was an
	// envvar we made up we check to see if it's set here and log a warning. This can be removed after a few
	// releases.
	if os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN") != "" {
		logging.Warningf("GITHUB_PERSONAL_ACCESS_TOKEN is no longer used for Github authentication, set GITHUB_TOKEN instead")
	}

	host := url.Host
	parts := strings.Split(strings.Trim(url.Path, "/"), "/")

	if host == "" {
		return nil, fmt.Errorf("github:// url must have a host part, was: %s", url)
	}

	if len(parts) != 1 && len(parts) != 2 {
		return nil, fmt.Errorf(
			"github:// url must have the format <host>/<organization>[/<repository>], was: %s",
			url)
	}

	organization := parts[0]
	if organization == "" {
		return nil, fmt.Errorf(
			"github:// url must have the format <host>/<organization>[/<repository>], was: %s",
			url)
	}

	repository := "pulumi-" + name
	if kind == apitype.ConverterPlugin {
		// Converter plugins are expected at a different repo path, e.g.
		// github.com/pulumi/pulumi-converter-aws rather than github.com/pulumi/pulumi-aws which would clash
		// with the providers of the same name.
		repository = "pulumi-converter-" + name
		if name == "yaml" {
			// We special case the yaml converter plugin to be in the pulumi-yaml repo. It's not ideal but its
			// to have this hardcoded here than having to deal with two repos for YAML, and long term this
			// should go away and be replaced with a registry lookup.
			repository = "pulumi-yaml"
		}
	}

	if kind == apitype.ToolPlugin {
		// Convention for tool plugins is to have them in a repository named "pulumi-tool-<name>".
		if !strings.HasPrefix(name, "pulumi-tool-") {
			// if it is not already fully qualified, we will prefix it with "pulumi-tool-"
			repository = "pulumi-tool-" + name
		}
	}

	if len(parts) == 2 {
		repository = parts[1]
	}

	return &githubSource{
		host:         host,
		organization: organization,
		repository:   repository,
		name:         name,
		kind:         kind,

		token: os.Getenv("GITHUB_TOKEN"),
	}, nil
}

func (source *githubSource) newHTTPRequest(ctx context.Context, url, accept string) (*http.Request, error) {
	var authorization string
	if source.token != "" {
		authorization = "token " + source.token
	}

	req, err := buildHTTPRequest(ctx, url, authorization)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	return req, nil
}

func isRateLimitError(downErr *downloadError) bool {
	return downErr.code == 403 && downErr.header.Get("x-ratelimit-remaining") == "0"
}

func (source *githubSource) getHTTPResponse(
	ctx context.Context,
	getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error),
	url, accept string,
) (io.ReadCloser, int64, error) {
	req, err := source.newHTTPRequest(ctx, url, accept)
	if err != nil {
		return nil, -1, err
	}
	resp, length, err := getHTTPResponse(req)
	if err == nil {
		return resp, length, nil
	}

	// We handle error responoses differently here based on the type:
	// - 403 errors that are also rate limit errors (x-ratelimit-remaining is 0) are wrapped in a more helpful message
	//   later.
	// - If the user has a GITHUB_TOKEN set, 401s and 403s (that are not rate limit errors) are handled by disabling
	//   the token
	//   and trying again.  This can be useful for example if the user has a fine grained token, which when set doesn't
	//   allow access to public repositories.
	// All other errors are returned as is.
	var downErr *downloadError
	if !errors.As(err, &downErr) || !isRateLimitError(downErr) {
		// If we see a 401 or 403 error and we're using a token we'll disable that token and try again
		if downErr != nil && (downErr.code == 401 || downErr.code == 403) && source.token != "" {
			source.token = ""
			source.tokenDisabled = true
			return source.getHTTPResponse(ctx, getHTTPResponse, url, accept)
		}
		return nil, -1, err
	}

	tryAgain := "."
	if reset, err := strconv.ParseInt(downErr.header.Get("x-ratelimit-reset"), 10, 64); err == nil {
		delay := time.Until(time.Unix(reset, 0).UTC())
		tryAgain = fmt.Sprintf(", try again in %s.", delay)
	}

	addAuth := ""
	if source.token == "" {
		if source.tokenDisabled {
			addAuth = " Your current GITHUB_TOKEN doesn't allow access to the repository, so we disabled it for this request." +
				" You can set GITHUB_TOKEN to a different token to make a request with a higher rate limit."
		} else {
			addAuth = " You can set GITHUB_TOKEN to make an authenticated request with a higher rate limit."
		}
	}

	logging.Errorf("GitHub rate limit exceeded for %s%s%s", req.URL, tryAgain, addAuth)
	return nil, -1, fmt.Errorf("rate limit exceeded: %w", err)
}

func (source *githubSource) GetLatestVersion(
	ctx context.Context,
	getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error),
) (*semver.Version, error) {
	releaseURL := fmt.Sprintf(
		"https://%s/repos/%s/%s/releases/latest",
		source.host, source.organization, source.repository)
	logging.V(9).Infof("plugin GitHub releases url: %s", releaseURL)
	resp, length, err := source.getHTTPResponse(ctx, getHTTPResponse, releaseURL, "application/json")
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(resp)

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err = json.NewDecoder(resp).Decode(&release); err != nil {
		return nil, fmt.Errorf("cannot decode github response len(%d): %w", length, err)
	}

	parsedVersion, err := semver.ParseTolerant(release.TagName)
	if err != nil {
		return nil, fmt.Errorf("invalid plugin version %s: %w", release.TagName, err)
	}
	return &parsedVersion, nil
}

func (source *githubSource) Download(
	ctx context.Context,
	version semver.Version, opSy string, arch string,
	getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error),
) (io.ReadCloser, int64, error) {
	releaseURL := fmt.Sprintf(
		"https://%s/repos/%s/%s/releases/tags/v%s",
		source.host, source.organization, source.repository, version)
	logging.V(9).Infof("plugin GitHub releases url: %s", releaseURL)
	resp, length, err := source.getHTTPResponse(ctx, getHTTPResponse, releaseURL, "application/json")
	if err != nil {
		return nil, -1, err
	}
	defer contract.IgnoreClose(resp)

	var release struct {
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"assets"`
	}
	if err = json.NewDecoder(resp).Decode(&release); err != nil {
		return nil, -1, fmt.Errorf("cannot decode github response len(%d): %w", length, err)
	}

	assetName := standardAssetName(source.name, source.kind, version, opSy, arch)
	assetURL := ""
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			assetURL = asset.URL
		}
	}
	if assetURL == "" {
		logging.V(9).Infof("github response: %v", release)
		logging.V(9).Infof("plugin asset '%s' not found", assetName)
		return nil, -1, fmt.Errorf("plugin asset '%s' not found", assetName)
	}

	logging.V(1).Infof("%s downloading from %s", source.name, assetURL)
	return source.getHTTPResponse(ctx, getHTTPResponse, assetURL, "application/octet-stream")
}

func (source *githubSource) URL() string {
	return fmt.Sprintf("github://%s/%s/%s", source.host, source.organization, source.repository)
}

// httpSource can download a plugin from a given http url, it doesn't support GetLatestVersion
type httpSource struct {
	name string
	kind apitype.PluginKind
	url  string
}

func newHTTPSource(name string, kind apitype.PluginKind, url *url.URL) *httpSource {
	contract.Requiref(
		url.Scheme == "http" || url.Scheme == "https",
		"url", `scheme must be "http" or "https", was %q`, url.Scheme)

	return &httpSource{
		name: name,
		kind: kind,
		url:  url.String(),
	}
}

func (source *httpSource) GetLatestVersion(
	ctx context.Context,
	getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error),
) (*semver.Version, error) {
	return nil, errors.New("GetLatestVersion is not supported for plugins from http sources")
}

func interpolateURL(serverURL string, name string, version semver.Version, os, arch string) string {
	// Expectation is the URL is already escaped, so we need to escape the {}'s in the replacement strings.
	replacer := strings.NewReplacer(
		"$%7BNAME%7D", url.QueryEscape(name),
		"$%7BVERSION%7D", url.QueryEscape(version.String()),
		"$%7BOS%7D", url.QueryEscape(os),
		"$%7BARCH%7D", url.QueryEscape(arch))
	return replacer.Replace(serverURL)
}

func (source *httpSource) Download(
	ctx context.Context,
	version semver.Version, opSy string, arch string,
	getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error),
) (io.ReadCloser, int64, error) {
	serverURL := interpolateURL(source.url, source.name, version, opSy, arch)
	serverURL = strings.TrimSuffix(serverURL, "/")
	logging.V(1).Infof("%s downloading from %s", source.name, serverURL)

	endpoint := fmt.Sprintf("%s/%s",
		serverURL,
		url.QueryEscape(fmt.Sprintf("pulumi-%s-%s-v%s-%s-%s.tar.gz", source.kind, source.name, version, opSy, arch)))

	req, err := buildHTTPRequest(ctx, endpoint, "")
	if err != nil {
		return nil, -1, err
	}
	return getHTTPResponse(req)
}

func (source *httpSource) URL() string {
	return source.url
}

// fallbackSource handles our current default logic of trying the pulumi public github then get.pulumi.com.
type fallbackSource struct {
	name string
	kind apitype.PluginKind
}

func newFallbackSource(name string, kind apitype.PluginKind) *fallbackSource {
	return &fallbackSource{
		name: name,
		kind: kind,
	}
}

func urlMustParse(rawURL string) *url.URL {
	url, err := url.Parse(rawURL)
	contract.AssertNoErrorf(err, "url.Parse(%q)", rawURL)
	return url
}

func (source *fallbackSource) GetLatestVersion(
	ctx context.Context,
	getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error),
) (*semver.Version, error) {
	// Try and get this package from our public pulumi github
	public, err := newGithubSource(urlMustParse("github://api.github.com/pulumi"), source.name, source.kind)
	if err != nil {
		return nil, err
	}
	version, err := public.GetLatestVersion(ctx, getHTTPResponse)
	if err != nil {
		return nil, err
	}

	return version, nil
}

func (source *fallbackSource) Download(
	ctx context.Context,
	version semver.Version, opSy string, arch string,
	getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error),
) (io.ReadCloser, int64, error) {
	// Try and get this package from public pulumi github
	public, err := newGithubSource(urlMustParse("github://api.github.com/pulumi"), source.name, source.kind)
	if err != nil {
		return nil, -1, err
	}
	resp, length, err := public.Download(ctx, version, opSy, arch, getHTTPResponse)
	if err == nil {
		return resp, length, nil
	}
	logging.Infof("Failed to download from GitHub, falling back to get.pulumi.com: %v", err)

	// Fallback to get.pulumi.com
	pulumi := newGetPulumiSource(source.name, source.kind)
	return pulumi.Download(ctx, version, opSy, arch, getHTTPResponse)
}

func (source *fallbackSource) URL() string {
	public, err := newGithubSource(urlMustParse("github://api.github.com/pulumi"), source.name, source.kind)
	if err != nil {
		return ""
	}
	return public.URL()
}

type checksumError struct {
	expected []byte
	actual   []byte
}

func (err *checksumError) Error() string {
	return fmt.Sprintf("invalid checksum, expected %x, actual %x", err.expected, err.actual)
}

// checksumSource will validate that the archive downloaded from the inner source matches a checksum
type checksumSource struct {
	source   PluginSource
	checksum map[string][]byte
}

func newChecksumSource(source PluginSource, checksum map[string][]byte) *checksumSource {
	return &checksumSource{
		source:   source,
		checksum: checksum,
	}
}

func (source *checksumSource) GetLatestVersion(
	ctx context.Context,
	getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error),
) (*semver.Version, error) {
	return source.source.GetLatestVersion(ctx, getHTTPResponse)
}

type checksumReader struct {
	checksum []byte
	io       io.ReadCloser
	hasher   hash.Hash
}

func (reader *checksumReader) Read(p []byte) (int, error) {
	n, err := reader.io.Read(p)
	if err != nil {
		if err == io.EOF {
			// Check the checksum matches
			actualChecksum := reader.hasher.Sum(nil)
			if !bytes.Equal(reader.checksum, actualChecksum) {
				return n, &checksumError{expected: reader.checksum, actual: actualChecksum}
			}
		}
		return n, err
	}

	m, err := reader.hasher.Write(p[0:n])
	contract.AssertNoErrorf(err, "error hashing input")
	contract.Assertf(m == n, "wrote %d bytes, expected %d", m, n)

	return n, nil
}

func (reader *checksumReader) Close() error {
	return reader.io.Close()
}

func (source *checksumSource) Download(
	ctx context.Context,
	version semver.Version, opSy string, arch string,
	getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error),
) (io.ReadCloser, int64, error) {
	checksum, ok := source.checksum[fmt.Sprintf("%s-%s", opSy, arch)]
	response, length, err := source.source.Download(ctx, version, opSy, arch, getHTTPResponse)
	if err != nil {
		return nil, -1, err
	}
	// If there's no checksum for this platform then skip validation.
	if !ok {
		return response, length, nil
	}

	return &checksumReader{
		checksum: checksum,
		hasher:   sha256.New(),
		io:       response,
	}, length, nil
}

func (source *checksumSource) URL() string {
	return source.source.URL()
}

// ProjectPlugin Information about a locally installed plugin specified by the project.
type ProjectPlugin struct {
	Name    string             // the simple name of the plugin.
	Kind    apitype.PluginKind // the kind of the plugin (language, resource, etc).
	Version *semver.Version    // the plugin's semantic version, if present.
	Path    string             // the path that a plugin is to be loaded from (this will always be a directory)
}

// Spec Return a PluginSpec object for this project plugin.
func (pp ProjectPlugin) Spec() (PluginSpec, error) {
	return NewPluginSpec(pp.Name, pp.Kind, pp.Version, "", nil)
}

// A PackageDescriptor specifies a package: the source PluginSpec that provides it, and any parameterization
// that must be applied to that plugin in order to produce the package.
type PackageDescriptor struct {
	// A specification for the plugin that provides the package.
	PluginSpec

	// An optional parameterization to apply to the providing plugin to produce
	// the package.
	Parameterization *Parameterization
}

func NewPackageDescriptor(spec PluginSpec, parameterization *Parameterization) PackageDescriptor {
	return PackageDescriptor{
		PluginSpec:       spec,
		Parameterization: parameterization,
	}
}

// PackageName returns the name of the package.
func (pd PackageDescriptor) PackageName() string {
	if pd.Parameterization != nil {
		return pd.Parameterization.Name
	}
	return pd.Name
}

// PackageVersion returns the version of the package.
func (pd PackageDescriptor) PackageVersion() *semver.Version {
	if pd.Parameterization != nil {
		return &pd.Parameterization.Version
	}
	return pd.Version
}

func (pd PackageDescriptor) String() string {
	name := pd.Name
	version := pd.Version
	if pd.Parameterization != nil {
		name = pd.Parameterization.Name
		version = &pd.Parameterization.Version
	}

	var v string
	if version != nil {
		v = fmt.Sprintf("-%s", version)
	}
	return name + v
}

// SortPackageDescriptors is a comparison function for PackageDescriptors that sorts by version, with version-less
// package descriptors appearing first. It returns -1 if the first argument should appear before the second, 0 if they
// are equal, and 1 if the first argument should appear after the second. Designed for use with functions like
// slices.SortFunc.
func SortPackageDescriptors(x PackageDescriptor, y PackageDescriptor) int {
	vx := x.PackageVersion()
	vy := y.PackageVersion()

	switch {
	case vx == nil && vy == nil:
		// Both packages lack any version. Consider them equal.
		return 0
	case vx == nil:
		// x has no version. We want version-less packages to come first, so sort x before y (-1).
		return -1
	case vy == nil:
		// y has no version. We want version-less packages to come first, so sort x after y (1).
		return 1
	default:
		// We have two versions. Compare them using semver.
		semverComparison := vx.Compare(*vy)
		switch semverComparison {
		case 0:
			// The versions are semantically equal. We'll compare their string representations to break ties (e.g. between 1.0.0
			// and 1.0.0+meta).
			return strings.Compare(vx.String(), vy.String())
		default:
			// The versions are not equal. Sort them using semver comparison.
			return semverComparison
		}
	}
}

// A Parameterization may be applied to a supporting plugin to yield a package.
type Parameterization struct {
	// The name of the package that will be produced by the parameterization.
	Name string
	// The version of the package that will be produced by the parameterization.
	Version semver.Version
	// A plugin-dependent bytestring representing the value of the parameter to be
	// passed to the plugin.
	Value []byte
}

// PluginSpec provides basic specification for a plugin.
type PluginSpec struct {
	Name              string             // the simple name of the plugin.
	Kind              apitype.PluginKind // the kind of the plugin (language, resource, etc).
	Version           *semver.Version    // the plugin's semantic version, if present.
	PluginDownloadURL string             // an optional server to use when downloading this plugin.
	PluginDir         string             // if set, will be used as the root plugin dir instead of ~/.pulumi/plugins.

	// if set will be used to validate the plugin downloaded matches. This is keyed by "$os-$arch", e.g. "linux-x64".
	Checksums map[string][]byte
}

type PluginVersionNotFoundError error

var urlRegex = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`^[^\./].*\.[a-z]+/[a-zA-Z0-9-/]*[a-zA-Z0-9/](@.*)?$`)
})

// Allow sha1 and sha256 hashes.
var gitCommitHashRegex = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`^[0-9a-fA-F]{4,64}$`)
})

func NewPluginSpec(
	source string,
	kind apitype.PluginKind,
	version *semver.Version,
	pluginDownloadURL string,
	checksums map[string][]byte,
) (PluginSpec, error) {
	spec, inference, err := parsePluginSpec(context.Background(), source, kind)
	if err != nil {
		return spec, err
	}

	if version != nil {
		if inference.explicitVersion {
			return PluginSpec{}, errors.New("cannot specify a version when the version is part of the name")
		}
		spec.Version = version
	}

	if pluginDownloadURL != "" {
		if inference.explicitPluginDownloadURL {
			return PluginSpec{}, errors.New("cannot specify a plugin download URL when the plugin name is a URL")
		}
		spec.PluginDownloadURL = pluginDownloadURL
	}
	spec.Checksums = checksums

	return spec, nil
}

type parsePluginSpecInference struct {
	explicitVersion           bool
	explicitPluginDownloadURL bool
}

func parsePluginSpec(
	ctx context.Context, source string, kind apitype.PluginKind,
) (PluginSpec, parsePluginSpecInference, error) {
	if strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "git://") || urlRegex().MatchString(source) {
		return parsePluginSpecFromURL(ctx, source, kind)
	}

	return parsePluginSpecFromName(ctx, source, kind)
}

func (inference *parsePluginSpecInference) parseVersion(spec string, parse func(version string) error) (string, error) {
	if i := strings.LastIndexByte(spec, '@'); i >= 0 {
		inference.explicitVersion = true
		return spec[:i], parse(spec[i+1:])
	}
	return spec, nil
}

func parsePluginSpecFromURL(
	ctx context.Context, spec string, kind apitype.PluginKind,
) (PluginSpec, parsePluginSpecInference, error) {
	// Parse the version if available.  This can either be a simple semver version, or a git commit hash.
	var version *semver.Version
	var inference parsePluginSpecInference
	spec, err := inference.parseVersion(spec, func(versionStr string) error {
		v, err := semver.ParseTolerant(versionStr)
		if err == nil {
			version = &v
			return nil
		}
		if gitCommitHashRegex().MatchString(versionStr) {
			version = &semver.Version{
				// VersionStr cannot start with a 0, so we prefix it with an 'x' to avoid this.
				Pre: []semver.PRVersion{{VersionStr: "x" + versionStr}},
			}
			return nil
		}
		return fmt.Errorf("VERSION must be valid semver or git commit hash: %s", versionStr)
	})
	if err != nil {
		return PluginSpec{}, inference, err
	}

	parsedURL, err := url.Parse(spec)
	if err != nil {
		return PluginSpec{}, inference, fmt.Errorf("invalid URL: %w", err)
	}
	switch parsedURL.Scheme {
	case "git", "https", "":
		parsedURL.Scheme = "https"
	default:
		return PluginSpec{}, inference, errors.New(`unknown URL scheme: expected "git" or "https"`)
	}

	gitURL, _, err := gitutil.ParseGitRepoURL(parsedURL.String())
	if err != nil {
		return PluginSpec{}, inference, err
	}
	pluginSpec := PluginSpec{
		Name:    strings.ReplaceAll(strings.TrimPrefix(gitURL, "https://"), "/", "_"),
		Kind:    kind,
		Version: version,
		// Prefix the url with `git://`, so we can later recognize this as a git URL.
		PluginDownloadURL: func(url url.URL) string { url.Scheme = "git"; return url.String() }(*parsedURL),
	}
	inference.explicitPluginDownloadURL = true

	// If there is no version specified, we version the plugin ourselves. This way the user gets
	// a consistent experience once the plugin is installed, and won't have any problems when the repo
	// is updated.  The version will then be added to the plugins SDK, and will be reused when the NewPluginSpec
	// is used, so the user gets a consistent experience.
	if pluginSpec.Version == nil {
		var err error
		pluginSpec.Version, err = gitutil.GetLatestTagOrHash(ctx, gitURL)
		if err != nil {
			return pluginSpec, inference, PluginVersionNotFoundError(err)
		}
	}

	return pluginSpec, inference, nil
}

func parsePluginSpecFromName(
	_ context.Context, spec string, kind apitype.PluginKind,
) (PluginSpec, parsePluginSpecInference, error) {
	var version *semver.Version
	var inference parsePluginSpecInference
	// Parse the version if available.  This must be a simple semver version.
	spec, err := inference.parseVersion(spec, func(versionStr string) error {
		v, err := semver.ParseTolerant(versionStr)
		if err != nil {
			return fmt.Errorf("VERSION must be valid semver: %s", versionStr)
		}
		version = &v
		return nil
	})

	return PluginSpec{
		Name:    spec,
		Kind:    kind,
		Version: version,
	}, inference, err
}

// IsGitPlugin returns if the plugin comes from the git source
func (spec PluginSpec) IsGitPlugin() bool {
	return strings.HasPrefix(spec.PluginDownloadURL, "git://")
}

// LocalName returns the local name of the plugin, which is used in the directory name, and a path
// within that directory if the plugin is located in a subdirectory.
func (spec PluginSpec) LocalName() (string, string) {
	if spec.IsGitPlugin() {
		trimmed := strings.TrimPrefix(spec.PluginDownloadURL, "git://")
		url, path, err := gitutil.ParseGitRepoURL("https://" + strings.TrimPrefix(trimmed, "git://"))
		if err != nil {
			return strings.ReplaceAll(trimmed, "/", "_"), ""
		}
		return strings.ReplaceAll(strings.TrimPrefix(url, "https://"), "/", "_"), path
	}
	return spec.Name, ""
}

// Dir gets the expected plugin directory for this plugin.
func (spec PluginSpec) Dir() string {
	localName, _ := spec.LocalName()
	dir := fmt.Sprintf("%s-%s", spec.Kind, localName)
	if spec.Version != nil {
		dir = fmt.Sprintf("%s-v%s", dir, spec.Version.String())
	}
	return dir
}

// SubDir gets the expected subdirectory for this plugin.
func (spec PluginSpec) SubDir() string {
	_, path := spec.LocalName()
	return path
}

// File gets the expected filename for this plugin, excluding any platform specific suffixes (e.g. ".exe" on
// windows).
func (spec PluginSpec) File() string {
	return fmt.Sprintf("pulumi-%s-%s", spec.Kind, spec.Name)
}

// DirPath returns the directory where this plugin should be installed.
func (spec PluginSpec) DirPath() (string, error) {
	var err error
	dir := spec.PluginDir
	if dir == "" {
		dir, err = GetPluginDir()
		if err != nil {
			return "", err
		}
	}

	return filepath.Join(dir, spec.Dir()), nil
}

// LockFilePath returns the full path to the plugin's lock file used during installation
// to prevent concurrent installs.
func (spec PluginSpec) LockFilePath() (string, error) {
	dir, err := spec.DirPath()
	if err != nil {
		return "", err
	}
	return dir + ".lock", nil
}

// PartialFilePath returns the full path to the plugin's partial file used during installation
// to indicate installation of the plugin hasn't completed yet.
func (spec PluginSpec) PartialFilePath() (string, error) {
	dir, err := spec.DirPath()
	if err != nil {
		return "", err
	}
	return dir + ".partial", nil
}

func (spec PluginSpec) String() string {
	var version string
	if v := spec.Version; v != nil {
		version = fmt.Sprintf("-%s", v)
	}
	return spec.Name + version
}

// PluginInfo provides basic information about a plugin.  Each plugin gets installed into a system-wide
// location, by default `~/.pulumi/plugins/<kind>-<name>-<version>/`.  A plugin may contain multiple files,
// however the primary loadable executable must be named `pulumi-<kind>-<name>`.
type PluginInfo struct {
	Name         string             // the simple name of the plugin.
	Path         string             // the path that a plugin was loaded from (this will always be a directory)
	Kind         apitype.PluginKind // the kind of the plugin (language, resource, etc).
	Version      *semver.Version    // the plugin's semantic version, if present.
	Size         uint64             // the size of the plugin, in bytes.
	InstallTime  time.Time          // the time the plugin was installed.
	LastUsedTime time.Time          // the last time the plugin was used.
	SchemaPath   string             // if set, used as the path for loading and caching the schema
	SchemaTime   time.Time          // if set and newer than the file at SchemaPath, used to invalidate a cached schema
}

// Spec returns the PluginSpec for this PluginInfo
func (info *PluginInfo) Spec() PluginSpec {
	return PluginSpec{Name: info.Name, Kind: info.Kind, Version: info.Version}
}

func (info PluginInfo) String() string {
	var version string
	if v := info.Version; v != nil {
		version = fmt.Sprintf("-%s", v)
	}
	return info.Name + version
}

// Delete removes the plugin from the cache.  It also deletes any supporting files in the cache, which includes
// any files that contain the same prefix as the plugin itself.
func (info *PluginInfo) Delete() error {
	dir := info.Path
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	// Attempt to delete any leftover .partial or .lock files.
	// Don't fail the operation if we can't delete these.
	contract.IgnoreError(os.Remove(dir + ".partial"))
	contract.IgnoreError(os.Remove(dir + ".lock"))
	return nil
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
	if err == nil {
		info.Size = size
	} else {
		logging.V(6).Infof("unable to get plugin dir size for %s: %v", path, err)
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

func newPluginSource(name string, kind apitype.PluginKind, pluginDownloadURL string) (PluginSource, error) {
	url, err := url.Parse(pluginDownloadURL)
	if err != nil {
		return nil, err
	}

	switch url.Scheme {
	case "github":
		return newGithubSource(url, name, kind)
	case "gitlab":
		return newGitlabSource(url, name, kind)
	case "http", "https":
		return newHTTPSource(name, kind, url), nil
	case "git":
		return newGitHTTPSSource(url)
	default:
		return nil, fmt.Errorf("unknown plugin source scheme: %s", url.Scheme)
	}
}

func (spec PluginSpec) GetSource() (PluginSource, error) {
	var source PluginSource
	var err error

	// The plugin has a set URL use that.
	if spec.PluginDownloadURL != "" {
		source, err = newPluginSource(spec.Name, spec.Kind, spec.PluginDownloadURL)
		if err != nil {
			return nil, err
		}
	} else {
		// Use our default fallback behaviour of github then get.pulumi.com
		source = newFallbackSource(spec.Name, spec.Kind)
	}

	// If the plugin URL matches an override, download the plugin from the override URL.
	if url, ok := pluginDownloadURLOverridesParsed.get(source.URL()); ok {
		source, err = newPluginSource(spec.Name, spec.Kind, url)
		if err != nil {
			return nil, err
		}
	}

	// Apply checksums if defined.
	if len(spec.Checksums) != 0 {
		source = newChecksumSource(source, spec.Checksums)
	}

	return source, nil
}

// GetLatestVersion tries to find the latest version for this plugin. This is currently only supported for
// plugins we can get from github releases. The context allows for I/O cancellation.
func (spec PluginSpec) GetLatestVersion(ctx context.Context) (*semver.Version, error) {
	source, err := spec.GetSource()
	if err != nil {
		return nil, err
	}
	return source.GetLatestVersion(ctx, getHTTPResponseWithRetry)
}

// Download fetches an io.ReadCloser for this plugin and also returns the size of the response (if known).
// The context allows for I/O cancellation.
func (spec PluginSpec) Download(ctx context.Context) (io.ReadCloser, int64, error) {
	// Figure out the OS/ARCH pair for the download URL.
	var opSy string
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		opSy = runtime.GOOS
	default:
		return nil, -1, fmt.Errorf("unsupported plugin OS: %s", runtime.GOOS)
	}
	var arch string
	switch runtime.GOARCH {
	case "amd64", "arm64":
		arch = runtime.GOARCH
	default:
		return nil, -1, fmt.Errorf("unsupported plugin architecture: %s", runtime.GOARCH)
	}

	// The plugin version is necessary for the endpoint. If it's not present, return an error.
	if spec.Version == nil {
		return nil, -1, fmt.Errorf("unknown version for plugin %s", spec.Name)
	}

	source, err := spec.GetSource()
	if err != nil {
		return nil, -1, err
	}
	return source.Download(ctx, *spec.Version, opSy, arch, getHTTPResponse)
}

func buildHTTPRequest(ctx context.Context, pluginEndpoint string, authorization string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pluginEndpoint, nil)
	if err != nil {
		return nil, err
	}

	userAgent := fmt.Sprintf("pulumi-cli/1 (%s; %s)", version.Version, runtime.GOOS)
	req.Header.Set("User-Agent", userAgent)

	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	return req, nil
}

func getHTTPResponse(req *http.Request) (io.ReadCloser, int64, error) {
	logging.V(9).Infof("full plugin download url: %s", req.URL)
	// This logs at level 11 because it could include authentication headers, we reserve log level 11 for
	// detailed api logs that may include credentials.
	logging.V(11).Infof("plugin install request headers: %v", req.Header)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, -1, err
	}

	// As above this might include authentication information, but also to be consistent at what level headers
	// print at.
	logging.V(11).Infof("plugin install response headers: %v", resp.Header)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		contract.IgnoreClose(resp.Body)
		return nil, -1, newDownloadError(resp.StatusCode, req.URL, resp.Header)
	}

	return resp.Body, resp.ContentLength, nil
}

func getHTTPResponseWithRetry(req *http.Request) (io.ReadCloser, int64, error) {
	logging.V(9).Infof("full plugin download url: %s", req.URL)
	// This logs at level 11 because it could include authentication headers, we reserve log level 11 for
	// detailed api logs that may include credentials.
	logging.V(11).Infof("plugin install request headers: %v", req.Header)

	resp, err := httputil.DoWithRetry(req, http.DefaultClient)
	if err != nil {
		return nil, -1, err
	}

	// As above this might include authentication information, but also to be consistent at what level headers
	// print at.
	logging.V(11).Infof("plugin install response headers: %v", resp.Header)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		contract.IgnoreClose(resp.Body)
		return nil, -1, newDownloadError(resp.StatusCode, req.URL, resp.Header)
	}

	return resp.Body, resp.ContentLength, nil
}

// downloadError is an error that happened during the HTTP download of a plugin.
type downloadError struct {
	msg    string
	code   int
	header http.Header
}

func (e *downloadError) Error() string {
	return e.msg
}

// Create a new downloadError with a message that indicates GITHUB_TOKEN should be set.
func newGithubPrivateRepoError(statusCode int, url *url.URL) error {
	return &downloadError{
		code: statusCode,
		msg: fmt.Sprintf("%d HTTP error fetching plugin from %s. "+
			"If this is a private GitHub repository, try "+
			"providing a token via the GITHUB_TOKEN environment variable. "+
			"See: https://github.com/settings/tokens",
			statusCode, url),
	}
}

// Create a new downloadError.
func newDownloadError(statusCode int, url *url.URL, header http.Header) error {
	if url.Host == "api.github.com" && statusCode == 404 {
		return newGithubPrivateRepoError(statusCode, url)
	}
	return &downloadError{
		code:   statusCode,
		msg:    fmt.Sprintf("%d HTTP error fetching plugin from %s", statusCode, url),
		header: header,
	}
}

// installLock acquires a file lock used to prevent concurrent installs.
func (spec PluginSpec) installLock() (unlock func(), err error) {
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

// Install installs a plugin's tarball into the cache. See InstallWithContext for details.
func (spec PluginSpec) Install(tgz io.ReadCloser, reinstall bool) error {
	return spec.InstallWithContext(context.Background(), tarPlugin{tgz}, reinstall)
}

// pluginDownloader is responsible for downloading plugins from PluginSpecs.
//
// It allows hooking into various stages of the download process
// to allow for custom behavior and progress reporting.
//
// All fields are optional.
type pluginDownloader struct {
	// WrapStream wraps the stream returned by the plugin source.
	// This is useful for things like reporting progress.
	WrapStream func(stream io.ReadCloser, size int64) io.ReadCloser

	// OnRetry receives a notification when a download fails
	// and is about to be retried.
	// err is the error that caused the retry.
	// attempt is the number of the attempt that failed (starting at 1).
	// limit is the maximum number of attempts.
	// delay is the amount of time that will be slept before the next attempt.
	// DO NOT sleep in this function. It's for observation only.
	OnRetry func(err error, attempt int, limit int, delay time.Duration)

	// Controls how to sleep between retries.
	After func(time.Duration) <-chan time.Time // == time.After
}

// copyBuffer copies from src to dst until either EOF is reached on src or an error occurs.
//
// This is an internal helper that's pretty much just a copy of io.Copy except it returns read and
// write errors separately. We only want to retry if the read (i.e. download) fails, if the write
// fails thats probably due to file permissions or space limitations and there's no point retrying.
func (d *pluginDownloader) copyBuffer(dst io.Writer, src io.Reader) (written int64, readErr error, writeErr error) {
	size := 32 * 1024
	if l, ok := src.(*io.LimitedReader); ok && int64(size) > l.N {
		if l.N < 1 {
			size = 1
		} else {
			size = int(l.N)
		}
	}
	buf := make([]byte, size)

	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				return written, nil, ew
			}
			if nr != nw {
				return written, nil, io.ErrShortWrite
			}
		}
		if er != nil {
			if er == io.EOF {
				er = nil
			}
			return written, er, nil
		}
	}
}

func (d *pluginDownloader) tryDownload(ctx context.Context, pkgPlugin PluginSpec, dst io.WriteCloser) (error, error) {
	defer dst.Close()
	tarball, expectedByteCount, err := pkgPlugin.Download(ctx)
	if err != nil {
		return err, nil
	}
	if d.WrapStream != nil {
		tarball = d.WrapStream(tarball, expectedByteCount)
	}
	defer tarball.Close()
	copiedByteCount, readErr, writerErr := d.copyBuffer(dst, tarball)
	if readErr != nil || writerErr != nil {
		return readErr, writerErr
	}
	if copiedByteCount != expectedByteCount {
		return nil, fmt.Errorf("expected %d bytes but copied %d when downloading plugin %s",
			expectedByteCount, copiedByteCount, pkgPlugin)
	}
	return nil, nil
}

func (d *pluginDownloader) tryDownloadToFile(ctx context.Context, pkgPlugin PluginSpec) (string, error, error) {
	file, err := os.CreateTemp("" /* default temp dir */, "pulumi-plugin-tar")
	if err != nil {
		return "", nil, err
	}
	readErr, writeErr := d.tryDownload(ctx, pkgPlugin, file)
	logging.V(10).Infof("try downloaded plugin %s to %s: %v %v", pkgPlugin, file.Name(), readErr, writeErr)
	if readErr != nil || writeErr != nil {
		err2 := os.Remove(file.Name())
		if err2 != nil {
			// only one of readErr or writeErr will be set
			err := readErr
			if err == nil {
				err = writeErr
			}

			return "", nil, fmt.Errorf("error while removing tempfile: %v. Context: %w", err2, err)
		}
		return "", readErr, writeErr
	}
	return file.Name(), nil, nil
}

func (d *pluginDownloader) downloadToFileWithRetry(ctx context.Context, pkgPlugin PluginSpec) (string, error) {
	delay := 80 * time.Millisecond
	backoff := 2.0
	maxAttempts := 5

	_, path, err := (&retry.Retryer{
		After: d.After,
	}).Until(context.Background(), retry.Acceptor{
		Delay:   &delay,
		Backoff: &backoff,
		Accept: func(attempt int, nextRetryTime time.Duration) (bool, interface{}, error) {
			if attempt >= maxAttempts {
				return false, nil, fmt.Errorf("failed all %d attempts", maxAttempts)
			}

			tempFile, readErr, writeErr := d.tryDownloadToFile(ctx, pkgPlugin)
			if readErr == nil && writeErr == nil {
				return true, tempFile, nil
			}
			if writeErr != nil {
				// Writes are local. If they fail,
				// there's no point retrying.
				return false, "", writeErr
			}

			// If the readErr is a checksum error don't retry.
			var checksumErr *checksumError
			if errors.As(readErr, &checksumErr) {
				return false, "", readErr
			}

			// Don't retry, since the request was processed and rejected.
			var downloadErr *downloadError
			if errors.As(readErr, &downloadErr) && (downloadErr.code == 404 || downloadErr.code == 403) {
				return false, "", readErr
			}

			if d.OnRetry != nil {
				d.OnRetry(readErr, attempt+1, maxAttempts, nextRetryTime)
			}

			return false, "", nil
		},
	})
	if err != nil {
		return "", err
	}

	return path.(string), nil
}

// DownloadToFile downloads the given PluginSpec to a temporary file and returns that temporary file. This has
// some retry logic to re-attempt the download if it errors for any reason.
func (d *pluginDownloader) DownloadToFile(ctx context.Context, pkgPlugin PluginSpec) (*os.File, error) {
	tarball, err := d.downloadToFileWithRetry(ctx, pkgPlugin)
	if err != nil {
		return nil, fmt.Errorf("failed to download plugin: %s: %w", pkgPlugin, err)
	}
	reader, err := os.Open(tarball)
	if err != nil {
		return nil, fmt.Errorf("failed to open downloaded plugin: %s: %w", pkgPlugin, err)
	}
	return reader, nil
}

// DownloadToFile downloads the given PluginInfo to a temporary file and returns that temporary file.
// This has some retry logic to re-attempt the download if it errors for any reason. The context supplied
// may be used for cancellable I/O.
func DownloadToFile(
	ctx context.Context,
	pkgPlugin PluginSpec,
	wrapper func(stream io.ReadCloser, size int64) io.ReadCloser,
	retry func(err error, attempt int, limit int, delay time.Duration),
) (*os.File, error) {
	return (&pluginDownloader{
		WrapStream: wrapper,
		OnRetry:    retry,
	}).DownloadToFile(ctx, pkgPlugin)
}

type PluginContent interface {
	io.Closer

	writeToDir(pathToDir string) error
}

func SingleFilePlugin(f *os.File, spec PluginSpec) PluginContent {
	return singleFilePlugin{F: f, Kind: spec.Kind, Name: spec.Name}
}

type singleFilePlugin struct {
	F    *os.File
	Kind apitype.PluginKind
	Name string
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
	return os.WriteFile(finalPath, bytes, 0o700) //nolint:gosec
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

// InstallWithContext installs a plugin's tarball into the cache. It validates that plugin names are in the expected
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
func (spec PluginSpec) InstallWithContext(ctx context.Context, content PluginContent, reinstall bool) error {
	defer contract.IgnoreClose(content)

	// Fetch the directory into which we will expand this tarball.
	finalDir, err := spec.DirPath()
	if err != nil {
		return err
	}

	// Create a file lock file at <pluginsdir>/<kind>-<name>-<version>.lock.
	unlock, err := spec.installLock()
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

func (spec PluginSpec) InstallDependencies(ctx context.Context) error {
	dir, err := spec.DirPath()
	if err != nil {
		return err
	}
	subdir := filepath.Join(dir, spec.SubDir())

	// Install dependencies, if needed.
	proj, err := LoadPluginProject(filepath.Join(subdir, "PulumiPlugin.yaml"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("loading PulumiPlugin.yaml: %w", err)
	}
	if proj != nil {
		runtime := strings.ToLower(proj.Runtime.Name())
		// For now, we only do this for Node.js and Python. For Go, the expectation is the binary is
		// already built. For .NET, similarly, a single self-contained binary could be used, but
		// otherwise `dotnet run` will implicitly run `dotnet restore`.
		// TODO[pulumi/pulumi#1334]: move to the language plugins so we don't have to hard code here.
		switch runtime {
		case "nodejs":
			var b bytes.Buffer
			if _, err := npm.Install(ctx, npm.AutoPackageManager, subdir, true /* production */, &b, &b); err != nil {
				os.Stderr.Write(b.Bytes())
				return fmt.Errorf("installing plugin dependencies: %w", err)
			}
		case "python":
			// We support two types of Python based plugins: bare directories or
			// buildable packages.
			//
			// If the plugin directory is a bare directory (that is not a Python
			// package), we expect it to have a `__main__.py` file that's the
			// entrypoint to the plugin. Dependencies must be specified in
			// `requirements.txt`.
			//
			// Plugins can also be Python packages, with a pyproject.toml. The
			// package needs to be buildable [1]. We expect the package to
			// include a module with the name of the project.
			//
			// See also sdk/python/cmd/pulumi-language-python/main.go#RunPlugin.
			//
			// 1: https://packaging.python.org/en/latest/tutorials/packaging-projects/#choosing-a-build-backend
			mainPy := filepath.Join(subdir, "__main__.py")
			_, err := os.Stat(mainPy)
			if os.IsNotExist(err) {
				// There is no `__main__.py` file. Check for pyproject.toml and if it's a buildable package.
				buildable, err := toolchain.IsBuildablePackage(subdir)
				if err != nil {
					return fmt.Errorf("checking if plugin is a buildable package: %w", err)
				}
				if !buildable {
					return errors.New("plugin is not runnable, it provides neither __main__.py nor a buildable pyproject.toml")
				}

				logging.V(6).Infof("Plugin at %s is a buildable Python package, installing it in package mode.", subdir)

				// We have a pyproject.toml and it's a buildable package. We'll create a virtualenv
				// and install the package into it.
				ambientTc, err := toolchain.ResolveToolchain(toolchain.PythonOptions{})
				if err != nil {
					return fmt.Errorf("getting ambient toolchain: %w", err)
				}
				cmd, err := ambientTc.ModuleCommand(ctx, "venv", "venv")
				cmd.Dir = subdir
				if err != nil {
					return fmt.Errorf("preparing venv command: %w", err)
				}
				if out, err := cmd.CombinedOutput(); err != nil {
					return fmt.Errorf("creating venv: %w; output %s", err, out)
				}

				tc, err := toolchain.ResolveToolchain(toolchain.PythonOptions{
					Toolchain:  toolchain.Pip,
					Root:       subdir,
					Virtualenv: "venv",
				})
				if err != nil {
					return fmt.Errorf("getting python toolchain: %w", err)
				}
				cmd, err = tc.ModuleCommand(ctx, "pip", "install", ".")
				cmd.Dir = subdir
				if err != nil {
					return fmt.Errorf("preparing pip install command: %w", err)
				}
				if out, err := cmd.CombinedOutput(); err != nil {
					return fmt.Errorf("installing package: %w; output %s", err, out)
				}
				return nil
			}

			// TODO[pulumi/pulumi/issues/16287]: Support toolchain options for installing plugins.
			tc, err := toolchain.ResolveToolchain(toolchain.PythonOptions{
				Toolchain:  toolchain.Pip,
				Root:       subdir,
				Virtualenv: "venv",
			})
			if err != nil {
				return fmt.Errorf("getting python toolchain: %w", err)
			}
			if err := tc.InstallDependencies(ctx, subdir, false, /*useLanguageVersionTools */
				false /*showOutput*/, os.Stdout, os.Stderr); err != nil {
				return fmt.Errorf("installing plugin dependencies: %w", err)
			}
		}
	}
	return nil
}

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

// Re exporting PluginKind to keep backward compatibility, this should be kept in sync with
// the definitions in sdk/go/common/apitype/plugins.go
//
// Deprecated: PluginKind type was moved to "github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
type PluginKind = apitype.PluginKind

// ProductKind types definition
//
// Deprecated: PluginKind type was moved to "github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
const (
	AnalyzerPlugin  = apitype.AnalyzerPlugin
	LanguagePlugin  = apitype.LanguagePlugin
	ResourcePlugin  = apitype.ResourcePlugin
	ConverterPlugin = apitype.ConverterPlugin
	ToolPlugin      = apitype.ToolPlugin
)

// IsPluginKind returns true if k is a valid plugin kind, and false otherwise.
//
// Deprecated: IsPluginKind type was moved to "github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
func IsPluginKind(k string) bool {
	return apitype.IsPluginKind(k)
}

// HasPlugin returns true if the given plugin exists.
func HasPlugin(spec PluginSpec) bool {
	dir, err := spec.DirPath()
	if err == nil {
		_, err := os.Stat(dir)
		if err == nil {
			partialFilePath, err := spec.PartialFilePath()
			if err == nil {
				if _, err := os.Stat(partialFilePath); os.IsNotExist(err) {
					return true
				}
			}
		}
	}
	return false
}

// HasPluginGTE returns true if the given plugin exists at the given version number or greater.
func HasPluginGTE(spec PluginSpec) (bool, error) {
	// If an exact match, return true right away.
	if HasPlugin(spec) {
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
	var match *PluginInfo
	if !enableLegacyPluginBehavior && spec.Version != nil {
		match = SelectCompatiblePlugin(plugs, spec)
	} else {
		match = LegacySelectCompatiblePlugin(plugs, spec)
	}
	return match != nil, nil
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

// GetPlugins returns a list of installed plugins without size info and last accessed metadata.
// Plugin size requires recursively traversing the plugin directory, which can be extremely
// expensive with the introduction of nodejs multilang components that have
// deeply nested node_modules folders.
func GetPlugins() ([]PluginInfo, error) {
	// To get the list of plugins, simply scan the directory in the usual place.
	dir, err := GetPluginDir()
	if err != nil {
		return nil, err
	}
	return getPlugins(dir, true /* skipMetadata */)
}

// GetPluginsWithMetadata returns a list of installed plugins with metadata about size,
// and last access (POOR RUNTIME PERF). Plugin size requires recursively traversing the
// plugin directory, which can be extremely expensive with the introduction of
// nodejs multilang components that have deeply nested node_modules folders.
func GetPluginsWithMetadata() ([]PluginInfo, error) {
	// To get the list of plugins, simply scan the directory in the usual place.
	dir, err := GetPluginDir()
	if err != nil {
		return nil, err
	}
	return getPlugins(dir, false /* skipMetadata */)
}

func getPlugins(dir string, skipMetadata bool) ([]PluginInfo, error) {
	files, err := os.ReadDir(dir)
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
			path := filepath.Join(dir, file.Name())
			plugin := PluginInfo{
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
			// computing plugin sizes can be very expensive (nested node_modules)
			if !skipMetadata {
				if err = plugin.SetFileMetadata(path); err != nil {
					return nil, err
				}
			}
			plugins = append(plugins, plugin)
		}
	}
	return plugins, nil
}

// We currently bundle some plugins with "pulumi" and thus expect them to be next to the pulumi binary.
// Eventually we want to fix this so new plugins are true plugins in the plugin cache.
func IsPluginBundled(kind apitype.PluginKind, name string) bool {
	return (kind == apitype.LanguagePlugin && name == "nodejs") ||
		(kind == apitype.LanguagePlugin && name == "go") ||
		(kind == apitype.LanguagePlugin && name == "python") ||
		(kind == apitype.LanguagePlugin && name == "dotnet") ||
		(kind == apitype.LanguagePlugin && name == "yaml") ||
		(kind == apitype.LanguagePlugin && name == "java") ||
		(kind == apitype.ResourcePlugin && name == "pulumi-nodejs") ||
		(kind == apitype.ResourcePlugin && name == "pulumi-python") ||
		(kind == apitype.AnalyzerPlugin && name == "policy") ||
		(kind == apitype.AnalyzerPlugin && name == "policy-python")
}

// GetPluginPath finds a plugin's path by its kind, name, and optional version.  It will match the latest version that
// is >= the version specified.  If no version is supplied, the latest plugin for that given kind/name pair is loaded,
// using standard semver sorting rules.  A plugin may be overridden entirely by placing it on your $PATH, though it is
// possible to opt out of this behavior by setting PULUMI_IGNORE_AMBIENT_PLUGINS to any non-empty value.
func GetPluginPath(d diag.Sink, spec PluginSpec, projectPlugins []ProjectPlugin,
) (string, error) {
	info, path, err := getPluginInfoAndPath(d, spec, true /* skipMetadata */, projectPlugins)
	if err != nil {
		return "", err
	}

	contract.Assertf(info.Path == filepath.Dir(path),
		"plugin executable (%v) is not inside plugin directory (%v)", path, info.Path)
	return path, err
}

func GetPluginInfo(d diag.Sink, spec PluginSpec, projectPlugins []ProjectPlugin,
) (*PluginInfo, error) {
	info, path, err := getPluginInfoAndPath(d, spec, false, projectPlugins)
	if err != nil {
		return nil, err
	}

	contract.Assertf(info.Path == filepath.Dir(path),
		"plugin executable (%v) is not inside plugin directory (%v)", path, info.Path)
	return info, nil
}

// Given a PluginInfo try to find the executable file that corresponds to it
func getPluginPath(info *PluginInfo) string {
	var path string
	exts := getCandidateExtensions()
	for _, ext := range exts {
		path = filepath.Join(info.Path, info.Spec().File()) + ext
		_, err := os.Stat(path)
		if err == nil {
			return path
		}
	}

	// We didn't actually find a file for this plugin, so just use the old behaviour of assuming the first
	// extension.
	return filepath.Join(info.Path, info.Spec().File()) + exts[0]
}

// getPluginInfoAndPath searches for a compatible plugin kind, name, and version and returns either:
//   - if found as an ambient plugin, nil and the path to the executable
//   - if found in the pulumi dir's installed plugins, a PluginInfo and path to the executable
//   - an error in all other cases.
func getPluginInfoAndPath(
	d diag.Sink,
	spec PluginSpec, skipMetadata bool,
	projectPlugins []ProjectPlugin,
) (*PluginInfo, string, error) {
	filename := spec.File()

	for i, p1 := range projectPlugins {
		for j, p2 := range projectPlugins {
			if j < i {
				if p2.Kind == p1.Kind && p2.Name == p1.Name {
					if p1.Version != nil && p2.Version != nil && p2.Version.Equals(*p1.Version) {
						return nil, "", fmt.Errorf(
							"multiple project plugins with kind %s, name %s, version %s",
							p1.Kind, p1.Name, p1.Version)
					}
				}
			}
		}
	}

	for _, plugin := range projectPlugins {
		if plugin.Kind != spec.Kind {
			continue
		}
		if plugin.Name != spec.Name {
			continue
		}
		if plugin.Version != nil && spec.Version != nil {
			if !plugin.Version.Equals(*spec.Version) {
				logging.Warningf(
					"Project plugin %s with version %s is incompatible with requested version %s.\n",
					spec.Name, plugin.Version, spec.Version)
				continue
			}
		}

		localSpec, err := plugin.Spec()
		if err != nil {
			return nil, "", err
		}
		info := &PluginInfo{
			Name:    localSpec.Name,
			Kind:    localSpec.Kind,
			Version: localSpec.Version,
			Path:    filepath.Clean(plugin.Path),
		}
		// computing plugin sizes can be very expensive (nested node_modules)
		if !skipMetadata {
			if err := info.SetFileMetadata(info.Path); err != nil {
				return nil, "", err
			}
		}
		path := getPluginPath(info)
		return info, path, nil
	}

	// If we have a version of the plugin on its $PATH, use it, unless we have opted out of this behavior explicitly.
	// This supports development scenarios.
	includeAmbient := !(env.IgnoreAmbientPlugins.Value())
	var ambientPath string
	if includeAmbient {
		if path, err := exec.LookPath(filename); err == nil {
			ambientPath = path
			logging.V(6).Infof("GetPluginPath(%s, %s, %v, %s): found on $PATH %s",
				spec.Kind, spec.Name, spec.Version, spec.PluginDownloadURL, path)
		}
	}

	// At some point in the future, bundled plugins will be located in the plugin cache, just like regular
	// plugins (see pulumi/pulumi#956 for some of the reasons why this isn't the case today). For now, they
	// ship next to the `pulumi` binary. While we encourage this folder to be on the $PATH (and so the check
	// above would have normally found the plugin) it's possible someone is running `pulumi` with an explicit
	// path on the command line or has done symlink magic such that `pulumi` is on the path, but the bundled
	// plugins are not, or has simply set IGNORE_AMBIENT_PLUGINS. So, if possible, look next to the instance
	// of `pulumi` that is running to find this bundled plugin.
	var bundledPath string
	if IsPluginBundled(spec.Kind, spec.Name) {
		exePath, exeErr := os.Executable()
		if exeErr == nil {
			fullPath, fullErr := filepath.EvalSymlinks(exePath)
			if fullErr == nil {
				for _, ext := range getCandidateExtensions() {
					candidate := filepath.Join(filepath.Dir(fullPath), filename+ext)
					// Let's see if the file is executable. On Windows, os.Stat() returns a mode of "-rw-rw-rw" so on
					// on windows we just trust the fact that the .exe can actually be launched.
					if stat, err := os.Stat(candidate); err == nil &&
						(stat.Mode()&0o100 != 0 || runtime.GOOS == windowsGOOS) {
						logging.V(6).Infof("GetPluginPath(%s, %s, %v, %s): found next to current executable %s",
							spec.Kind, spec.Name, spec.Version, spec.PluginDownloadURL, candidate)
						bundledPath = candidate
						break
					}
				}
			}
		}
	}

	// We prefer the ambient path, but we need to check if this is the same as the bundled
	// path to decide if we're warning or not.
	pluginPath := bundledPath
	if ambientPath != "" {
		if ambientPath != bundledPath {
			// They don't match _but_ it might be they just don't match because the pulumi install is symlinked,
			// e.g. /opt/homebrew/bin/pulumi-language-nodejs -> /opt/homebrew/Cellar/pulumi/3.77.0/bin/pulumi-language-nodejs
			// So before we warn, lets just check if we can resolve symlinks in the ambient path and then check again.
			fullAmbientPath, err := filepath.EvalSymlinks(ambientPath)
			// N.B, that we don't _return_ the resolved path, we return the original path. Also if resolving
			// hits any errors then we just skip this warning, better to not warn than to error in a new way.
			if err == nil {
				if fullAmbientPath != bundledPath {
					var expected string
					if bundledPath != "" {
						expected = " expected " + bundledPath
					}
					d.Warningf(diag.Message("", "using %s from $PATH at %s%s"), filename, ambientPath, expected)
				}
			}
		}
		pluginPath = ambientPath
	}
	if pluginPath != "" {
		info := &PluginInfo{
			Kind: spec.Kind,
			Name: spec.Name,
			Path: filepath.Dir(pluginPath),
		}
		// computing plugin sizes can be very expensive (nested node_modules)
		if !skipMetadata {
			if err := info.SetFileMetadata(info.Path); err != nil {
				return nil, "", err
			}
		}
		return info, pluginPath, nil
	}

	// Wasn't ambient, and wasn't bundled, so now check the plugin cache.
	var plugins []PluginInfo
	var err error
	if skipMetadata {
		plugins, err = GetPlugins()
	} else {
		plugins, err = GetPluginsWithMetadata()
	}
	if err != nil {
		return nil, "", fmt.Errorf("loading plugin list: %w", err)
	}

	var match *PluginInfo
	if !enableLegacyPluginBehavior &&
		spec.Version != nil &&
		isPreReleaseVersion(*spec.Version) {
		// We're looking for a plugin matching an exact hash, so we can't use the semver range logic.
		logging.V(6).Infof("GetPluginPath(%s, %s, %v, %s): enabling prerelease plugin behaviour",
			spec.Kind, spec.Name, spec.Version, spec.PluginDownloadURL)
		match = SelectPrereleasePlugin(plugins, spec)
	} else if !enableLegacyPluginBehavior && spec.Version != nil {
		logging.V(6).Infof("GetPluginPath(%s, %s, %v, %s): enabling new plugin behavior",
			spec.Kind, spec.Name, spec.Version, spec.PluginDownloadURL)
		match = SelectCompatiblePlugin(plugins, spec)
	} else {
		logging.V(6).Infof("GetPluginPath(%s, %s, %v, %s): using legacy plugin behavior",
			spec.Kind, spec.Name, spec.Version, spec.PluginDownloadURL)
		match = LegacySelectCompatiblePlugin(plugins, spec)
	}

	_, subdir := spec.LocalName()
	// If the plugin is located in a subdir, we need to fix up the path to include the subdir.
	if subdir != "" && match != nil {
		match.Path = filepath.Join(match.Path, subdir)
	}

	if match != nil {
		matchPath := getPluginPath(match)
		logging.V(6).Infof("GetPluginPath(%s, %s, %v, %s): found in cache at %s",
			spec.Kind, spec.Name, spec.Version, spec.PluginDownloadURL, matchPath)
		return match, matchPath, nil
	}

	return nil, "", NewMissingError(spec, includeAmbient)
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
	case iVersion.EQ(*jVersion):
		return iVersion.String() < jVersion.String()
	default:
		return iVersion.LT(*jVersion)
	}
}
func (sp SortedPluginInfo) Swap(i, j int) { sp[i], sp[j] = sp[j], sp[i] }

// SelectPrereleasePlugin selects a plugin from the list of plugins, which matches the exact version we
// are looking for, based on the commit hash.
func SelectPrereleasePlugin(
	plugins []PluginInfo, spec PluginSpec,
) *PluginInfo {
	name, _ := spec.LocalName()
	for _, cur := range plugins {
		if cur.Kind == spec.Kind && cur.Name == name && cur.Version != nil && cur.Version.EQ(*spec.Version) {
			return &cur
		}
	}
	return nil
}

// LegacySelectCompatiblePlugin selects a plugin from the list of plugins with the given kind and name that
// satisfies the requested version. It returns the highest version plugin greater than the requested version,
// or an error if no such plugin could be found.
//
// If there exist plugins in the plugin list that don't have a version, LegacySelectCompatiblePlugin will select
// them if there are no other compatible plugins available.
func LegacySelectCompatiblePlugin(
	plugins []PluginInfo, spec PluginSpec,
) *PluginInfo {
	name, _ := spec.LocalName()
	var match *PluginInfo
	for _, cur := range plugins {
		// Since the value of cur changes as we iterate, we can't save a pointer to it. So let's have a local
		// that we can take a pointer to if this plugin is the best match yet.
		plugin := cur
		if plugin.Kind == spec.Kind && plugin.Name == name {
			// Always pick the most recent version of the plugin available. Even if this is an exact match,
			// we keep on searching just in case there's a newer version available.
			var m *PluginInfo
			if match == nil {
				// no existing match
				if spec.Version == nil {
					m = &plugin // no version spec, accept anything
				} else if plugin.Version == nil || plugin.Version.GTE(*spec.Version) {
					// Either the plugin doesn't have a version, in which case we'll take it but prefer
					// anything else, or it has a version >= requested.
					m = &plugin
				}
			} else {
				// existing match
				if plugin.Version != nil && match.Version == nil {
					// existing match is unversioned, but this plugin has a version, so prefer it.
					m = &plugin
				} else if plugin.Version == nil {
					// this plugin is unversioned ignore it, our current match might also be unversioned but
					// we just pick the first we see in this case.
				} else {
					// both have versions, pick the greater stable one.
					matchIsPre := len(match.Version.Pre) != 0
					pluginIsPre := len(plugin.Version.Pre) != 0

					// The plugin has to at least be greater than the requested version.
					if spec.Version == nil || plugin.Version.GTE(*spec.Version) {
						if matchIsPre && !pluginIsPre {
							// If one is pre-release and the other is not, prefer the non-pre-release one.
							m = &plugin
						} else if !matchIsPre && pluginIsPre {
							// current match is not pre-release, but this plugin is, so prefer the current match.
						} else if plugin.Version.GT(*match.Version) {
							// Else if the plugin is greater than the current match, prefer it.
							m = &plugin
						}
					}
				}
			}

			if m != nil {
				match = m
				logging.V(6).Infof("LegacySelectCompatiblePlugin(%s, %s, %s): found candidate (#%s)",
					spec.Kind, name, spec.Version, match.Version)
			}
		}
	}
	return match
}

// SelectCompatiblePlugin selects a plugin from the list of plugins with the given kind and name that sastisfies the
// requested semver range. It returns the highest version plugin that satisfies the requested constraints, or an error
// if no such plugin could be found.
//
// If there exist plugins in the plugin list that don't have a version, SelectCompatiblePlugin will select them if there
// are no other compatible plugins available.
func SelectCompatiblePlugin(
	plugins []PluginInfo, spec PluginSpec,
) *PluginInfo {
	name, _ := spec.LocalName()
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
		case plugin.Kind != spec.Kind || plugin.Name != name:
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
		case spec.Version.EQ(*plugin.Version):
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
		return nil
	}
	return &bestMatch
}

// ReadCloserProgressBar displays a progress bar for the given closer and returns a wrapper closer to manipulate it.
func ReadCloserProgressBar(
	closer io.ReadCloser, size int64, message string, colorization colors.Colorization,
) io.ReadCloser {
	if size == -1 {
		return closer
	}

	if !cmdutil.Interactive() {
		return closer
	}

	// If we know the length of the download, show a progress bar.
	bar := pb.New(int(size))
	bar.Output = os.Stderr
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

var PluginNameRegexp = regexp.MustCompile(`^(?P<Name>[a-zA-Z0-9-][a-zA-Z0-9-_.]*[a-zA-Z0-9])$`)

// pluginRegexp matches plugin directory names: pulumi-KIND-NAME-VERSION.
var pluginRegexp = regexp.MustCompile(
	"^(?P<Kind>[a-z]+)-" + // KIND
		"(?P<Name>[a-zA-Z0-9-][a-zA-Z0-9-_.]*[a-zA-Z0-9])-" + // NAME
		"v(?P<Version>.*)$") // VERSION

// installingPluginRegexp matches the name of temporary folders. Previous versions of Pulumi first extracted
// plugins to a temporary folder with a suffix of `.tmpXXXXXX` (where `XXXXXX`) is a random number, from
// os.CreateTemp. We should ignore these folders.
var installingPluginRegexp = regexp.MustCompile(`\.tmp[0-9]+$`)

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
			if IsPluginKind(v) {
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

// getPluginSize recursively computes how much space is devoted to a given plugin.
func getPluginSize(path string) (uint64, error) {
	file, err := os.Stat(path)
	if err != nil {
		return 0, nil
	}

	size := uint64(0)
	if file.IsDir() {
		subs, err := os.ReadDir(path)
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
		fs := file.Size()
		if fs < 0 {
			return 0, fmt.Errorf("file size is negative: %d", fs)
		}
		//nolint:gosec // Guarded by the check above.
		size += uint64(fs)
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
	bc.bar.FinishPrint("\r")
	return bc.readCloser.Close()
}
