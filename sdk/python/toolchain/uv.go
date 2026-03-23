// Copyright 2024, Pulumi Corporation.
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

package toolchain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/BurntSushi/toml"
	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/errutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"gopkg.in/yaml.v3"
)

// cachedUvVersion caches the result of running `uv --version` so we only
// shell out once per process. The uv binary doesn't change during a
// process's lifetime.
var (
	cachedUvVersionOnce sync.Once
	cachedUvVersion     semver.Version
	cachedUvVersionErr  error
	cachedUvPath        string
	cachedUvPathErr     error

	// uvSyncedPaths tracks virtualenv paths that have been synced by
	// InstallDependencies. This allows Command() to skip the redundant
	// `uv sync --inexact` when it's called on the same virtualenv shortly
	// after installation. The map is keyed by the absolute virtualenv path.
	uvSyncedPaths   = make(map[string]bool)
	uvSyncedPathsMu sync.Mutex
)

func getUvVersion() (semver.Version, error) {
	cachedUvVersionOnce.Do(func() {
		cachedUvPath, cachedUvPathErr = exec.LookPath("uv")
		if cachedUvPathErr != nil {
			cachedUvVersionErr = errors.New("Could not find `uv` executable.\n" +
				"Install uv and make sure it is in your PATH.")
			return
		}

		cmd := exec.Command(cachedUvPath, "--version")
		versionString, err := cmd.Output()
		if err != nil {
			cachedUvVersionErr = fmt.Errorf("failed to get uv version: %w", err)
			return
		}
		cachedUvVersion, cachedUvVersionErr = ParseUvVersion(string(versionString))
	})
	return cachedUvVersion, cachedUvVersionErr
}

type uv struct {
	// The absolute path to the virtual env.
	virtualenvPath string
	// The root directory of the project.
	root string
	// The version of uv.
	version semver.Version
	// synced tracks whether we have already run `uv sync` for this
	// toolchain instance so we can skip redundant syncs in Command().
	// This is safe because within a single language-host RPC call
	// (e.g. InstallDependencies followed by Run), the project's
	// dependencies don't change between our own operations.
	synced bool
	// needsNoWorkspace is pre-computed at construction time from the uv
	// version. When true, `uv add` calls must include --no-workspace.
	needsNoWorkspace bool
}

var minUvVersion = semver.MustParse("0.4.26")

// Pre-parsed version thresholds used in hot-path comparisons.
var (
	uvVersion060 = semver.MustParse("0.6.0")
	uvVersion080 = semver.MustParse("0.8.0")
)

var defaultVirtualEnv = ".venv"

var _ Toolchain = &uv{}

func newUv(root, virtualenv string) (*uv, error) {
	version, err := getUvVersion()
	if err != nil {
		return nil, err
	}

	if virtualenv == "" {
		// If virtualenv is not set, look for the nearest uv.lock or pyproject.toml to
		// determine where to place the virtualenv.
		uvLockDir, err := searchup(root, "uv.lock")
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("error while looking for pyproject.toml in %s: %w", root, err)
			}
			// No uv.lock, do we have a pyproject.toml?
			pyprojectTomlDir, err := searchup(root, "pyproject.toml")
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return nil, fmt.Errorf("error while looking for pyproject.toml in %s: %w", root, err)
				}
				// We have no uv.lock and no pyproject.toml, place the virtualenv in the project root.
				virtualenv = filepath.Join(root, defaultVirtualEnv)
			} else {
				// We have a pyproject.toml, place the virtualenv next to it.
				virtualenv = filepath.Join(pyprojectTomlDir, defaultVirtualEnv)
			}
		} else {
			// We have a uv.lock, place the virtualenv next to it.
			virtualenv = filepath.Join(uvLockDir, defaultVirtualEnv)
		}
	}
	if !filepath.IsAbs(virtualenv) {
		virtualenv = filepath.Join(root, virtualenv)
	}

	logging.V(9).Infof("Python toolchain: using uv version %s", version)

	u := &uv{
		virtualenvPath:   virtualenv,
		root:             root,
		version:          version,
		needsNoWorkspace: version.GE(uvVersion080),
	}

	return u, nil
}

