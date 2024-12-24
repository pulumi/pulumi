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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

type uv struct {
	// The absolute path to the virtual env.
	virtualenvPath string
	// The root directory of the project.
	root string
}

var minUvVersion = semver.MustParse("0.4.26")

var defaultVirtualEnv = ".venv"

var _ Toolchain = &uv{}

func newUv(root, virtualenv string) (*uv, error) {
	_, err := exec.LookPath("uv")
	if err != nil {
		return nil, errors.New("Could not find `uv` executable.\n" +
			"Install uv and make sure is is in your PATH.")
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

	u := &uv{
		virtualenvPath: virtualenv,
		root:           root,
	}

	// Validate the version
	cmd := u.uvCommand(context.Background(), "", false, nil, nil, "--version")
	versionString, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get uv version: %w", err)
	}
	version, err := u.uvVersion(string(versionString))
	if err != nil {
		return nil, err
	}
	logging.V(9).Infof("Python toolchain: using uv version %s", version)

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

	// Look for a uv.lock file.
	// If no uv.lock file is found, look for a pyproject.toml file.
	// If no pyproject.toml file is found, create it, and then look for a
	// requirements.txt file to install dependencies.
	if _, err := searchup(cwd, "uv.lock"); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("error while looking for uv.lock in %s: %w", cwd, err)
		}

		// No uv.lock found, look for pyproject.toml.
		if _, err := searchup(cwd, "pyproject.toml"); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("error while looking for pyproject.toml in %s: %w", cwd, err)
			}
			// No pyproject.toml found, we'll create it with `uv init`.
			// First we'll look for a requirements.txt file. If we find one, we'll use the directory
			// that contains the requirements.txt file as the directory for `pyproject.toml`.
			// We'll also install the dependencies from the requirements.txt file., and then
			// remove the requirements.txt file.
			requirementsTxtDir, err := searchup(cwd, "requirements.txt")
			pyprojectTomlDir := cwd
			hasRequirementsTxt := false
			if err == nil {
				pyprojectTomlDir = requirementsTxtDir
				hasRequirementsTxt = true
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("error while looking for requirements.txt in %s: %w", cwd, err)
			}

			initCmd := u.uvCommand(ctx, pyprojectTomlDir, showOutput, infoWriter, errorWriter,
				"init", "--no-readme", "--no-package", "--no-pin-python")
			if err := initCmd.Run(); err != nil {
				return errorWithStderr(err, "error initializing python project")
			}

			if hasRequirementsTxt {
				requirementsTxt := filepath.Join(requirementsTxtDir, "requirements.txt")
				addCmd := u.uvCommand(ctx, cwd, showOutput, infoWriter, errorWriter, "add", "-r", requirementsTxt)
				if err := addCmd.Run(); err != nil {
					return errorWithStderr(err, "error installing dependecies from requirements.txt")
				}
				// Remove the requirements.txt file, after calling `uv add`, the
				// dependencies are tracked in pyproject.toml.
				if err := os.Remove(requirementsTxt); err != nil {
					return fmt.Errorf("failed to remove %q", requirementsTxt)
				}
				if showOutput {
					if _, err := infoWriter.Write([]byte("Deleted requirements.txt, " +
						"dependencies for this project are tracked in pyproject.toml\n")); err != nil {
						return fmt.Errorf("failed to write to infoWriter: %w", err)
					}
				}
			}

			// `uv init` creates a `hello.py` file, delete it.
			contract.IgnoreError(os.Remove(filepath.Join(cwd, "hello.py")))
		}
	}

	// We now have either a uv.lock or at least a pyproject.toml file, and we can use uv
	// install the dependencies.
	syncCmd := u.uvCommand(ctx, cwd, showOutput, infoWriter, errorWriter, "sync")
	if err := syncCmd.Run(); err != nil {
		return errorWithStderr(err, "error installing dependencies")
	}
	return nil
}

