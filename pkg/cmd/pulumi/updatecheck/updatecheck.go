// Copyright 2026, Pulumi Corporation.
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

// Package updatecheck checks whether a newer version of the CLI is available and builds the
// command metadata that accompanies the version-check request.
package updatecheck

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/djherbis/times"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// haveNewerDevVersion checks whether we have a newer dev version available.
func haveNewerDevVersion(devVersion semver.Version, curVersion semver.Version) bool {
	if devVersion.Major != curVersion.Major {
		return devVersion.Major > curVersion.Major
	}
	if devVersion.Minor != curVersion.Minor {
		return devVersion.Minor > curVersion.Minor
	}
	if devVersion.Patch != curVersion.Patch {
		return devVersion.Patch > curVersion.Patch
	}

	// The dev version string looks like: v1.0.0-11-g4ff08363.  We
	// can determine whether we have a newer dev version by
	// comparing the second part of the version string, which is
	// the number of commits since the last tag.
	devVersionParts := strings.Split(devVersion.String(), "-")
	curVersionParts := strings.Split(curVersion.String(), "-")

	// We're being leninent with parsing here.  If we can't parse
	// a version number correctly for any reason, we default to
	// pretending there is no newer version, and not warning the
	// user.  As this is only a warning this is better than
	// asserting or crashing in the error case.
	if len(devVersionParts) != 3 || len(curVersionParts) != 3 {
		return false
	}
	devCommits, err := strconv.Atoi(devVersionParts[1])
	if err != nil {
		return false
	}
	curCommits, err := strconv.Atoi(curVersionParts[1])
	if err != nil {
		return false
	}
	return devCommits > curCommits
}

// CheckResult is the outcome of a version check that found a newer version: a warning to show the
// user and the version information to cache.
type CheckResult struct {
	Diag        *diag.Diag
	versionInfo cachedVersionInfo
}

// checkForUpdate checks to see if the CLI needs to be updated, and if so emits a warning, as well as information
// as to how it can be upgraded.
func checkForUpdate(ctx context.Context, cloudURL string, metadata map[string]string) *CheckResult {
	curVer, err := semver.ParseTolerant(version.Version)
	if err != nil {
		slog.InfoContext(ctx, "error parsing current version", "err", err)
	}

	// We don't care about warning about updates if this is a locally-compiled version
	if isLocalVersion(curVer) {
		return nil
	}

	isCurVerDev := isDevVersion(curVer)
	canPrompt, lastPromptTimestampMS := checkVersionPrompt(isCurVerDev)

	latestVer, oldestAllowedVer, devVer, err := getCLIVersionInfo(ctx, cloudURL, metadata)
	if err != nil {
		slog.InfoContext(ctx, fmt.Sprintf("error fetching latest version information; set %s to true to skip update checks",
			env.SkipUpdateCheck.Var().Name()), "err", err)
	}

	willPrompt := canPrompt &&
		((isCurVerDev && haveNewerDevVersion(devVer, curVer)) ||
			(!isCurVerDev && oldestAllowedVer.GT(curVer)))

	if willPrompt {
		lastPromptTimestampMS = time.Now().UnixMilli() // We're prompting, update the timestamp
	}

	if willPrompt {
		if isCurVerDev {
			latestVer = devVer
		}

		msg := getUpgradeMessage(latestVer, curVer, isCurVerDev)
		return &CheckResult{
			Diag: diag.RawMessage("", msg),
			versionInfo: cachedVersionInfo{
				LatestVersion:         latestVer.String(),
				OldestWithoutWarning:  oldestAllowedVer.String(),
				LatestDevVersion:      devVer.String(),
				LastPromptTimeStampMS: lastPromptTimestampMS,
			},
		}
	}

	return nil
}

// Start begins a version check against cloudURL in the background, carrying the given command
// metadata. It returns a channel that receives the result, or nil when update checks are
// disabled. Collect the result later with Finish; Start never blocks.
func Start(ctx context.Context, cloudURL string, metadata map[string]string) <-chan *CheckResult {
	if env.SkipUpdateCheck.Value() {
		slog.InfoContext(ctx, "skipping update check")
		return nil
	}

	ch := make(chan *CheckResult, 1)
	go func() {
		ch <- checkForUpdate(ctx, cloudURL, metadata)
		close(ch)
	}()
	return ch
}

// Finish collects the result of a Start-ed check without blocking, caching its version
// information. It returns the result so callers can surface the upgrade warning, or nil when
// the check was skipped, has not finished, or found nothing to report.
func Finish(ch <-chan *CheckResult) *CheckResult {
	if ch == nil {
		return nil
	}

	select {
	case result, ok := <-ch:
		if !ok || result == nil {
			return nil
		}
		if err := cacheVersionInfo(result.versionInfo); err != nil {
			slog.Info("failed to cache version info", "err", err)
		}
		return result
	default:
		return nil
	}
}

