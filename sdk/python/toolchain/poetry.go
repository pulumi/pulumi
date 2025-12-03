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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/errutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"gopkg.in/yaml.v3"
)

type poetry struct {
	// The executable path for poetry.
	poetryExecutable string
	// The directory that contains the poetry project.
	directory string
	// The version of poetry.
	version semver.Version
}

var _ Toolchain = &poetry{}

var minPoetryVersion = semver.Version{Major: 1, Minor: 8, Patch: 0}

func newPoetry(directory string) (*poetry, error) {
	poetryPath, err := exec.LookPath("poetry")
	if err != nil {
		return nil, errors.New("Could not find `poetry` executable.\n" +
			"Install poetry and make sure is is in your PATH, or set the toolchain option in Pulumi.yaml to `pip`.")
	}
	versionOut, err := poetryVersionOutput(poetryPath)
	if err != nil {
		return nil, err
	}
	version, err := validateVersion(versionOut)
	if err != nil {
		return nil, err
	}
	logging.V(9).Infof("Python toolchain: using poetry at %s in %s", poetryPath, directory)
	return &poetry{
		poetryExecutable: poetryPath,
		directory:        directory,
		version:          version,
	}, nil
}

func poetryVersionOutput(poetryPath string) (string, error) {
	// Passing `--no-plugins` makes this a fair bit faster
	cmd := exec.Command(poetryPath, "--version", "--no-ansi", "--no-plugins") //nolint:gosec
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run poetry --version: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
}

func validateVersion(versionOut string) (semver.Version, error) {
	re := regexp.MustCompile(`Poetry \(version (?P<version>\d+\.\d+(.\d+)?).*\)`)
	matches := re.FindStringSubmatch(versionOut)
	i := re.SubexpIndex("version")
	if i < 0 || len(matches) < i {
		return semver.Version{}, fmt.Errorf("unexpected output from poetry --version: %q", versionOut)
	}
	version := matches[i]
	sem, err := semver.ParseTolerant(version)
	if err != nil {
		return semver.Version{}, fmt.Errorf("failed to parse poetry version %q: %w", version, err)
	}
	if sem.LT(minPoetryVersion) {
		return semver.Version{}, fmt.Errorf("poetry version %s is less than the minimum required version %s",
			version, minPoetryVersion)
	}
	logging.V(9).Infof("Python toolchain: using poetry version %s", sem)
	return sem, nil
}

func (p *poetry) InstallDependencies(ctx context.Context,
	cwd string, useLanguageVersionTools, showOutput bool, infoWriter, errorWriter io.Writer,
) error {
	if useLanguageVersionTools {
		if err := installPython(ctx, cwd, showOutput, infoWriter, errorWriter); err != nil {
			return err
		}
	}

	// If there's no `pyproject.toml` file, we first need to prepare the project.
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

		if err := p.PrepareProject(ctx, projectName, cwd, showOutput, infoWriter, errorWriter); err != nil {
			return fmt.Errorf("error preparing project: %w", err)
		}
	}

	poetryCmd := exec.Command(p.poetryExecutable, "install", "--no-ansi") //nolint:gosec
	if useLanguageVersionTools {
		// For poetry to work nicely with pyenv, we need to make poetry use the active python,
		// otherwise poetry will use the python version used to run poetry itself.
		use, _, _, err := usePyenv(cwd)
		if err != nil {
			return fmt.Errorf("checking for pyenv: %w", err)
		}
		if use {
			poetryCmd.Env = append(os.Environ(), "POETRY_VIRTUALENVS_PREFER_ACTIVE_PYTHON=true")
		}
	}
	poetryCmd.Dir = p.directory
	poetryCmd.Stdout = infoWriter
	poetryCmd.Stderr = errorWriter
	return poetryCmd.Run()
}

