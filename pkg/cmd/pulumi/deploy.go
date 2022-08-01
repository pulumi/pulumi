// Copyright 2016-2018, Pulumi Corporation.
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

package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"strings"

	yaml "gopkg.in/yaml.v3"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type YamlResource struct {
	Type       string                 `json:"type" yaml:"type"`
	Properties map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
}

type YamlProject struct {
	// Name is a required fully qualified name.
	Name tokens.PackageName `json:"name" yaml:"name"`
	// Runtime is a required runtime that executes code.
	Runtime   string                  `json:"runtime" yaml:"runtime"`
	Resources map[string]YamlResource `json:"resources,omitempty" yaml:"resources,omitempty"`
}

type singletonDeployment struct {
	resourceType string
	resourceName string
}

func b64encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func (d *singletonDeployment) ProjectName() string {
	return strings.Join([]string{"global-stack", b64encode(d.resourceName), b64encode(d.resourceType)}, "_")
}

func (d *singletonDeployment) ProjectDir() string {
	// TODO escape
	return strings.Join([]string{"global-stack", d.resourceName, d.resourceType}, "_")
}

// intentionally disabling here for cleaner err declaration/assignment.
// nolint: vetshadow
func newDeployCmd() *cobra.Command {
	// up implementation used when the source of the Pulumi program is in the current working directory.

	ensureProject := func() error {
		/*
		 */
		home, err := workspace.GetPulumiHomeDir()
		resourceName, resourceType := "my-bucket", "gcp:storage:Bucket"

		d := singletonDeployment{
			resourceName: resourceName,
			resourceType: resourceType,
		}
		p := path.Join(home, "global-stacks", d.ProjectDir())
		if err := os.Chdir(p); err != nil {
			if err := os.MkdirAll(p, 0744); err != nil {
				return err
			}
			if err := os.Chdir(p); err != nil {
				return err
			}
		}
		resources := make(map[string]YamlResource)
		resources[resourceName] = YamlResource{
			Type:       resourceType,
			Properties: nil,
		}
		proj := YamlProject{
			Name:      tokens.PackageName(d.ProjectName()),
			Runtime:   "yaml",
			Resources: resources,
		}
		projectText, err := yaml.Marshal(proj)
		if err != nil {
			return err
		}
		os.WriteFile("Pulumi.yaml", []byte(projectText), 0744)

		opts := display.Options{
			Color:         cmdutil.GetGlobalColorization(),
			IsInteractive: false,
		}

		b, err := currentBackend(opts)
		if err != nil {
			return err
		}

		fmt.Printf("%v %v\n", b, d)
		stackRef, parseErr := b.ParseStackReference(strings.Join([]string{d.ProjectName(), "global"}, "/"))
		if parseErr != nil {
			return parseErr
		}

		s, err := createStack(b, stackRef, nil, true, "")
		contract.Ignore(s)
		contract.IgnoreError(err)
		return nil
	}

	var cmd = &cobra.Command{
		Use:   "deploy <resource-type> <resource-name> <args>",
		Short: "WIP",
		Args:  cmdutil.MinimumNArgs(2),
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			err := ensureProject()
			if err != nil {
				result.FromError(err)
			}
			up := newUpCmd()
			up.SetArgs([]string{
				"-s", "global",
			})
			up.Execute()
			return nil
		}),
	}
	return cmd
}
