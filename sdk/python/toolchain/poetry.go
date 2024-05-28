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
	logging.V(9).Infof("Python toolchain: using poetry at %s", poetryPath)
	return &poetry{
		poetryExecutable: poetryPath,
		directory:        directory,
	}, nil
}

func (p *poetry) InstallDependenciesWithWriters(ctx context.Context,
	root string, showOutput bool, infoWriter, errorWriter io.Writer,
) error {
	poetryCmd := exec.Command(p.poetryExecutable, "install") //nolint:gosec
	// poetryCmd.Dir = root
	poetryCmd.Dir = p.directory
	poetryCmd.Stdout = infoWriter
	poetryCmd.Stderr = errorWriter
	// TODO: pip adds setuptools and wheels to the venv by default.
	return poetryCmd.Run()
}

func (p *poetry) ListPackages(ctx context.Context, transitive bool) ([]PythonPackage, error) {
	args := []string{"-m", "pip", "list", "-v", "--format", "json"}
	if !transitive {
		args = append(args, "--not-required")
	}

	cmd, err := p.Command(ctx, args...)
	if err != nil {
		return nil, err
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("calling `python %s`: %w", strings.Join(args, " "), err)
	}

	// Parse the JSON output; on some systems pip -v verbose mode
	// follows JSON with non-JSON trailer, so we need to be
	// careful when parsing and ignore the trailer.
	var packages []PythonPackage
	jsonDecoder := json.NewDecoder(bytes.NewBuffer(output))
	if err := jsonDecoder.Decode(&packages); err != nil {
		return nil, fmt.Errorf("parsing `python %s` output: %w", strings.Join(args, " "), err)
	}

	return packages, nil
}

func (p *poetry) Command(ctx context.Context, args ...string) (*exec.Cmd, error) {
	virtualenvPath, err := p.virtualenvPath(ctx)
	if err != nil {
		return nil, err
	}

	pythonCmd := os.Getenv("PULUMI_PYTHON_CMD")
	if pythonCmd == "" {
		pythonCmd = "python3" // TODO: do we need to look for `python`? See CommandPath
	}

	var cmd *exec.Cmd
	cmdPath := filepath.Join(virtualenvPath, virtualEnvBinDirName(), pythonCmd)
	if needsPythonShim(cmdPath) {
		shimCmd := fmt.Sprintf(pythonShimCmdFormat, pythonCmd)
		cmd = exec.CommandContext(ctx, shimCmd, args...)
	} else {
		cmd = exec.CommandContext(ctx, cmdPath, args...)
	}
	cmd.Env = ActivateVirtualEnv(os.Environ(), virtualenvPath)
	cmd.Dir = p.directory
	fmt.Printf("cmd.Path = %s\n", cmd.Path)
	return cmd, nil
}

func (p *poetry) Setup(ctx context.Context) error {
	// TODO
	return nil
}

func (p *poetry) About(ctx context.Context) (Info, error) {
	// TODO
	return Info{}, nil
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
