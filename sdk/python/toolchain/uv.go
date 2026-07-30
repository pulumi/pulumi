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

	"golang.org/x/sync/errgroup"

	"github.com/BurntSushi/toml"
	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/errutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type uv struct {
	// The absolute path to the virtual env.
	virtualenvPath string
	// The root directory of the project.
	root string
	// The version of uv.
	version semver.Version
}

var minUvVersion = semver.MustParse("0.4.26")

var defaultVirtualEnv = ".venv"

var _ Toolchain = &uv{}

func newUv(root, virtualenv string) (*uv, error) {
	_, err := exec.LookPath("uv")
	if err != nil {
		return nil, errors.New("Could not find `uv` executable.\n" +
			"Install uv and make sure it is in your PATH.")
	}

	if virtualenv == "" {
		// If virtualenv is not set, look for the nearest uv.lock or pyproject.toml to
		// determine where to place the virtualenv.
		projectDir := root
		uvLockDir, err := searchup(root, "uv.lock")
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("error while looking for uv.lock in %s: %w", root, err)
			}
			// No uv.lock, do we have a pyproject.toml?
			pyprojectTomlDir, err := searchup(root, "pyproject.toml")
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return nil, fmt.Errorf("error while looking for pyproject.toml in %s: %w", root, err)
				}
				// We have no uv.lock and no pyproject.toml, place the virtualenv in the project root.
			} else {
				// We have a pyproject.toml, place the virtualenv next to it.
				projectDir = pyprojectTomlDir
			}
		} else {
			// We have a uv.lock, place the virtualenv next to it.
			projectDir = uvLockDir
		}
		if env := os.Getenv("UV_PROJECT_ENVIRONMENT"); env != "" {
			virtualenv = env
			if !filepath.IsAbs(virtualenv) {
				virtualenv = filepath.Join(projectDir, virtualenv)
			}
		} else {
			virtualenv = filepath.Join(projectDir, defaultVirtualEnv)
		}
	}
	if !filepath.IsAbs(virtualenv) {
		virtualenv = filepath.Join(root, virtualenv)
	}

	cmd := exec.Command("uv", "--version")
	versionString, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get uv version: %w", err)
	}
	version, err := ParseUvVersion(string(versionString))
	if err != nil {
		return nil, err
	}
	logging.V(9).Infof("Python toolchain: using uv version %s", version)

	u := &uv{
		virtualenvPath: virtualenv,
		root:           root,
		version:        version,
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

	// If there's no `uv.lock` we first need to prepare the project.
	if _, err := searchup(cwd, "uv.lock"); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("error while looking for uv.lock in %s: %w", cwd, err)
		}
		var projectName string
		if projectPath, err := workspace.DetectProjectPathFrom(cwd); err == nil && projectPath != "" {
			if project, err := workspace.LoadProject(projectPath); err == nil {
				projectName = string(project.Name)
			}
		}
		if err := u.PrepareProject(ctx, projectName, cwd, showOutput, infoWriter, errorWriter); err != nil {
			return fmt.Errorf("error preparing project: %w", err)
		}
	}

	if err := u.checkPyprojectHasProject(cwd); err != nil {
		return err
	}

	// We now have either a uv.lock or at least a pyproject.toml file, and we can use uv
	// install the dependencies.
	syncCmd := u.uvCommand(ctx, cwd, showOutput, infoWriter, errorWriter, "sync")
	if !showOutput {
		_, err := syncCmd.Output()
		return errutil.ErrorWithStderr(err, "error installing dependencies")
	} else {
		return syncCmd.Run()
	}
}

