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
	Name       string
	Definition YamlResourceDef
}
type YamlResourceDef struct {
	Type       string                 `json:"type" yaml:"type"`
	Properties map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
}

type YamlProject struct {
	// Name is a required fully qualified name.
	Name tokens.PackageName `json:"name" yaml:"name"`
	// Runtime is a required runtime that executes code.
	Runtime   string                     `json:"runtime" yaml:"runtime"`
	Resources map[string]YamlResourceDef `json:"resources,omitempty" yaml:"resources,omitempty"`
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

type ObjectArgs struct {
	children map[string]interface{} // either ObjectArgs | ListArgs
}

type ListArgs struct {
	children []interface{} // either ObjectArgs | ListArgs
}

func parseObj(args []string) interface{} {
	obj := make(map[string]interface{})

	openObj := 0
	openList := 0

	start := 0

argLoop:
	for i, k := range args {
		s := strings.SplitN(k, "=", 2)
		key, value := s[0], s[1]
		switch value {
		case "[[":
			if openList > 0 {
				// disregard as we are parsing a list
				continue argLoop
			}
			if openObj == 0 {
				start = i
			}
			openObj += 1
			continue argLoop
		case "]]":
			if openList > 0 {
				// disregard as we are parsing a list
				continue argLoop
			}
			openObj += -1
			if openObj == 0 {
				obj[key] = parseArgs(args[start : i+1])
			}
			continue argLoop
		case "[":
			if openObj > 0 {
				// disregard as we are parsing an object
				continue argLoop
			}
			if openList == 0 {
				start = i
			}
			openList += 1
			continue argLoop
		case "]":
			if openObj > 0 {
				// disregard as we are parsing an object
				continue argLoop
			}
			openList += -1
			if openList == 0 {
				obj[key] = parseArgs(args[start : i+1])
			}
			continue argLoop
		}
		if openObj > 0 || openList > 0 {
			continue argLoop
		}
		obj[key] = value
	}
	return obj
}

func parseList(args []string) interface{} {
	lst := make([]interface{}, 0, len(args))

	openObj := 0
	openList := 0

	start := 0

argLoop:
	for i, k := range args {
		s := strings.Split(k, "=")
		_, value := s[0], s[len(s)-1]
		switch value {
		case "[[":
			if openList > 0 {
				// disregard as we are parsing a list
				continue argLoop
			}
			if openObj == 0 {
				start = i
			}
			openObj += 1
			continue argLoop
		case "]]":
			if openList > 0 {
				// disregard as we are parsing a list
				continue argLoop
			}
			openObj += -1
			if openObj == 0 {
				lst = append(lst, parseArgs(args[start:i+1]))
			}
			continue argLoop
		case "[":
			if openObj > 0 {
				// disregard as we are parsing an lst
				continue argLoop
			}
			if openList == 0 {
				start = i
			}
			openList += 1
			continue argLoop
		case "]":
			if openObj > 0 {
				// disregard as we are parsing an lst
				continue argLoop
			}
			openList += -1
			if openList == 0 {
				lst = append(lst, parseArgs(args[start:i+1]))
			}
			continue argLoop
		}
		if openObj > 0 || openList > 0 {
			continue argLoop
		}
		lst = append(lst, k)
	}
	return lst
}

func parseArgs(a []string) interface{} {
	// minimum size of 2
	args := make([]string, 0, len(a))
	s := strings.SplitN(a[0], "=", 2)
	prefix := s[0]

	for i, arg := range a {
		args = append(args, strings.TrimPrefix(arg, prefix+"="))
		fmt.Println(args[i])
	}

	if args[0] == "[[" {
		return parseObj(args[1 : len(args)-1])
	} else if args[0] == "[" {
		return parseList(args[1 : len(args)-1])
	}
	panic("should not get here")
}

func parseResource(args []string) (YamlResource, error) {
	/*
	 */
	resourceName, resourceType, props := args[1], args[0], args[2:]
	fmt.Println("args", props)

	newProps := make([]string, 0, len(props)+2)

	newProps = append(newProps, "=[[")
	for _, prop := range props {
		newProps = append(newProps, "="+prop)
	}
	newProps = append(newProps, "=[[")

	p := parseObj(props)

	finalProps, ok := p.(map[string]interface{})
	if !ok {
		return YamlResource{}, fmt.Errorf("failed to parse args")
	}
	// args are implied to be in an object
	return YamlResource{
		Name: resourceName,
		Definition: YamlResourceDef{
			Type:       resourceType,
			Properties: finalProps,
		},
	}, nil
}

// intentionally disabling here for cleaner err declaration/assignment.
// nolint: vetshadow
func newDeployCmd() *cobra.Command {
	// up implementation used when the source of the Pulumi program is in the current working directory.

	ensureProject := func(res YamlResource) error {
		/*
		 */
		home, err := workspace.GetPulumiHomeDir()

		d := singletonDeployment{
			resourceName: res.Name,
			resourceType: res.Definition.Type,
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
		resources := make(map[string]YamlResourceDef)
		resources[res.Name] = res.Definition

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

			res, err := parseResource(args)
			if err != nil {
				return result.FromError(err)
			}

			if err := ensureProject(res); err != nil {
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