func (u *uv) InstallDependencies(ctx context.Context, cwd string, useLanguageVersionTools,
	showOutput bool, infoWriter, errorWriter io.Writer,
) error {
	if useLanguageVersionTools {
		if err := installPython(ctx, cwd, showOutput, infoWriter, errorWriter); err != nil {
			return err
		}
	}

	// If there's no `uv.lock` or `pyproject.toml` file, we first need to prepare the project.
	// Track whether a lockfile exists so we can reuse this below for --frozen.
	_, lockErr := searchup(cwd, "uv.lock")
	hasLockFile := lockErr == nil
	if !hasLockFile {
		if !errors.Is(lockErr, os.ErrNotExist) {
			return fmt.Errorf("error while looking for uv.lock in %s: %w", cwd, lockErr)
		}
		// No uv.lock found, look for pyproject.toml.
		if _, err := searchup(cwd, "pyproject.toml"); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("error while looking for pyproject.toml in %s: %w", cwd, err)
			}
			// No pyproject.toml found, this is likely a template with a requirements.txt, convert it to a
			// pyproject.toml file.
			// We can't use workspace.LoadProject here because the workspace module depends on toolchain.
			// TODO: https://github.com/pulumi/pulumi/issues/20953
			//
			// We can also remove the call to `PrepareProject` here eventually. Before `Language.Template` existed, the
			// creation of a `pyproject.toml` file happened during `pulumi install`. It is possible to have a half
			// initialized project, for example from `pulumi new ... --generate-only` which has a `requirements.txt`
			// that still needs to be converted. We want to maintain the same behavior as before here for a while.
			// TODO: https://github.com/pulumi/pulumi/issues/20987
			var projectName string
			pulumiYamlPath := filepath.Join(cwd, "Pulumi.yaml")
			if pulumiYamlData, err := os.ReadFile(pulumiYamlPath); err == nil {
				var pulumiConfig struct {
					Name tokens.PackageName `json:"name" yaml:"name"`
				}
				if err := yaml.Unmarshal(pulumiYamlData, &pulumiConfig); err == nil {
					projectName = string(pulumiConfig.Name)
				}
			}
			if err := u.PrepareProject(ctx, projectName, cwd, showOutput, infoWriter, errorWriter); err != nil {
				return fmt.Errorf("error preparing project: %w", err)
			}
		}
	}

	// We now have either a uv.lock or at least a pyproject.toml file, and we can use uv
	// install the dependencies.
	syncArgs := []string{"sync"}
	// When a lockfile already exists, use --frozen to skip dependency
	// resolution and install directly from the lock. This avoids the
	// resolver entirely and is significantly faster.
	if hasLockFile {
		syncArgs = append(syncArgs, "--frozen")
	}
	if !showOutput {
		// Suppress progress output when the caller doesn't need it.
		syncArgs = append(syncArgs, "--no-progress")
	}
	syncCmd := u.uvCommand(ctx, cwd, showOutput, infoWriter, errorWriter, syncArgs...)
	if !showOutput {
		_, err := syncCmd.Output()
		if err != nil {
			return errutil.ErrorWithStderr(err, "error installing dependencies")
		}
		u.markSynced()
		return nil
	} else {
		if err := syncCmd.Run(); err != nil {
			return err
		}
		u.markSynced()
		return nil
	}
}