// getCLIVersionInfo returns information about the latest version of the CLI and the oldest version that should be
// allowed without warning, as well as the amount of time to cache this information.
func getCLIVersionInfo(
	ctx context.Context,
	cloudURL string,
	metadata map[string]string,
) (semver.Version, semver.Version, semver.Version, error) {
	creds, err := workspace.GetStoredCredentials()
	apiToken := creds.AccessTokens[creds.Current]

	if err != nil || creds.Current != cloudURL {
		apiToken = ""
		metadata = nil
	}

	c := client.NewClient(cloudURL, apiToken, false, cmdutil.Diag())
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	latest, oldest, dev, err := c.GetCLIVersionInfo(ctx, metadata)
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, err
	}

	brewLatest, isBrew, err := getLatestBrewFormulaVersion()
	if err != nil {
		slog.InfoContext(ctx, "error determining if the running executable was installed with brew", "err", err)
	}
	if isBrew {
		// When consulting Homebrew for version info, we just use the latest version as the oldest allowed.
		latest, oldest, dev = brewLatest, brewLatest, brewLatest
	}

	// Don't return the err from getLatestBrewFormulaVersion here, we just log that above.
	return latest, oldest, dev, nil
}

// cacheVersionInfo saves version information in a cache file to be looked up later.
func cacheVersionInfo(info cachedVersionInfo) error {
	updateCheckFile, err := pkgWorkspace.GetCachedVersionFilePath()
	if err != nil {
		return err
	}

	file, err := os.OpenFile(updateCheckFile, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(file)

	return json.NewEncoder(file).Encode(info)
}

// readVersionInfo reads version information from the cache file.
func readVersionInfo() (cachedVersionInfo, error) {
	updateCheckFile, err := pkgWorkspace.GetCachedVersionFilePath()
	if err != nil {
		return cachedVersionInfo{}, err
	}

	file, err := os.Open(updateCheckFile)
	if err != nil {
		return cachedVersionInfo{}, err
	}
	defer contract.IgnoreClose(file)

	var info cachedVersionInfo
	if err := json.NewDecoder(file).Decode(&info); err != nil {
		return cachedVersionInfo{}, err
	}

	return info, nil
}

// checkVersionPrompt determines if
//   - enough time has passed since we last prompted the user
//   - the timestamp when we last prompted the user
//
// If we can't read the cached versions file, we return true and a zero time,
// indicating that we want to possibly prompt the user for an upgrade.
func checkVersionPrompt(devVersion bool) (bool, int64) {
	updateCheckFile, err := pkgWorkspace.GetCachedVersionFilePath()
	if err != nil {
		return true, 0
	}

	ts, err := times.Stat(updateCheckFile)
	if err != nil {
		return true, 0
	}

	info, err := readVersionInfo()
	if err != nil {
		return true, 0
	}

	// Prompt at most once a day for regular versions, and at most once an hour for dev versions.
	promptCacheTime := 24 * time.Hour
	if devVersion {
		promptCacheTime = 1 * time.Hour
	}

	// Fallback to the file modification date if we didn't save a last prompt timestamp yet.
	lastPrompt := ts.ModTime()
	if info.LastPromptTimeStampMS > 0 {
		lastPrompt = time.UnixMilli(info.LastPromptTimeStampMS)
	}

	nextPrompt := lastPrompt.Add(promptCacheTime)
	expired := nextPrompt.Before(time.Now())

	return expired, lastPrompt.UnixMilli()
}

// cachedVersionInfo is the on disk format of the version information the CLI caches between runs.
type cachedVersionInfo struct {
	LatestVersion         string `json:"latestVersion"`
	OldestWithoutWarning  string `json:"oldestWithoutWarning"`
	LatestDevVersion      string `json:"latestDevVersion"`
	LastPromptTimeStampMS int64  `json:"LastPromptMS,omitempty"`
}

// getUpgradeMessage gets a message to display to a user instructing them they are out of date and how to move from
// current to latest.
func getUpgradeMessage(latest semver.Version, current semver.Version, isDevVersion bool) string {
	cmd := getUpgradeCommand(isDevVersion)

	// If the current version is "very old", we'll return a more urgent message. "Very old" is defined as more than 24
	// minor versions behind when the major versions are the same. Assuming a release cadence of on average 1 minor
	// version per week, this translates to roughly 6 months. Note that we don't consider major version differences, since
	// it's hard to know what we'd want to do in those cases. E.g. it might be that a new version of Pulumi is radically
	// different, rather than "just improved", and so we don't want to warn about that.
	prefix := "A new version of Pulumi is available."

	minorDiff := diffMinorVersions(current, latest)
	if minorDiff > 24 {
		prefix = colors.SpecAttention +
			"You are running a very old version of Pulumi and should upgrade as soon as possible." + colors.Reset
	}

	msg := fmt.Sprintf("%s To upgrade from version '%s' to '%s', ", prefix, current, latest)
	if cmd != "" {
		msg += "run \n   " + cmd + "\nor "
	}

	msg += "visit https://pulumi.com/docs/install/ for manual instructions and release notes."
	return msg
}

// diffMinorVersions compares two semver versions.
//   - If the major versions of the two versions are the same, it returns the difference in their minor versions. This
//     difference will be a positive number if v2 is greater than v1 and a negative number if v1 is greater than v2.
//   - If the major versions differ, it returns 0.
func diffMinorVersions(v1 semver.Version, v2 semver.Version) int64 {
	if v1.Major != v2.Major {
		return 0
	}

	return int64(v2.Minor - v1.Minor)
}

// getUpgradeCommand returns a command that will upgrade the CLI to the newest version. If we can not determine how
// the CLI was installed, the empty string is returned.
func getUpgradeCommand(isDevVersion bool) string {
	curUser, err := user.Current()
	if err != nil {
		return ""
	}

	exe, err := os.Executable()
	if err != nil {
		return ""
	}

	isBrew, err := isBrewInstall(exe)
	if err != nil {
		slog.Info("error determining if the running executable was installed with brew", "err", err)
	}
	if isBrew {
		return "$ brew update && brew upgrade pulumi"
	}

	if filepath.Dir(exe) != filepath.Join(curUser.HomeDir, workspace.BookkeepingDir, "bin") {
		return ""
	}

	if runtime.GOOS != "windows" {
		command := "$ curl -sSL https://get.pulumi.com | sh"
		if isDevVersion {
			command = command + " -s -- --version dev"
		}
		return command
	}

	powershellCmd := `"%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe"`

	if _, err := exec.LookPath("powershell"); err == nil {
		powershellCmd = "powershell"
	}

	powershellCmd = "> " + powershellCmd + ` -NoProfile -InputFormat None -ExecutionPolicy Bypass -Command "iex ` +
		`((New-Object System.Net.WebClient).DownloadString('https://get.pulumi.com/install.ps1'))"`
	if isDevVersion {
		powershellCmd = powershellCmd + " -version dev"
	}
	return powershellCmd
}

// isBrewInstall returns true if the current running executable is running on macOS and was installed with brew.
func isBrewInstall(exe string) (bool, error) {
	if runtime.GOOS != "darwin" {
		return false, nil
	}

	exePath, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return false, err
	}

	brewBin, err := exec.LookPath("brew")
	if err != nil {
		return false, err
	}

	brewPrefixCmd := exec.Command(brewBin, "--prefix", "pulumi")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	brewPrefixCmd.Stdout = &stdout
	brewPrefixCmd.Stderr = &stderr
	if err = brewPrefixCmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = stderr.Bytes()
		}
		return false, fmt.Errorf("'brew --prefix pulumi' failed: %w", err)
	}

	brewPrefixCmdOutput := strings.TrimSpace(stdout.String())
	if brewPrefixCmdOutput == "" {
		return false, errors.New("trimmed output from 'brew --prefix pulumi' is empty")
	}

	brewPrefixPath, err := filepath.EvalSymlinks(brewPrefixCmdOutput)
	if err != nil {
		return false, err
	}

	brewPrefixExePath := filepath.Join(brewPrefixPath, "bin", "pulumi")
	return exePath == brewPrefixExePath, nil
}