func (u *uv) EnsureVenv(ctx context.Context, cwd string, useLanguageVersionTools, showOutput bool,
	infoWriter, errorWriter io.Writer,
) error {
	venvCmd := u.uvCommand(ctx, cwd, showOutput, infoWriter, errorWriter, "venv", "--quiet",
		"--allow-existing", u.virtualenvPath)
	if err := venvCmd.Run(); err != nil {
		return errorWithStderr(err, "error creating virtual environment")
	}

	return nil
}

func (u *uv) ValidateVenv(ctx context.Context) error {
	if !IsVirtualEnv(u.virtualenvPath) {
		return fmt.Errorf("'%s' is not a virtualenv", u.virtualenvPath)
	}
	return nil
}

func (u *uv) ListPackages(ctx context.Context, transitive bool) ([]PythonPackage, error) {
	// We use `pip` instead of `uv pip` because `uv pip` does not respect the
	// `-v` flag, which is required to get the package location.
	// https://github.com/astral-sh/uv/issues/9838
	pipCmd, err := u.ModuleCommand(ctx, "pip", "list", "--format", "json", "-v")
	if err != nil {
		return nil, fmt.Errorf("preparing pip list command: %w", err)
	}
	// Check if pip is installed, if not, we'll fallback to `uvx pip`, which will install an
	// isolated pip for us.
	cmd, err := u.ModuleCommand(ctx, "pip")
	if err != nil {
		return nil, fmt.Errorf("preparing check pip command: %w", err)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(out), "No module named pip") {
			cmd := exec.CommandContext(ctx, "uvx", "pip", "list", "--format", "json", "-v")
			cmd.Dir = u.root
			pipCmd = cmd
		} else {
			return nil, errorWithStderr(err, "checking for pip")
		}
	}

	output, err := pipCmd.Output()
	if err != nil {
		return nil, errorWithStderr(err, "listing packages")
	}

	var packages []PythonPackage
	if err := json.Unmarshal(output, &packages); err != nil {
		return nil, fmt.Errorf("parsing package list: %w", err)
	}

	return packages, nil
}

func (u *uv) Command(ctx context.Context, args ...string) (*exec.Cmd, error) {
	// Note that we do not use `uv run python` here because this results in a
	// process tree of `python-language-runtime -> uv -> python`. This is
	// problematic because on error we kill the plugin and its children, but not
	// the children of the children. On macOS and Linux, when uv is killed, it
	// kills its children, so we have no problem here. On Windows however, it
	// does not, and we end up with an orphaned Python process that's
	// busy-waiting in the eventloop and never exits.
	var cmd *exec.Cmd
	name := "python"
	if runtime.GOOS == windows {
		name = name + ".exe"
	}
	cmdPath := filepath.Join(u.virtualenvPath, virtualEnvBinDirName(), name)
	if needsPythonShim(cmdPath) {
		shimCmd := fmt.Sprintf(pythonShimCmdFormat, name)
		cmd = exec.CommandContext(ctx, shimCmd, args...)
	} else {
		cmd = exec.CommandContext(ctx, cmdPath, args...)
	}
	cmd.Env = ActivateVirtualEnv(cmd.Environ(), u.virtualenvPath)
	cmd.Dir = u.root
	return cmd, nil
}

func (u *uv) ModuleCommand(ctx context.Context, module string, args ...string) (*exec.Cmd, error) {
	moduleArgs := append([]string{"-m", module}, args...)
	return u.Command(ctx, moduleArgs...)
}

func (u *uv) About(ctx context.Context) (Info, error) {
	cmd, err := u.Command(ctx, "--version")
	if err != nil {
		return Info{}, err
	}
	var out []byte
	if out, err = cmd.Output(); err != nil {
		return Info{}, fmt.Errorf("failed to get version: %w", err)
	}
	version := strings.TrimSpace(strings.TrimPrefix(string(out), "Python "))

	cmd, err = u.Command(ctx, "-c", "import sys; print(sys.executable)")
	if err != nil {
		return Info{}, err
	}
	out, err = cmd.Output()
	if err != nil {
		return Info{}, fmt.Errorf("failed to get python executable path: %w", err)
	}
	executable := strings.TrimSpace(string(out))

	return Info{
		Executable: executable,
		Version:    version,
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

func (u *uv) uvVersion(versionString string) (semver.Version, error) {
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
