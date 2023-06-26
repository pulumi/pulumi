// Copyright 2016-2023, Pulumi Corporation.
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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/spf13/cobra"
)

func getDeployment(s backend.Stack) (apitype.DeploymentV3, error) {
	ctx := commandContext()
	var depV3 *apitype.DeploymentV3
	// Export the latest version of the checkpoint by default. Otherwise, we require that
	// the backend/stack implements the ability the export previous checkpoints.
	deployment, err := s.ExportDeployment(ctx)
	if err != nil {
		return apitype.DeploymentV3{}, err
	}
	switch deployment.Version {
	case 3:
		_ = json.Unmarshal(deployment.Deployment, &depV3)
	default:
		// Unsupported Version
		return apitype.DeploymentV3{}, fmt.Errorf("unsupported deployment version %d", deployment.Version)
	}
	if depV3 == nil {
		return apitype.DeploymentV3{}, fmt.Errorf("unable to unmarshal state")
	}
	return *depV3, nil
}

func handleNoEditor(filename string) {
	var response string
	fmt.Printf("EDITOR environment variable is not set. You can edit the file manually at: %s\n", filename)
	surveycore.DisableColor = true
	surveyIcons := survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question = survey.Icon{}
		icons.SelectFocus = survey.Icon{Text: cmdutil.GetGlobalColorization().Colorize(colors.BrightGreen + ">" + colors.Reset)}
	})
	if err := survey.AskOne(&survey.Select{
		Message: "When you are done editing the file, select continue to preview the changes.",
		Options: []string{"continue"},
	}, &response, surveyIcons); err != nil {
		fmt.Println("cancelled")
		os.Exit(1)
	}
}

func openInEditor(filename string) {
	if editor := os.Getenv("EDITOR"); editor != "" {
		cmd := exec.Command(editor, filename)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("Failed to exec EDITOR: %v\n", err)
			os.Exit(1)
		}
	} else {
		handleNoEditor(filename)
	}
}

func formatDiff(olds, news deploy.Snapshot) string {

	type state struct {
		inputs  resource.PropertyMap
		outputs resource.PropertyMap
	}

	oldStates := map[resource.URN]state{}
	newStates := map[resource.URN]state{}

	urns := map[resource.URN]struct{}{}

	for _, old := range olds.Resources {
		oldStates[old.URN] = state{
			inputs:  old.Inputs,
			outputs: old.Outputs,
		}
		urns[old.URN] = struct{}{}
	}

	for _, new := range news.Resources {
		newStates[new.URN] = state{
			inputs:  new.Inputs,
			outputs: new.Outputs,
		}
		urns[new.URN] = struct{}{}
	}

	msg := ""
	for urn := range urns {
		old, hasOld := oldStates[urn]
		new, hasNew := newStates[urn]

		if !hasOld && hasNew {
			msg += fmt.Sprintf("Added resource: %s\n", urn)
		}
		if !hasNew && hasOld {
			msg += fmt.Sprintf("Deleted resource: %s\n", urn)
		} else {
			var displayedHeader bool
			header := fmt.Sprintf("Changed resource: %s\n", urn)
			if diff := old.inputs.Diff(new.inputs, resource.IsInternalPropertyKey); diff != nil {
				b := &bytes.Buffer{}
				display.PrintObjectDiff(b, *diff, nil, false, 4, false, false, false)
				if m := b.String(); m != "" {
					msg += header
					msg += "    inputs:\n"
					msg += m
					displayedHeader = true
				}
			}
			if diff := old.outputs.Diff(new.outputs, resource.IsInternalPropertyKey); diff != nil {
				b := &bytes.Buffer{}
				display.PrintObjectDiff(b, *diff, nil, false, 4, false, false, false)
				if m := b.String(); m != "" {
					if !displayedHeader {
						msg += header
					}
					msg += "    outputs:\n"
					msg += m
				}
			}
		}
	}
	return cmdutil.GetGlobalColorization().Colorize(msg)
}

func newStateEditCommand() *cobra.Command {
	var useJson bool
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit the current stack's state",
		Long: `Edit the current stack's state

Subcommands of this command can be used to surgically edit parts of a stack's state. These can be useful when
troubleshooting a stack or when performing specific edits that otherwise would require editing the state file by hand.`,
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			ctx := commandContext()
			stackName := ""

			// Fetch the current stack and export its deployment
			s, err := requireStack(ctx, stackName, stackLoadOnly, display.Options{IsInteractive: false})
			if err != nil {
				return result.FromError(err)
			}
			olds, err := s.Snapshot(ctx, stack.DefaultSecretsProvider)
			if err != nil {
				return result.FromError(err)
			}

			if olds == nil {
				return result.FromError(errors.New("could not read stack's current deployment"))
			}

			deployment, err := getDeployment(s)
			if err != nil {
				return result.FromError(err)
			}

			var sf stateFrontend = &yamlStateFrontend{}
			if useJson {
				sf = &jsonStateFrontend{}
			}

			sf.SaveToFile(deployment)
			for {
				failed := false
				openInEditor(sf.GetBackingFile())
				// Read the filename into a deployment v3
				news, err := sf.ReadNewSnapshot()
				if err != nil {
					failed = true
					fmt.Println(err)
				}

				if err := news.VerifyIntegrity(); err != nil {
					failed = true
					fmt.Println(err)
				}

				// Survey user
				edit := "edit"
				reset := "reset"
				cancel := "cancel"
				var response string
				options := []string{
					// `accept` is only added if there is no error.
					edit,
					reset,
					cancel,
				}

				accept := "accept"
				if failed {
					// There was an error, so we can't add the yes option
					// Print the edited state is not valid in RED
					fmt.Println(cmdutil.GetGlobalColorization().Colorize(colors.BrightRed + "Error: Edited state is not valid" + colors.Reset))
				} else {
					msg, err := sf.Diff()
					if err != nil {
						return result.FromError(err)
					}

					fmt.Printf("Previewing changes on `%s`\n", s.Ref().FullyQualifiedName())
					if msg != "" {
						fmt.Println(msg)
						options = append([]string{accept}, options...)
					} else {
						fmt.Printf("warning: no changes `%s`\n", s.Ref().FullyQualifiedName())
					}
				}

				// Now prompt the user for a yes, no, or details, and then proceed accordingly.
				surveycore.DisableColor = true
				surveyIcons := survey.WithIcons(func(icons *survey.IconSet) {
					icons.Question = survey.Icon{}
					icons.SelectFocus = survey.Icon{Text: cmdutil.GetGlobalColorization().Colorize(colors.BrightGreen + ">" + colors.Reset)}
				})
				if err := survey.AskOne(&survey.Select{
					Message: "Choices",
					Options: options,
					Default: options[1],
				}, &response, surveyIcons); err != nil {
					return result.FromError(fmt.Errorf("confirmation cancelled, not proceeding with the state edit: %w", err))
				}

				switch response {
				case accept:
					err := saveSnapshot(ctx, s, &news, false)
					if err != nil {
						return result.FromError(err)
					}
					break
				case edit:
					continue
				case reset:
					if err := sf.Reset(); err != nil {
						return result.FromError(err)
					}
					continue
				case cancel:
					return nil
				}
			}
		}),
	}
	cmd.PersistentFlags().BoolVar(
		&useJson, "json", false,
		"Remove the stack and its config file after all resources in the stack have been deleted")
	return cmd
}
