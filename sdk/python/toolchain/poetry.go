package toolchain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

type poetry struct {
	// The executable path for poetry.
	poetryExecutable string
	// The directory that contains the poetry project.
	directory string
}

var _ Toolchain = &poetry{}

func newPoetry(directory string) (*poetry, error) {
	poetryPath, err := exec.LookPath("poetry")
	if err != nil {
		return nil, fmt.Errorf("poetry not found on path: %w", err)
	}
	logging.V(9).Infof("Python toolchain: using poetry at %s in %s", poetryPath, directory)
	return &poetry{
		poetryExecutable: poetryPath,
		directory:        directory,
	}, nil
}

func (p *poetry) InstallDependencies(ctx context.Context,
	root string, showOutput bool, infoWriter, errorWriter io.Writer,
) error {
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
