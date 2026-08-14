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

package selfupdate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// staleSuffix marks binaries displaced by an update. Windows refuses to delete a running executable, so the old
// binary is renamed out of the way and cleaned up by a later run instead.
const staleSuffix = ".pulumi-old"

// stagingPrefix names the directory an update downloads and unpacks into. Windows can hold a handle to a freshly
// written executable for a moment, which makes removing that directory fail, so leftovers are swept by a later run.
const stagingPrefix = ".pulumi-update-"

// managedPathMarkers are path segments belonging to package managers that own their own copy of the CLI. Updating
// underneath one leaves the manager's metadata describing a version that is no longer on disk.
var managedPathMarkers = []string{
	"/cellar/",      // Homebrew
	"/nix/store/",   // Nix
	"node_modules/", // npm / npx shim
	"/.asdf/",       // asdf
	"/mise/",        // mise
}

// errNotUpdatable reports that the running CLI cannot be updated in place. Hint carries the command the user should
// run instead, when we know it.
type errNotUpdatable struct {
	reason string
	hint   string
}

func (e *errNotUpdatable) Error() string {
	if e.hint == "" {
		return e.reason
	}
	return e.reason + "\nTo upgrade, run:\n    " + e.hint
}

// detectInstallDir returns the directory holding the running CLI, provided it looks like an install produced by
// get.pulumi.com and is safe to replace.
func detectInstallDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not locate the running executable: %w", err)
	}
	return installDirFor(exe)
}

// installDirFor applies the update eligibility rules to a given executable path.
func installDirFor(exe string) (string, error) {
	// Resolve symlinks so that a link on $PATH is judged by where it actually points.
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("could not resolve %s: %w", exe, err)
	}
	dir := filepath.Dir(resolved)

	if marker := managedPathMarker(dir); marker != "" {
		return "", &errNotUpdatable{
			reason: fmt.Sprintf("%s looks like it was installed by a package manager, so it should be upgraded "+
				"with that package manager rather than replaced in place.", resolved),
			hint: managedPathHint(marker),
		}
	}

	// Both installers lay the bundle out flat, so the language plugins sit alongside the CLI. A directory without
	// them is a shim or a development build, neither of which this command should overwrite.
	siblings, err := filepath.Glob(filepath.Join(dir, "pulumi-language-*"))
	if err != nil {
		return "", err
	}
	if len(siblings) == 0 {
		return "", &errNotUpdatable{
			reason: fmt.Sprintf("%s does not look like a Pulumi CLI install: the bundled language plugins are "+
				"not present alongside it.", resolved),
		}
	}

	if err := checkWritable(dir); err != nil {
		return "", &errNotUpdatable{
			reason: fmt.Sprintf("%s is not writable by the current user: %v", dir, err),
		}
	}

	return dir, nil
}

// managedPathMarker returns the package manager marker found in dir, or the empty string if there is none.
func managedPathMarker(dir string) string {
	// Normalise separators directly rather than with filepath.ToSlash, which only rewrites them on Windows.
	needle := strings.ToLower(strings.ReplaceAll(dir, `\`, "/")) + "/"
	for _, marker := range managedPathMarkers {
		if strings.Contains(needle, marker) {
			return marker
		}
	}
	return ""
}

func managedPathHint(marker string) string {
	switch marker {
	case "/cellar/":
		return "brew update && brew upgrade pulumi"
	case "node_modules/":
		return "npm update @pulumi/pulumi"
	default:
		return ""
	}
}

// checkWritable reports whether new files can be created in dir.
func checkWritable(dir string) error {
	probe, err := os.CreateTemp(dir, ".pulumi-update-probe-*")
	if err != nil {
		return err
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		return err
	}
	return os.Remove(name)
}

// swapIn replaces every pulumi binary in installDir with its counterpart from stagedDir. Each file is moved into
// place individually after the existing one is renamed aside, so a failure part way through can be rolled back and
// no window exists in which the CLI is missing entirely.
func swapIn(stagedDir, installDir string) error {
	staged, err := os.ReadDir(stagedDir)
	if err != nil {
		return err
	}

	// A displaced binary from an earlier update may still be on disk if it was running at the time.
	removeStale(installDir)

	type swapped struct{ from, to string }
	var displaced []swapped

	rollback := func() {
		for _, s := range displaced {
			// Best effort: the update already failed, so surface the original error rather than this one.
			//nolint:forbidigo // staging sits alongside the install directory, so this rename stays on one file system
			_ = os.Rename(s.to, s.from)
		}
	}

	stamp := strconv.Itoa(os.Getpid())
	for _, entry := range staged {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "pulumi") {
			continue
		}

		target := filepath.Join(installDir, entry.Name())
		if _, err := os.Lstat(target); err == nil {
			aside := target + staleSuffix + "." + stamp
			//nolint:forbidigo // renaming within one directory, which is also how a running executable is displaced
			if err := os.Rename(target, aside); err != nil {
				rollback()
				return fmt.Errorf("could not move the existing %s aside: %w", entry.Name(), err)
			}
			displaced = append(displaced, swapped{from: target, to: aside})
		} else if !errors.Is(err, os.ErrNotExist) {
			rollback()
			return err
		}

		//nolint:forbidigo // staging sits alongside the install directory, so this rename stays on one file system
		if err := os.Rename(filepath.Join(stagedDir, entry.Name()), target); err != nil {
			rollback()
			return fmt.Errorf("could not install %s: %w", entry.Name(), err)
		}
	}

	// The update is committed, so the displaced binaries are safe to drop. The running executable is among them on
	// Windows and cannot be deleted yet; removeStale picks it up next time.
	removeStale(installDir)
	return nil
}

// removeStale deletes binaries displaced by this or an earlier update, ignoring the ones still held open.
func removeStale(installDir string) {
	stale, err := filepath.Glob(filepath.Join(installDir, "*"+staleSuffix+".*"))
	if err != nil {
		return
	}
	for _, path := range stale {
		_ = os.Remove(path)
	}
}

// removeOrphanedStaging deletes staging directories an earlier update could not remove.
func removeOrphanedStaging(parentDir string) {
	orphans, err := filepath.Glob(filepath.Join(parentDir, stagingPrefix+"*"))
	if err != nil {
		return
	}
	for _, path := range orphans {
		_ = os.RemoveAll(path)
	}
}