// PrepareProject prepares a project for use with uv. It will create a suitable pyproject.toml project file. If a
// requirements.txt file exists, its dependencies will be added to pyproject.toml. No-op if pyproject.toml exists.
func (u *uv) PrepareProject(
	ctx context.Context, projectName, cwd string, showOutput bool, infoWriter, errorWriter io.Writer,
) error {
	_, err := searchup(cwd, "pyproject.toml")
	if err == nil {
		// There's already a pyproject.toml, we're done.
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error while looking for nearest pyproject.toml in %s: %w", cwd, err)
	}

	requirementsTxtDir, err := searchup(cwd, "requirements.txt")
	pyprojectTomlDir := cwd
	hasRequirementsTxt := false
	if err == nil {
		pyprojectTomlDir = requirementsTxtDir
		hasRequirementsTxt = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error while looking for requirements.txt in %s: %w", cwd, err)
	}

	args := []string{"init", "--bare", "--no-package", "--no-pin-python"}
	deleteHello := false
	if u.version.LT(uvVersion060) {
		// The `--bare` option prevents `uv init` from creating a
		// `main.py` file, but this is only available in uv 0.6. Prior
		// to 0.6, uv always creates a `hello.py` file, which we
		// manually delete below.
		// https://github.com/astral-sh/uv/blob/main/CHANGELOG.md#060
		args = []string{"init", "--no-readme", "--no-package", "--no-pin-python"}
		deleteHello = true
	}

	// Set the name in pyproject to that of the pulumi project
	if projectName != "" {
		args = append(args, "--name", projectName)
	}

	initCmd := u.uvCommand(ctx, pyprojectTomlDir, showOutput, infoWriter, errorWriter, args...)
	if err := initCmd.Run(); err != nil {
		return errutil.ErrorWithStderr(err, "error initializing python project")
	}

	if hasRequirementsTxt {
		requirementsTxt := filepath.Join(requirementsTxtDir, "requirements.txt")
		args := []string{"add", "--no-sync", "-r", requirementsTxt}
		needs, err := u.needsNoWorkspacesFlag(ctx)
		if err != nil {
			return err
		}
		if needs {
			args = append(args, "--no-workspace")
		}
		addCmd := u.uvCommand(ctx, cwd, showOutput, infoWriter, errorWriter, args...)
		if err := addCmd.Run(); err != nil {
			return errutil.ErrorWithStderr(err, "error installing dependecies from requirements.txt")
		}
		// Remove the requirements.txt file, after calling `uv add`, the
		// dependencies are tracked in pyproject.toml.
		if err := os.Remove(requirementsTxt); err != nil {
			return fmt.Errorf("failed to remove %q: %w", requirementsTxt, err)
		}
		if showOutput {
			if _, err := infoWriter.Write([]byte("Deleted requirements.txt, " +
				"dependencies for this project are tracked in pyproject.toml\n")); err != nil {
				return fmt.Errorf("failed to write to infoWriter: %w", err)
			}
		}
	}

	// `uv init` prior to 0.6 creates a `hello.py` file, delete it.
	if deleteHello {
		contract.IgnoreError(os.Remove(filepath.Join(cwd, "hello.py")))
	}
	return nil
}

func (u *uv) LinkPackages(ctx context.Context, packages map[string]string) error {
	logging.V(9).Infof("uv linking %s", packages)
	args := []string{"add", "--no-sync"} // Don't update the venv

	needs, err := u.needsNoWorkspacesFlag(ctx)
	if err != nil {
		return err
	}
	if needs {
		args = append(args, "--no-workspace")
	}

	paths := slices.Collect(maps.Values(packages))
	args = append(args, paths...)
	cmd := u.uvCommand(ctx, u.root, false, nil, nil, args...)
	if _, err := cmd.Output(); err != nil {
		return errutil.ErrorWithStderr(err, "linking packages")
	}
	return nil
}

func (u *uv) EnsureVenv(ctx context.Context, cwd string, useLanguageVersionTools, showOutput bool,
	infoWriter, errorWriter io.Writer,
) error {
	// If we already synced, the venv was created as part of the sync. Skip
	// the redundant venv creation.
	if u.isSynced() {
		return nil
	}
	venvCmd := u.uvCommand(ctx, cwd, showOutput, infoWriter, errorWriter, "venv", "--quiet",
		"--allow-existing", u.virtualenvPath)
	if err := venvCmd.Run(); err != nil {
		return errutil.ErrorWithStderr(err, "error creating virtual environment")
	}

	return nil
}

func (u *uv) ValidateVenv(ctx context.Context) error {
	if !IsVirtualEnv(u.virtualenvPath) {
		return fmt.Errorf("'%s' is not a virtualenv", u.virtualenvPath)
	}
	return nil
}

