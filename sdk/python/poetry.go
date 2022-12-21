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

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/yamlutil"
	"gopkg.in/yaml.v3"
)

type Poetry struct {
	SDependencyTool
}

func promptInstallPoetry() error {
	fmt.Println("Poetry not found. Please install Poetry to continue.")
	fmt.Println("See https://python-poetry.org/docs/#installation for installation instructions.")
	fmt.Println("Once installed, run `pulumi up` again.")
	return errors.Errorf("Poetry not found")
}

func createPoetry() (string, error) {
	var poetryPath string
	var err error
	if poetryPath, err = exec.LookPath("poetry"); err != nil {
		if err := promptInstallPoetry(); err != nil {
			return "", err
		}
	}
	return poetryPath, nil
}

func (p *Poetry) Prepare(ctx context.Context, root, venvDir string, showOutput bool, infoWriter, errorWriter io.Writer) error {
	_, err := createPoetry()
	if err != nil {
		return err
	}
	_, err = p.GetBinaryPath()
	if err != nil {
		return err
	}
	return err
}

func (p *Poetry) GetBinaryPath() (string, error) {
	if p.GetPath() == "" {
		path, err := createPoetry()
		if err != nil {
			return "", err
		}
		p.SetPath(path)
	}
	return p.GetPath(), nil
}

func (p *Poetry) runCommand(ctx context.Context, root string, args []string) (*exec.Cmd, error) {
	path, err := p.GetBinaryPath()
	if err != nil {
		return nil, err
	}
	poetryCmd := exec.CommandContext(ctx, path, args...)
	poetryCmd.Dir = root
	poetryCmd.Env = os.Environ()
	return poetryCmd, nil
}

type RuntimeConfig struct {
	Name    string            `yaml:"name"`
	Options map[string]string `yaml:"options"`
}

func (p *Poetry) InstallDependencies(ctx context.Context, root, venvDir string, showOutput bool, infoWriter, errorWriter io.Writer) error {
	installCommand, err := p.runCommand(ctx, root, []string{"install"})
	if err != nil {
		return err
	}
	output, err := installCommand.CombinedOutput()
	if err != nil {
		return err
	}
	fmt.Fprintf(infoWriter, "%s\n", string(output))

	poetryEnvCmd, err := p.runCommand(ctx, root, []string{"env", "info", "-p"})
	if err != nil {
		return err
	}
	poetryEnvPath, err := poetryEnvCmd.CombinedOutput()
	if err != nil {
		return err
	}
	pulumiYamlFilepath := filepath.Join(root, "Pulumi.yaml")
	filedata, err := os.ReadFile(pulumiYamlFilepath)
	if err != nil {
		return err
	}
	var workspaceDocument yaml.Node
	err = yaml.Unmarshal(filedata, &workspaceDocument)
	if err != nil {
		return err
	}
	runtime, found, err := yamlutil.Get(&workspaceDocument, "runtime")
	if err != nil {
		return err
	}
	if found {
		options, found, err := yamlutil.Get(runtime, "options")
		if err != nil {
			return err
		}
		if found {
			virtualenv, found, err := yamlutil.Get(options, "virtualenv")
			if err != nil {
				return err
			}
			if found {
				virtualenv.Value = string(poetryEnvPath)
			} else {
				err = yamlutil.Insert(options, "virtualenv", string(poetryEnvPath))
				if err != nil {
					return err
				}
			}
		} else {
			virtualEnvMap := make(map[string]string)
			virtualEnvMap["virtualenv"] = string(poetryEnvPath)
			marshalledVirtualenv, err := yaml.Marshal(virtualEnvMap)
			if err != nil {
				return err
			}
			err = yamlutil.Insert(runtime, "options", string(marshalledVirtualenv))
			if err != nil {
				return err
			}
		}
	} else {
		runtimeConfig := RuntimeConfig{
			Name: "python",
			Options: map[string]string{
				"virtualenv": string(poetryEnvPath),
			},
		}
		marshalledVirtualenv, err := yaml.Marshal(runtimeConfig)
		if err != nil {
			return err
		}

		err = yamlutil.Insert(&workspaceDocument, "runtime", string(marshalledVirtualenv))
		if err != nil {
			return err
		}
	}
	bytes, err := yamlutil.Edit(filedata, &workspaceDocument)
	if err != nil {
		return err
	}
	err = os.WriteFile(pulumiYamlFilepath, bytes, 0600)
	if err != nil {
		return err
	}
	return nil
}
