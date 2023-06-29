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
	"strconv"
	"strings"
	"sync"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
	fmt.Printf("no EDITOR environment variable detected.\nchanges will be read from: `%s`\n", filename)
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

func yamlHistory(history apitype.UpdateInfo) (string, error) {
	b, err := json.Marshal(history)
	contract.AssertNoError(err)

	idk := map[string]interface{}{}
	err = json.Unmarshal(b, &idk)
	contract.AssertNoError(err)

	writer := &bytes.Buffer{}
	if err != nil {
		return "", fmt.Errorf("could not open file: %w", err)
	}

	enc := yaml.NewEncoder(writer)
	enc.SetIndent(2)
	if err = enc.Encode(idk); err != nil {
		return "", fmt.Errorf("could not serialize deployment as YAML : %w", err)
	}

	{
		b, err := sortYAML(writer.Bytes())
		if err != nil {
			return "", err
		}

		return string(b), nil
	}
}

func newStateEditCommand() *cobra.Command {
	var useJson bool
	var maxHistory int
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

			if maxHistory < 0 {
				return result.FromError(errors.New("--max-history must be >= 0"))
			}
			if useJson && maxHistory > 0 {
				return result.FromError(errors.New("--max-history cannot be used with json"))
			}

			// Fetch the current stack and export its deployment
			s, err := requireStack(ctx, stackName, stackLoadOnly, display.Options{IsInteractive: false})
			if err != nil {
				return result.FromError(err)
			}

			var sf stateFrontend
			if useJson {
				sf = &jsonStateFrontend{}
			} else {
				ysf := &yamlStateFrontend{
					header: `# Make your edits to the state below.
# To continue deploying the stack, save your changes and exit the editor.
# You will be able to preview your changes before deploying the update.
# If you specified --max-history, previous states will be appended to the bottom of this YAML file
# as additional documents, but only the top-most state will be considered.

`,
				}
				if maxHistory > 0 {
					// Check that the stack and its backend supports the ability to do this.
					be, ok := s.Backend().(httpstate.Backend)
					if !ok {
						return result.FromError(
							fmt.Errorf("the current backend (%s) does not provide the ability to export previous deployments",
								be.Name()),
						)
					}

					specificExpBE, ok := be.(backend.SpecificDeploymentExporter)
					if !ok {
						return result.FromError(
							fmt.Errorf("the current backend (%s) does not provide the ability to export previous deployments",
								be.Name()),
						)
					}

					// Try to read the current project
					project, _, err := readProject()
					if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
						return result.FromError(err)
					}

					cloudURL, err := workspace.GetCurrentCloudURL(project)
					if err != nil {
						return result.FromError(
							fmt.Errorf("could not determine current cloud: %w", err),
						)
					}

					account, err := workspace.GetAccount(cloudURL)
					if err != nil {
						return result.FromError(
							fmt.Errorf("getting stored credentials: %w", err),
						)
					}
					pc := client.NewClient(cloudURL, account.AccessToken, false, nil)

					parts := strings.Split(s.Ref().FullyQualifiedName().String(), "/")
					if len(parts) != 3 {
						return result.FromError(
							fmt.Errorf("invalid stack reference: %s", s.Ref().String()),
						)
					}
					orgName, projectName, stackName := parts[0], parts[1], parts[2]

					hs, err := pc.GetStackUpdates(ctx, client.StackIdentifier{
						Owner:   orgName,
						Project: projectName,
						Stack:   stackName,
					}, maxHistory, 0)

					if err != nil {
						return result.FromError(
							fmt.Errorf("failed to get stack updates: %w", err),
						)
					}
					history := make([]string, len(hs))
					wg := sync.WaitGroup{}
					for i, h := range hs {
						i, h := i, h
						wg.Add(1)
						go func() {
							defer wg.Done()
							var dep *apitype.UntypedDeployment
							dep, err = specificExpBE.ExportDeploymentForVersion(ctx, s, strconv.Itoa(h.Version))
							h.Deployment = dep.Deployment
							entry, err := yamlHistory(h)
							if err != nil {
								entry = fmt.Sprintf("# Unable to export previous deployment %d\n", h.Version)
							}
							history[i] = entry
						}()
					}
					wg.Wait()
					ysf.footer = "---\n" + strings.Join(history, "---\n")
				}
				sf = ysf
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

				// Print a banner so it's clear this is going to the cloud.
				fmt.Printf(cmdutil.GetGlobalColorization().Colorize(
					colors.SpecHeadline+"Previewing edit (%s)"+colors.Reset+"\n\n"), s.Ref().FullyQualifiedName())

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

					if strings.TrimSpace(msg) != "" {
						fmt.Printf(cmdutil.GetGlobalColorization().Colorize(
							colors.SpecHeadline + "Changes:" + colors.Reset + "\n"))
						cleaned := msg
						if cleaned[0] == '\n' {
							cleaned = cleaned[1:]
						}
						cleaned = "    " + cleaned
						cleaned = strings.Replace(cleaned, "\n", "\n    ", -1)
						fmt.Print(cleaned)
						options = append([]string{accept}, options...)
					} else {
						fmt.Printf("warning: no changes `%s`\n", s.Ref().FullyQualifiedName())
					}
				}

				// Now prompt the user for a yes, no, or details, and then proceed accordingly.
				// Create a prompt. If this is a refresh, we'll add some extra text so it's clear we aren't updating resources.
				var prompt string
				if failed {
					prompt = "\b" + cmdutil.GetGlobalColorization().Colorize(
						colors.SpecPrompt+"What would you like to do?"+colors.Reset)
				} else {
					prompt = "\b" + cmdutil.GetGlobalColorization().Colorize(
						colors.SpecPrompt+"Do you want to perform this edit?"+colors.Reset)
				}
				surveycore.DisableColor = true
				surveyIcons := survey.WithIcons(func(icons *survey.IconSet) {
					icons.Question = survey.Icon{}
					icons.SelectFocus = survey.Icon{Text: cmdutil.GetGlobalColorization().Colorize(colors.BrightGreen + ">" + colors.Reset)}
				})
				if err := survey.AskOne(&survey.Select{
					Message: prompt,
					Options: options,
					Default: edit,
				}, &response, surveyIcons); err != nil {
					return result.FromError(fmt.Errorf("confirmation cancelled, not proceeding with the state edit: %w", err))
				}

				switch response {
				case accept:
					err := saveSnapshot(ctx, s, &news, false)
					if err != nil {
						return result.FromError(err)
					}
					return nil
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
	cmd.PersistentFlags().IntVarP(
		&maxHistory, "max-history", "n", 0,
		"Maximum number of history states to embed in the edit. (incompatible with --json")
	cmd.PersistentFlags().BoolVar(
		&useJson, "json", false,
		"Remove the stack and its config file after all resources in the stack have been deleted")
	return cmd
}