func getLatestBrewFormulaVersion() (semver.Version, bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return semver.Version{}, false, err
	}

	isBrew, err := isBrewInstall(exe)
	if err != nil {
		return semver.Version{}, false, err
	}
	if !isBrew {
		return semver.Version{}, false, nil
	}

	const formulaJSON = "https://formulae.brew.sh/api/formula/pulumi.json"
	url, err := url.Parse(formulaJSON)
	contract.AssertNoErrorf(err, "Could not parse URL %q", formulaJSON)

	resp, err := httputil.DoWithRetry(&http.Request{
		Method: http.MethodGet,
		URL:    url,
	}, http.DefaultClient)
	if err != nil {
		return semver.Version{}, false, err
	}
	defer contract.IgnoreClose(resp.Body)

	type versions struct {
		Stable string `json:"stable"`
	}
	var formula struct {
		Versions versions `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&formula); err != nil {
		return semver.Version{}, false, err
	}

	stable, err := semver.ParseTolerant(formula.Versions.Stable)
	if err != nil {
		return semver.Version{}, false, err
	}
	return stable, true, nil
}

func isLocalVersion(s semver.Version) bool {
	if len(s.Pre) == 0 {
		return false
	}

	devStrings := regexp.MustCompile(`alpha|beta|dev|rc`)
	return !s.Pre[0].IsNum && devStrings.MatchString(s.Pre[0].VersionStr)
}

func isDevVersion(s semver.Version) bool {
	if len(s.Pre) == 0 {
		return false
	}

	devRegex := regexp.MustCompile(`\d*-g[0-9a-f]*$`)
	return !s.Pre[0].IsNum && devRegex.MatchString(s.Pre[0].VersionStr)
}