func (p *poetry) PrepareProject(
	ctx context.Context, projectName, cwd string, showOutput bool, infoWriter, errorWriter io.Writer,
) error {
	_, err := searchup(cwd, "pyproject.toml")
	if err == nil {
		// There's already a pyproject.toml, we're done.
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error while looking for pyproject.toml in %s: %w", cwd, err)
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

	var deps map[string]any
	requirementsTxt := filepath.Join(requirementsTxtDir, "requirements.txt")
	if hasRequirementsTxt {
		f, err := os.Open(requirementsTxt)
		if err != nil {
			return fmt.Errorf("failed to open %q: %w", requirementsTxt, err)
		}
		deps, err = dependenciesFromRequirementsTxt(f, requirementsTxtDir)
		contract.IgnoreClose(f)
		if err != nil {
			return fmt.Errorf("failed to parse %q: %w", requirementsTxt, err)
		}
	}

	pyprojectToml := filepath.Join(pyprojectTomlDir, "pyproject.toml")
	b, err := p.generatePyProjectTOML(projectName, deps)
	if err != nil {
		return fmt.Errorf("failed to generate %q: %w", pyprojectToml, err)
	}

	pyprojectFile, err := os.Create(pyprojectToml)
	if err != nil {
		return fmt.Errorf("failed to create %q: %w", pyprojectToml, err)
	}
	defer pyprojectFile.Close()

	if _, err := pyprojectFile.Write([]byte(b)); err != nil {
		return fmt.Errorf("failed to write to %q: %w", pyprojectToml, err)
	}

	if hasRequirementsTxt {
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
	return nil
}

func (p *poetry) LinkPackages(ctx context.Context, packages map[string]string) error {
	logging.V(9).Infof("poetry linking %s", packages)
	args := []string{"add", "--lock"} // Add package to lockfile only
	paths := slices.Collect(maps.Values(packages))
	args = append(args, paths...)
	cmd := exec.Command("poetry", args...)
	if err := cmd.Run(); err != nil {
		return errutil.ErrorWithStderr(err, "linking packages")
	}
	return nil
}

func (p *poetry) ListPackages(ctx context.Context, transitive bool) ([]PythonPackage, error) {
	args := []string{"list", "-v", "--format", "json"}
	if !transitive {
		args = append(args, "--not-required")
	}

	cmd, err := p.ModuleCommand(ctx, "pip", args...)
	if err != nil {
		return nil, err
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("calling `python %s`: %w", strings.Join(cmd.Args, " "), err)
	}

	var packages []PythonPackage
	jsonDecoder := json.NewDecoder(bytes.NewBuffer(output))
	if err := jsonDecoder.Decode(&packages); err != nil {
		return nil, fmt.Errorf("parsing `python %s` output: %w", strings.Join(cmd.Args, " "), err)
	}

	return packages, nil
}

func (p *poetry) Command(ctx context.Context, args ...string) (*exec.Cmd, error) {
	virtualenvPath, err := p.VirtualEnvPath(ctx)
	if err != nil {
		return nil, err
	}

	var cmd *exec.Cmd
	name := "python"
	if runtime.GOOS == windows {
		name = name + ".exe"
	}
	cmdPath := filepath.Join(virtualenvPath, virtualEnvBinDirName(), name)
	cmd = exec.CommandContext(ctx, cmdPath, args...)
	cmd.Env = ActivateVirtualEnv(os.Environ(), virtualenvPath)
	cmd.Dir = p.directory
	return cmd, nil
}

func (p *poetry) ModuleCommand(ctx context.Context, module string, args ...string) (*exec.Cmd, error) {
	moduleArgs := append([]string{"-m", module}, args...)
	return p.Command(ctx, moduleArgs...)
}

func (p *poetry) About(ctx context.Context) (Info, error) {
	var executable string
	var pythonVersion semver.Version

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		version, err := getPythonVersion(ctx, p.Command)
		if err != nil {
			logging.V(9).Infof("Python toolchain: %v", err)
		} else {
			pythonVersion = version
		}
		// Don't fail if we could not parse the python version
		return nil
	})
	g.Go(func() error {
		var err error
		executable, err = getPythonExecutablePath(ctx, p.Command)
		return err
	})

	if err := g.Wait(); err != nil {
		return Info{}, err
	}

	return Info{
		PythonExecutable: executable,
		PythonVersion:    pythonVersion,
		ToolchainVersion: p.version,
	}, nil
}