func (u *uv) ListPackages(_ context.Context, transitive bool) ([]plugin.DependencyInfo, error) {
	lockDir, err := searchup(u.root, "uv.lock")
	if err != nil {
		return nil, fmt.Errorf("could not find uv.lock: %w", err)
	}
	lockFilePath := filepath.Join(lockDir, "uv.lock")
	content, err := os.ReadFile(lockFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", lockFilePath, err)
	}
	virtual, err := uvVirtualPackages(content)
	if err != nil {
		return nil, fmt.Errorf("could not identify virtual packages in %s: %w", lockFilePath, err)
	}
	// Pass the already-read content to avoid a redundant file read.
	return listPackagesFromLockFileContent(lockFilePath, content, transitive, virtual)
}

// uvLockFile is a minimal representation of uv.lock for identifying virtual packages.
type uvLockFile struct {
	Package []uvLockPackage `toml:"package"`
}

type uvLockPackage struct {
	Name   string `toml:"name"`
	Source struct {
		Virtual string `toml:"virtual"`
	} `toml:"source"`
}

// uvVirtualPackages returns the names of packages that are virtual (i.e. the project root or workspace members) in a
// uv.lock file. Virtual packages have source = { virtual = "..." } and are not real installable packages.
func uvVirtualPackages(content []byte) (map[string]bool, error) {
	var lock uvLockFile
	if _, err := toml.Decode(string(content), &lock); err != nil {
		return nil, err
	}
	virtual := make(map[string]bool)
	for _, pkg := range lock.Package {
		if pkg.Source.Virtual != "" {
			virtual[normalizePythonPackageName(pkg.Name)] = true
		}
	}
	return virtual, nil
}

// markSynced records that this virtualenv has been synced, both on the
// instance and globally so that new instances for the same path skip the sync.
func (u *uv) markSynced() {
	u.synced = true
	uvSyncedPathsMu.Lock()
	uvSyncedPaths[u.virtualenvPath] = true
	uvSyncedPathsMu.Unlock()
}

// isSynced returns true if this virtualenv has already been synced, either
// by this instance or by a previous instance with the same path.
func (u *uv) isSynced() bool {
	if u.synced {
		return true
	}
	uvSyncedPathsMu.Lock()
	synced := uvSyncedPaths[u.virtualenvPath]
	uvSyncedPathsMu.Unlock()
	if synced {
		u.synced = true
	}
	return synced
}

func (u *uv) Command(ctx context.Context, args ...string) (*exec.Cmd, error) {
	// Note that we do not use `uv run python` here because this results in a
	// process tree of `python-language-runtime -> uv -> python`. This is
	// problematic because on error we kill the plugin and its children, but not
	// the children of the children. On macOS and Linux, when uv is killed, it
	// kills its children, so we have no problem here. On Windows however, it
	// does not, and we end up with an orphaned Python process that's
	// busy-waiting in the eventloop and never exits.
	// See https://github.com/astral-sh/uv/issues/11817
	//
	// To maintain uv's behaviour that `uv run ...` should keep the venv
	// up-to-date, we run `uv sync` first, provided there is a `pyproject.toml`.
	// However, if we already ran a sync for this virtualenv (e.g. during
	// InstallDependencies), we can skip the redundant sync.
	if !u.isSynced() {
		// Check for uv.lock first; its presence implies pyproject.toml exists.
		_, lockErr := searchup(u.root, "uv.lock")
		hasLock := lockErr == nil
		hasPyproject := hasLock
		if !hasLock {
			if !errors.Is(lockErr, os.ErrNotExist) {
				return nil, fmt.Errorf("error while looking for uv.lock in %s: %w", u.root, lockErr)
			}
			if _, err := searchup(u.root, "pyproject.toml"); err == nil {
				hasPyproject = true
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("error while looking for pyproject.toml in %s: %w", u.root, err)
			}
		}
		if hasPyproject {
			// uv run does an "inexact" sync, that is it leaves extraneous
			// dependencies alone and does not remove them.
			syncArgs := []string{"sync", "--inexact", "--no-progress"}
			// Use --frozen when a lockfile exists to skip resolution.
			if hasLock {
				syncArgs = append(syncArgs, "--frozen")
			}
			venvCmd := u.uvCommand(ctx, u.root, false, nil, nil, syncArgs...)
			if _, err := venvCmd.Output(); err != nil {
				return nil, errutil.ErrorWithStderr(err, "error creating virtual environment")
			}
			u.markSynced()
		}
	}

	var cmd *exec.Cmd
	_, cmdPath := u.pythonExecutable()
	cmd = exec.CommandContext(ctx, cmdPath, args...)
	env := ActivateVirtualEnv(cmd.Environ(), u.virtualenvPath)
	env = append(env,
		"PYTHONDONTWRITEBYTECODE=1",
		// Skip scanning user site-packages directory, which is unnecessary
		// when running inside a virtualenv and saves startup time.
		"PYTHONNOUSERSITE=1",
	)
	cmd.Env = env
	cmd.Dir = u.root
	return cmd, nil
}

