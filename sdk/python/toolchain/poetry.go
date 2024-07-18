package toolchain

import (
	"bufio"
	"bytes"
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

	"github.com/BurntSushi/toml"
	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

type poetry struct {
	// The executable path for poetry.
	poetryExecutable string
	// The directory that contains the poetry project.
	directory string
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
	if err := validateVersion(versionOut); err != nil {
		return nil, err
	}
	logging.V(9).Infof("Python toolchain: using poetry at %s in %s", poetryPath, directory)
	return &poetry{
		poetryExecutable: poetryPath,
		directory:        directory,
	}, nil
}

func poetryVersionOutput(poetryPath string) (string, error) {
	cmd := exec.Command(poetryPath, "--version", "--no-ansi") //nolint:gosec
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run poetry --version: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
}

func validateVersion(versionOut string) error {
	re := regexp.MustCompile(`Poetry \(version (?P<version>\d+\.\d+(.\d+)?).*\)`)
	matches := re.FindStringSubmatch(versionOut)
	i := re.SubexpIndex("version")
	if i < 0 || len(matches) < i {
		return fmt.Errorf("unexpected output from poetry --version: %q", versionOut)
	}
	version := matches[i]
	sem, err := semver.ParseTolerant(version)
	if err != nil {
		return fmt.Errorf("failed to parse poetry version %q: %w", version, err)
	}
	if sem.LT(minPoetryVersion) {
		return fmt.Errorf("poetry version %s is less than the minimum required version %s", version, minPoetryVersion)
	}
	logging.V(9).Infof("Python toolchain: using poetry version %s", sem)
	return nil
}

func (p *poetry) InstallDependencies(ctx context.Context,
	root string, showOutput bool, infoWriter, errorWriter io.Writer,
) error {
	// If pyproject.toml does not exist, but we have a requirements.txt,
	// generate a new pyproject.toml.
	pyprojectToml := filepath.Join(root, "pyproject.toml")
	if _, err := os.Stat(pyprojectToml); err != nil && errors.Is(err, os.ErrNotExist) {
		requirementsTxt := filepath.Join(root, "requirements.txt")
		if _, err := os.Stat(requirementsTxt); err != nil && errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("could not find pyproject.toml or requirements.txt in %s", root)
		}
		if err := p.convertRequirementsTxt(requirementsTxt, pyprojectToml); err != nil {
			return err
		}
	}

	poetryCmd := exec.Command(p.poetryExecutable, "install", "--no-ansi") //nolint:gosec
	poetryCmd.Dir = p.directory
	poetryCmd.Stdout = infoWriter
	poetryCmd.Stderr = errorWriter
	return poetryCmd.Run()
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
		return nil, fmt.Errorf("calling `python -m pip %s`: %w", strings.Join(args, " "), err)
	}

	var packages []PythonPackage
	jsonDecoder := json.NewDecoder(bytes.NewBuffer(output))
	if err := jsonDecoder.Decode(&packages); err != nil {
		return nil, fmt.Errorf("parsing `python -m pip %s` output: %w", strings.Join(args, " "), err)
	}

	return packages, nil
}

func (p *poetry) Command(ctx context.Context, args ...string) (*exec.Cmd, error) {
	virtualenvPath, err := p.virtualenvPath(ctx)
	if err != nil {
		return nil, err
	}

	var cmd *exec.Cmd
	name := "python"
	if runtime.GOOS == windows {
		name = name + ".exe"
	}
	cmdPath := filepath.Join(virtualenvPath, virtualEnvBinDirName(), name)
	if needsPythonShim(cmdPath) {
		shimCmd := fmt.Sprintf(pythonShimCmdFormat, name)
		cmd = exec.CommandContext(ctx, shimCmd, args...)
	} else {
		cmd = exec.CommandContext(ctx, cmdPath, args...)
	}
	cmd.Env = ActivateVirtualEnv(os.Environ(), virtualenvPath)
	cmd.Dir = p.directory
	return cmd, nil
}

func (p *poetry) ModuleCommand(ctx context.Context, module string, args ...string) (*exec.Cmd, error) {
	moduleArgs := append([]string{"-m", module}, args...)
	return p.Command(ctx, moduleArgs...)
}

func (p *poetry) About(ctx context.Context) (Info, error) {
	cmd, err := p.Command(ctx, "--version")
	if err != nil {
		return Info{}, err
	}
	executable := cmd.Path
	var out []byte
	if out, err = cmd.Output(); err != nil {
		return Info{}, fmt.Errorf("failed to get version: %w", err)
	}
	version := strings.TrimSpace(strings.TrimPrefix(string(out), "Python "))

	return Info{
		Executable: executable,
		Version:    version,
	}, nil
}

func (p *poetry) ValidateVenv(ctx context.Context) error {
	virtualenvPath, err := p.virtualenvPath(ctx)
	if err != nil {
		return err
	}
	if !IsVirtualEnv(virtualenvPath) {
		return fmt.Errorf("'%s' is not a virtualenv", virtualenvPath)
	}
	return nil
}

func (p *poetry) EnsureVenv(ctx context.Context, cwd string, showOutput bool, infoWriter, errorWriter io.Writer) error {
	_, err := p.virtualenvPath(ctx)
	if err != nil {
		// Couldn't get the virtualenv path, this means it does not exist. Let's create it.
		return p.InstallDependencies(ctx, cwd, showOutput, infoWriter, errorWriter)
	}
	return nil
}

func (p *poetry) virtualenvPath(ctx context.Context) (string, error) {
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

func (p *poetry) convertRequirementsTxt(requirementsTxt, pyprojectToml string) error {
	f, err := os.Open(requirementsTxt)
	if err != nil {
		return fmt.Errorf("failed to open %q", requirementsTxt)
	}

	deps, err := dependenciesFromRequirementsTxt(f)
	contract.IgnoreError(f.Close())
	if err != nil {
		return fmt.Errorf("failed to gather dependencies from %q", requirementsTxt)
	}

	b, err := p.generatePyProjectTOML(deps)
	if err != nil {
		return fmt.Errorf("failed to generate %q", pyprojectToml)
	}

	pyprojectFile, err := os.Create(pyprojectToml)
	if err != nil {
		return fmt.Errorf("failed to create %q", pyprojectToml)
	}
	defer pyprojectFile.Close()

	if _, err := pyprojectFile.Write([]byte(b)); err != nil {
		return fmt.Errorf("failed to write to %q", pyprojectToml)
	}

	if err := os.Remove(requirementsTxt); err != nil {
		return fmt.Errorf("failed to remove %q", requirementsTxt)
	}

	return nil
}

func (p *poetry) generatePyProjectTOML(dependencies map[string]string) (string, error) {
	type BuildSystem struct {
		Requires     []string `toml:"requires,omitempty" json:"requires,omitempty"`
		BuildBackend string   `toml:"build-backend,omitempty" json:"build-backend,omitempty"`
	}

	type Pyproject struct {
		BuildSystem *BuildSystem           `toml:"build-system,omitempty" json:"build-system,omitempty"`
		Tool        map[string]interface{} `toml:"tool,omitempty" json:"tool,omitempty"`
	}

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

	w := &bytes.Buffer{}
	encoder := toml.NewEncoder(w)
	encoder.Indent = "" // Disable indentation
	if err := encoder.Encode(pp); err != nil {
		return "", err
	}
	return w.String(), nil
}

func dependenciesFromRequirementsTxt(r io.Reader) (map[string]string, error) {
	versionRe := regexp.MustCompile("[<>=]+.+")
	deps := map[string]string{
		"python": "^3.8",
	}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return map[string]string{}, err
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
