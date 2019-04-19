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

package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackOutputCmd() *cobra.Command {
	var jsonOut bool
	var stackName string

	cmd := &cobra.Command{
		Use:   "output [property-name]",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "Show a stack's output properties",
		Long: "Show a stack's output properties.\n" +
			"\n" +
			"By default, this command lists all output properties exported from a stack.\n" +
			"If a specific property-name is supplied, just that property's value is shown.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Fetch the current stack and its output properties.
			s, err := requireStack(stackName, false, opts, true /*setCurrent*/)
			if err != nil {
				return err
			}
			snap, err := s.Snapshot(commandContext())
			if err != nil {
				return err
			}

			outputs, err := getStackOutputs(snap)
			if err != nil {
				return errors.Wrap(err, "getting outputs")
			}
			if outputs == nil {
				outputs = make(map[string]interface{})
			}

			// If there is an argument, just print that property.  Else, print them all (similar to `pulumi stack`).
			if len(args) > 0 {
				name := args[0]
				v, has := outputs[name]
				if has {
					if jsonOut {
						if err := printJSON(v); err != nil {
							return err
						}
					} else {
						fmt.Printf("%v\n", stringifyOutput(v))
					}
				} else {
					return errors.Errorf("current stack does not have output property '%v'", name)
				}
			} else if jsonOut {
				if err := printJSON(outputs); err != nil {
					return err
				}
			} else {
				printStackOutputs(outputs)
			}
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&jsonOut, "json", "j", false, "Emit output as JSON")
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "", "The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

func getStackOutputs(snap *deploy.Snapshot) (map[string]interface{}, error) {
	state, err := stack.GetRootStackResource(snap)
	if err != nil {
		return nil, err
	}

	// TODO(ellismg): We probably want to adjust this interface slightly. Instead of just taking an encrypter, it
	// should take something that lests us control how SecretValues are handled. For example, we may by default want
	// to say that secret values are just returned as `[secret]` and if you pass --show-secrets we will show them. As
	// is, right now, you'd see weird JSON encoding of secret outputs. Note that in order to construct the snapshot
	// you had to have access to see the secrets, so we aren't disclosing anything you already didn't have access to
	// by passing config.NopEncrypter here.  But you will end up seeing the wire encoding of a property map which
	// isn't super intuitive.
	return stack.SerializeProperties(state.Outputs, config.NopEncrypter)
}