func (u *uv) ModuleCommand(ctx context.Context, module string, args ...string) (*exec.Cmd, error) {
	moduleArgs := append([]string{"-m", module}, args...)
	return u.Command(ctx, moduleArgs...)
}

func (u *uv) About(ctx context.Context) (Info, error) {
	var executable string
	var pythonVersion semver.Version

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		version, err := getPythonVersion(ctx, u.Command)
		if err != nil {
			logging.V(9).Infof("getPythonVersion: %v", err)
		} else {
			pythonVersion = version
		}
		// Don't fail if we could not parse the python version
		return nil
	})
	g.Go(func() error {
		var err error
		executable, err = getPythonExecutablePath(ctx, u.Command)
		return err
	})

	if err := g.Wait(); err != nil {
		return Info{}, err
	}

	return Info{
		PythonExecutable: executable,
		PythonVersion:    pythonVersion,
		ToolchainVersion: u.version,
	}, nil
}

func (u *uv) uvCommand(ctx context.Context, cwd string, showOutput bool,
	infoWriter, errorWriter io.Writer, args ...string,
) *exec.Cmd {
	// Use the cached uv path from getUvVersion() to avoid repeated LookPath calls.
	uvPath := cachedUvPath
	if uvPath == "" {
		uvPath = "uv"
	}
	cmd := exec.CommandContext(ctx, uvPath, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if showOutput {
		cmd.Stdout = infoWriter
		cmd.Stderr = errorWriter
	}
	cmd.Env = append(cmd.Environ(),
		"UV_PROJECT_ENVIRONMENT="+u.virtualenvPath,
		"PYTHONDONTWRITEBYTECODE=1",
		"PYTHONNOUSERSITE=1",
		// Use hardlinks where possible to avoid copying cached packages
		// into the virtual environment, significantly speeding up installs.
		"UV_LINK_MODE=hardlink",
		// Suppress progress bars globally via env var so every uv
		// subprocess is quiet without needing --no-progress per call.
		"UV_NO_PROGRESS=1",
	)
	return cmd
}

func (u *uv) pythonExecutable() (string, string) {
	name := "python"
	if runtime.GOOS == windows {
		name = name + ".exe"
	}
	return name, filepath.Join(u.virtualenvPath, virtualEnvBinDirName(), name)
}

func (u *uv) VirtualEnvPath(_ context.Context) (string, error) {
	return u.virtualenvPath, nil
}

func (u *uv) needsNoWorkspacesFlag(ctx context.Context) (bool, error) {
	return u.needsNoWorkspace, nil
}

func ParseUvVersion(versionString string) (semver.Version, error) {
	versionString = strings.TrimSpace(versionString)
	re := regexp.MustCompile(`uv (?P<version>\d+\.\d+(.\d+)?).*`)
	matches := re.FindStringSubmatch(versionString)
	i := re.SubexpIndex("version")
	if i < 0 || len(matches) < i {
		return semver.Version{}, fmt.Errorf("unexpected output from `uv --version`: %q", versionString)
	}
	v := matches[i]
	sem, err := semver.ParseTolerant(v)
	if err != nil {
		return semver.Version{}, fmt.Errorf("failed to parse uv version %q: %w", versionString, err)
	}
	if sem.LT(minUvVersion) {
		return semver.Version{}, fmt.Errorf("uv version %s is less than the minimum required version %s",
			versionString, minUvVersion)
	}
	return sem, nil
}
