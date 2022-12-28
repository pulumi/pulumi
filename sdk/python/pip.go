// Copyright 2016-2022, Pulumi Corporation.
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

package python

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

type Pip struct {
	SDependencyTool
}

func getPip() (string, error) {
	if pip, err := exec.LookPath("pip"); err == nil {
		return pip, nil
	} else {
		return "", errors.Errorf("pip not found")
	}
}

func (p *Pip) InstallDependencies(ctx context.Context, root, venvDir string, showOutput bool, infoWriter, errorWriter io.Writer) error {
	err := installDependenciesWithPip(ctx, root, venvDir, showOutput, infoWriter, errorWriter)
	return err
}

func (p *Pip) GetBinaryPath() (string, error) {
	if p.GetPath() == "" {
		path, err := getPip()
		if err != nil {
			return "", err
		}
		p.SetPath(path)
	}
	return p.GetPath(), nil
}

func (p *Pip) Prepare(ctx context.Context, root, venvDir string, showOutput bool, infoWriter, errorWriter io.Writer) error {
	err := createVenv(ctx, root, venvDir, showOutput, infoWriter, errorWriter)
	if err != nil {
		return err
	}
	return nil
}

func createVenv(ctx context.Context, root, venvDir string, showOutput bool, infoWriter, errorWriter io.Writer) error {
	print := func(message string) {
		if showOutput {
			fmt.Fprintf(infoWriter, "%s\n", message)
		}
	}
	print("Creating virtual environment...")
	// Create the virtual environment by running `python -m venv <venvDir>`.
	if !filepath.IsAbs(venvDir) {
		venvDir = filepath.Join(root, venvDir)
	}

	cmd, err := Command(ctx, "-m", "venv", venvDir)
	if err != nil {
		return err
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		if len(output) > 0 {
			fmt.Fprintf(errorWriter, "%s\n", string(output))
		}
		return errors.Wrapf(err, "creating virtual environment at %s", venvDir)
	}

	print("Finished creating virtual environment")
	return nil
}

func installDependenciesWithPip(ctx context.Context, root, venvDir string, showOutput bool, infoWriter, errorWriter io.Writer) error {
	print := func(message string) {
		if showOutput {
			fmt.Fprintf(infoWriter, "%s\n", message)
		}
	}
	runPipInstall := func(errorMsg string, arg ...string) error {
		pipCmd := VirtualEnvCommand(venvDir, "python", append([]string{"-m", "pip", "install"}, arg...)...)
		pipCmd.Dir = root
		pipCmd.Env = ActivateVirtualEnv(os.Environ(), venvDir)

		wrapError := func(err error) error {
			return errors.Wrapf(err, "%s via '%s'", errorMsg, strings.Join(pipCmd.Args, " "))
		}

		if showOutput {
			// Show stdout/stderr output.
			pipCmd.Stdout = infoWriter
			pipCmd.Stderr = errorWriter
			if err := pipCmd.Run(); err != nil {
				return wrapError(err)
			}
		} else {
			// Otherwise, only show output if there is an error.
			if output, err := pipCmd.CombinedOutput(); err != nil {
				if len(output) > 0 {
					fmt.Fprintf(errorWriter, "%s\n", string(output))
				}
				return wrapError(err)
			}
		}
		return nil
	}

	print("Updating pip, setuptools, and wheel in virtual environment...")

	err := runPipInstall("updating pip, setuptools, and wheel", "--upgrade", "pip", "setuptools", "wheel")
	if err != nil {
		return err
	}

	print("Finished updating")

	// If `requirements.txt` doesn't exist, exit early.
	requirementsPath := filepath.Join(root, "requirements.txt")
	if _, err := os.Stat(requirementsPath); os.IsNotExist(err) {
		return nil
	}

	print("Installing dependencies in virtual environment...")

	err = runPipInstall("installing dependencies", "-r", "requirements.txt")
	if err != nil {
		return err
	}
	return nil
}