// PrepareProject prepares a project for use with uv. It will create a suitable pyproject.toml project file. If a
// requirements.txt file exists, its dependencies will be added to pyproject.toml. If a pyproject.toml exists but
// has no [project] section and a colocated requirements.txt is present, [project] is appended and the deps from
// requirements.txt are merged in. No-op if pyproject.toml already has a [project] section.
func (u *uv) PrepareProject(
	ctx context.Context, projectName, cwd string, showOutput bool, infoWriter, errorWriter io.Writer,
) error {
	existingPyprojectDir, err := searchup(cwd, "pyproject.toml")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error while looking for nearest pyproject.toml in %s: %w", cwd, err)
	}
	if err == nil {
		py, err := LoadPyproject(existingPyprojectDir)
		if err != nil {
			return err
		}
		if py.Project != nil {
			// Already has a [project] section, nothing to do.
			return nil
		}
		// pyproject.toml exists but has no [project]. If requirements.txt sits next to it, append a minimal
		// [project] section and merge the deps in.
		requirementsTxt := filepath.Join(existingPyprojectDir, "requirements.txt")
		if _, statErr := os.Stat(requirementsTxt); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("error while looking for requirements.txt: %w", statErr)
		}
		name := projectName
		if name == "" {
			name = filepath.Base(existingPyprojectDir)
		}
		pyprojectPath := filepath.Join(existingPyprojectDir, "pyproject.toml")
		section := fmt.Sprintf("\n[project]\nname = %q\nversion = \"0.1.0\"\ndependencies = []\n", name)
		f, err := os.OpenFile(pyprojectPath, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			return fmt.Errorf("opening %s: %w", pyprojectPath, err)
		}
		if _, err := f.WriteString(section); err != nil {
			contract.IgnoreClose(f)
			return fmt.Errorf("appending [project] to %s: %w", pyprojectPath, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("closing %s: %w", pyprojectPath, err)
		}
		return u.mergeRequirementsTxt(ctx, existingPyprojectDir, showOutput, infoWriter, errorWriter)
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
	if u.version.LT(semver.MustParse("0.6.0")) {
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
		if err := u.mergeRequirementsTxt(ctx, requirementsTxtDir, showOutput, infoWriter, errorWriter); err != nil {
			return err
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
	// Skip if the venv already exists. `uv venv --allow-existing` re-copies the launcher into `python.exe`, and on
	// Windows that can fail with a file-lock error if any earlier process still holds the existing python.exe (e.g.
	// recursive plugin installs that run multiple uv operations against the same venv).
	if IsVirtualEnv(u.virtualenvPath) {
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

	var lock uvLockFile
	if _, err := toml.Decode(string(content), &lock); err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", lockFilePath, err)
	}

	// In a uv workspace `uv.lock` contains the dependencies of *all* workspace members, but `uv sync`
	// only installs the current member's dependencies. Walk the lock's dependency graph so we only
	// report its transitive dependencies.
	if packages, ok := lock.scopeToMember(lockDir, u.root, transitive); ok {
		return packages, nil
	}

	// Fall back to reading the whole lock file if we couldn't locate the current project within it
	logging.V(5).Infof("could not locate %s within %s, reporting all locked packages", u.root, lockFilePath)
	return listPackagesFromLockFile(lockFilePath, transitive, lock.virtualPackages())
}

// uvLockFile is a representation of uv.lock sufficient to walk its dependency graph and identify
// workspace members.
type uvLockFile struct {
	Package []uvLockPackage `toml:"package"`
}

type uvLockDependency struct {
	Name string `toml:"name"`
}

type uvLockPackage struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
	Source  struct {
		Virtual  string `toml:"virtual"`
		Editable string `toml:"editable"`
	} `toml:"source"`
	Dependencies    []uvLockDependency            `toml:"dependencies"`
	DevDependencies map[string][]uvLockDependency `toml:"dev-dependencies"`
}

// isMember reports whether the package is the project root or a workspace member rather than an installable
// dependency.
func (p *uvLockPackage) isMember() bool {
	return p.Source.Virtual != "" || p.Source.Editable != ""
}

// memberPath returns the workspace-relative path recorded for the project root or a workspace member.
func (p *uvLockPackage) memberPath() string {
	if p.Source.Virtual != "" {
		return p.Source.Virtual
	}
	return p.Source.Editable
}

// virtualPackages returns the names of packages that are virtual (i.e. the project root or workspace
// members) in the lock file. These are not real installable packages.
func (l *uvLockFile) virtualPackages() map[string]bool {
	virtual := make(map[string]bool)
	for _, pkg := range l.Package {
		if pkg.Source.Virtual != "" {
			virtual[normalizePythonPackageName(pkg.Name)] = true
		}
	}
	return virtual
}

// scopeToMember walks the lock's dependency graph starting from the workspace member located at root and
// returns its (transitive) dependencies. The boolean result is false if the member could not be located in
// the lock, in which case the caller should fall back to reporting all locked packages.
func (l *uvLockFile) scopeToMember(lockDir, root string, transitive bool) ([]plugin.DependencyInfo, bool) {
	// uv records member paths relative to the lock file directory, using forward slashes. The project root
	// itself is recorded as ".".
	rel, err := filepath.Rel(lockDir, root)
	if err != nil {
		return nil, false
	}
	rel = filepath.ToSlash(rel)

	byName := make(map[string]*uvLockPackage, len(l.Package))
	var current *uvLockPackage
	for i := range l.Package {
		pkg := &l.Package[i]
		byName[normalizePythonPackageName(pkg.Name)] = pkg
		if pkg.isMember() && pkg.memberPath() == rel {
			current = pkg
		}
	}
	if current == nil {
		return nil, false
	}

	// Breadth-first walk from the current member. The member's own dependencies and dependency-group
	// dependencies seed the queue; from there we follow runtime dependencies only, matching what gets
	// installed into the virtualenv.
	queue := make([]string, 0, len(current.Dependencies))
	for _, dep := range current.Dependencies {
		queue = append(queue, normalizePythonPackageName(dep.Name))
	}
	for _, group := range current.DevDependencies {
		for _, dep := range group {
			queue = append(queue, normalizePythonPackageName(dep.Name))
		}
	}

	seen := make(map[string]bool)
	var packages []plugin.DependencyInfo
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] {
			continue
		}
		seen[name] = true

		pkg, ok := byName[name]
		if !ok {
			continue
		}
		for _, dep := range pkg.Dependencies {
			queue = append(queue, normalizePythonPackageName(dep.Name))
		}
		// Workspace members (including the current one, if it appears as a dependency) are not installable
		// packages, so leave them out of the reported set.
		if pkg.isMember() {
			continue
		}
		packages = append(packages, plugin.DependencyInfo{Name: name, Version: pkg.Version})
	}

	if transitive {
		return packages, true
	}
	direct, err := filterDirectPythonDependencies(root, packages)
	if err != nil {
		return nil, false
	}
	return direct, true
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
	pyprojectTomlDir, err := searchup(u.root, "pyproject.toml")
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("error while looking for pyproject.toml in %s: %w", u.root, err)
		}
	}
	if pyprojectTomlDir != "" {
		if err := u.checkPyprojectHasProject(u.root); err != nil {
			return nil, err
		}
		// uv run does an "inexact" sync, that is it leaves extraneous
		// dependencies alone and does not remove them.
		venvCmd := u.uvCommand(ctx, u.root, false, nil, nil, "sync", "--inexact")
		if _, err := venvCmd.Output(); err != nil {
			return nil, errutil.ErrorWithStderr(err, "error creating virtual environment")
		}
	}

	var cmd *exec.Cmd
	_, cmdPath := u.pythonExecutable()
	cmd = exec.CommandContext(ctx, cmdPath, args...)
	cmd.Env = ActivateVirtualEnv(cmd.Environ(), u.virtualenvPath)
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
	cmd := exec.CommandContext(ctx, "uv", args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if showOutput {
		cmd.Stdout = infoWriter
		cmd.Stderr = errorWriter
	}
	cmd.Env = append(cmd.Environ(), "UV_PROJECT_ENVIRONMENT="+u.virtualenvPath)
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

// mergeRequirementsTxt runs `uv add -r requirements.txt` against the pyproject.toml in pyprojectDir, then removes the
// requirements.txt file
func (u *uv) mergeRequirementsTxt(
	ctx context.Context, pyprojectDir string, showOutput bool, infoWriter, errorWriter io.Writer,
) error {
	requirementsTxt := filepath.Join(pyprojectDir, "requirements.txt")
	args := []string{"add", "--no-sync", "-r", requirementsTxt}
	needs, err := u.needsNoWorkspacesFlag(ctx)
	if err != nil {
		return err
	}
	if needs {
		args = append(args, "--no-workspace")
	}
	addCmd := u.uvCommand(ctx, pyprojectDir, showOutput, infoWriter, errorWriter, args...)
	if err := addCmd.Run(); err != nil {
		return errutil.ErrorWithStderr(err, "error installing dependencies from requirements.txt")
	}
	if err := os.Remove(requirementsTxt); err != nil {
		return fmt.Errorf("failed to remove %q: %w", requirementsTxt, err)
	}
	if showOutput && infoWriter != nil {
		if _, err := infoWriter.Write([]byte("Deleted requirements.txt, " +
			"dependencies for this project are tracked in pyproject.toml\n")); err != nil {
			return fmt.Errorf("failed to write to infoWriter: %w", err)
		}
	}
	return nil
}

func (u *uv) checkPyprojectHasProject(cwd string) error {
	if _, err := searchup(cwd, "uv.lock"); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error while looking for uv.lock in %s: %w", cwd, err)
	}
	pyprojectDir, err := searchup(cwd, "pyproject.toml")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("error while looking for pyproject.toml in %s: %w", cwd, err)
	}
	pyproject, err := LoadPyproject(pyprojectDir)
	if err != nil {
		return err
	}
	if pyproject.Project == nil {
		return fmt.Errorf("%s is missing a [project] section, which uv requires to install dependencies; "+
			"add a [project] section with a name and dependencies to use the uv toolchain",
			filepath.Join(pyprojectDir, "pyproject.toml"))
	}
	return nil
}

func (u *uv) needsNoWorkspacesFlag(ctx context.Context) (bool, error) {
	// Starting with version 0.8.0, uv will automatically add packages in subdirectories as workspace members. However
	// the generated SDK might not have a `pyproject.toml`, which is required for uv workspace members. To add the
	// generated SDK as a normal dependency, we can run `uv add --no-workspace`, but this flag is only available on
	// version 0.8.0 and up.
	if u.version.GE(semver.MustParse("0.8.0")) {
		return true, nil
	}
	return false, nil
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