func (p *poetry) ValidateVenv(ctx context.Context) error {
	virtualenvPath, err := p.VirtualEnvPath(ctx)
	if err != nil {
		return err
	}
	if !IsVirtualEnv(virtualenvPath) {
		return fmt.Errorf("'%s' is not a virtualenv", virtualenvPath)
	}
	return nil
}

func (p *poetry) EnsureVenv(ctx context.Context, cwd string, useLanguageVersionTools,
	showOutput bool, infoWriter, errorWriter io.Writer,
) error {
	_, err := p.VirtualEnvPath(ctx)
	if err != nil {
		// Couldn't get the virtualenv path, this means it does not exist. Let's create it.
		return p.InstallDependencies(ctx, cwd, useLanguageVersionTools, showOutput, infoWriter, errorWriter)
	}
	return nil
}

func (p *poetry) VirtualEnvPath(ctx context.Context) (string, error) {
	pathCmd := exec.CommandContext(ctx, p.poetryExecutable, "env", "info", "--path") //nolint:gosec
	pathCmd.Dir = p.directory
	out, err := pathCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get venv path: %w", err)
	}
	virtualenvPath := strings.TrimSpace(string(out))
	if virtualenvPath == "" {
		return "", errors.New("expected a virtualenv path, got empty string")
	}
	return virtualenvPath, nil
}

func (p *poetry) generatePyProjectTOML(name string, dependencies map[string]any) (string, error) {
	pp := Pyproject{
		BuildSystem: &BuildSystem{
			Requires:     []string{"poetry-core"},
			BuildBackend: "poetry.core.masonry.api",
		},
		Tool: map[string]any{
			"poetry": map[string]any{
				"package-mode": false,
				"dependencies": dependencies,
			},
		},
	}
	if name != "" {
		pp.Project = &Project{
			Name: name,
		}
	}

	w := &bytes.Buffer{}
	encoder := toml.NewEncoder(w)
	encoder.Indent = "" // Disable indentation
	if err := encoder.Encode(pp); err != nil {
		return "", err
	}
	return w.String(), nil
}

func dependenciesFromRequirementsTxt(r io.Reader, basePath string) (map[string]any, error) {
	versionRe := regexp.MustCompile("[<>=]+.+")
	deps := map[string]any{
		"python": "^3.10",
	}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return map[string]any{}, err
		}
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comment lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Drop trailing comments
		parts := strings.SplitN(line, "#", 2)
		line = strings.TrimSpace(parts[0])

		// find the version specififer: "pulumi>=3.0.0,<4.0.0" -> ">=3.0.0,<4.0.0".
		version := string(versionRe.Find([]byte(line)))
		// package is everything before the version specififer.
		pkg := strings.TrimSpace(strings.Replace(line, version, "", 1))

		version = strings.TrimSpace(version)
		if version == "" {
			// If the pkg references a local path, we have to create a dependency entry that points to the path instead
			// of a versioned dependency.
			if info, err := os.Stat(pkg); err == nil {
				relPath, err := filepath.Rel(basePath, pkg)
				if err != nil {
					return map[string]any{}, err
				}
				if info.IsDir() {
					name := strings.ReplaceAll(info.Name(), "-", "_")
					deps[name] = map[string]string{"path": relPath}
				} else if strings.HasSuffix(pkg, ".whl") {
					// If it's a wheel file, we get the package name from the wheel filename. By convention, everything
					// up to the first `-` is the package name.
					parts := strings.SplitN(filepath.Base(pkg), "-", 2)
					if len(parts) == 2 {
						deps[parts[0]] = map[string]string{"path": relPath}
					}
				}
				continue
			}

			version = "*"
		}
		// Drop `==` for an exact version match
		if strings.HasPrefix(version, "==") {
			version = strings.TrimSpace(version[2:])
		}

		deps[pkg] = version
	}

	return deps, nil
}
