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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"
	dyff "github.com/dixler/dyff/pkg/pulumi"
	"github.com/hashicorp/go-multierror"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newStackImportCmd() *cobra.Command {
	var force bool
	var file string
	var stackName string
	cmd := &cobra.Command{
		Use:   "import",
		Args:  cmdutil.MaximumNArgs(0),
		Short: "Import a deployment from standard in into an existing stack",
		Long: "Import a deployment from standard in into an existing stack.\n" +
			"\n" +
			"A deployment that was exported from a stack using `pulumi stack export` and\n" +
			"hand-edited to correct inconsistencies due to failed updates, manual changes\n" +
			"to cloud resources, etc. can be reimported to the stack using this command.\n" +
			"The updated deployment will be read from standard in.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Fetch the current stack and import a deployment.
			s, err := requireStack(ctx, stackName, stackLoadOnly, opts)
			if err != nil {
				return err
			}
			stackName := s.Ref().Name()

			// Read from stdin or a specified file
			reader := os.Stdin
			if file != "" {
				reader, err = os.Open(file)
				if err != nil {
					return fmt.Errorf("could not open file: %w", err)
				}
			}

			// Read the checkpoint from stdin.  We decode this into a json.RawMessage so as not to lose any fields
			// sent by the server that the client CLI does not recognize (enabling round-tripping).
			var deployment apitype.UntypedDeployment
			if err = json.NewDecoder(reader).Decode(&deployment); err != nil {
				return err
			}

			// We do, however, now want to unmarshal the json.RawMessage into a real, typed deployment.  We do this so
			// we can check that the deployment doesn't contain resources from a stack other than the selected one. This
			// catches errors wherein someone imports the wrong stack's deployment (which can seriously hork things).
			snapshot, err := stack.DeserializeUntypedDeployment(ctx, &deployment, stack.DefaultSecretsProvider)
			if err != nil {
				return checkDeploymentVersionError(err, stackName.String())
			}
			original, err := s.ExportDeployment(ctx)
			if err == nil && cmdutil.Interactive() {
				diagnostics := ""
				if err := snapshot.VerifyIntegrity(); err != nil {
					diagnostics += fmt.Sprintf("The edited state is not valid: %v\n", err)
				}

				// Survey user
				cancel := "cancel"
				var response string
				options := []string{
					// `accept` is only added if there is no error.
					cancel,
				}
				// Print a banner so it's clear this is going to the cloud.
				fmt.Printf(cmdutil.GetGlobalColorization().Colorize(
					colors.SpecHeadline+"Previewing stack import (%s)"+colors.Reset+"\n\n"), s.Ref().FullyQualifiedName())

				accept := "accept"
				if diagnostics != "" {
					// There was an error, so we can't add the yes option
					// Print the edited state is not valid in RED
					fmt.Println(cmdutil.GetGlobalColorization().Colorize(colors.BrightRed + "Error: Edited state is not valid" + colors.Reset))
				} else {
					// make original.Deployment and deployment.Deployment into temp files with filenames.
					originalFile, err := ioutil.TempFile("", "original")
					if err != nil {
						return err
					}
					defer os.Remove(originalFile.Name())
					_, err = originalFile.Write(original.Deployment)
					if err != nil {
						return err
					}

					newFile, err := ioutil.TempFile("", "new")
					if err != nil {
						return err
					}
					defer os.Remove(newFile.Name())
					_, err = newFile.Write(deployment.Deployment)
					if err != nil {
						return err
					}

					// run diff on them
					msg, err := dyff.Compare(originalFile.Name(), newFile.Name())
					if err != nil {
						diagnostics += fmt.Sprintf("unable to compute diff: %v\n", err)
					} else {

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
				}

				// Now prompt the user for a yes, no, or details, and then proceed accordingly.
				// Create a prompt. If this is a refresh, we'll add some extra text so it's clear we aren't updating resources.
				var prompt string
				if diagnostics != "" {
					fmt.Printf(cmdutil.GetGlobalColorization().Colorize(colors.BrightRed + diagnostics + colors.Reset))
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
					Default: cancel,
				}, &response, surveyIcons); err != nil {
					return fmt.Errorf("confirmation cancelled, not proceeding with the stack import: %w", err)
				}

				if response == cancel {
					return nil
				}
			}

			if err := saveSnapshot(ctx, s, snapshot, force); err != nil {
				return err
			}
			fmt.Printf("Import complete.\n")
			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "", "The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"Force the import to occur, even if apparent errors are discovered beforehand (not recommended)")
	cmd.PersistentFlags().StringVarP(
		&file, "file", "", "", "A filename to read stack input from")

	return cmd
}

func saveSnapshot(ctx context.Context, s backend.Stack, snapshot *deploy.Snapshot, force bool) error {
	stackName := s.Ref().Name()
	var result error
	for _, res := range snapshot.Resources {
		if res.URN.Stack() != stackName.Q() {
			msg := fmt.Sprintf("resource '%s' is from a different stack (%s != %s)",
				res.URN, res.URN.Stack(), stackName)
			if force {
				// If --force was passed, just issue a warning and proceed anyway.
				// Note: we could associate this diagnostic with the resource URN
				// we have.  However, this sort of message seems to be better as
				// something associated with the stack as a whole.
				cmdutil.Diag().Warningf(diag.Message("" /*urn*/, msg))
			} else {
				// Otherwise, gather up an error so that we can quit before doing damage.
				result = multierror.Append(result, errors.New(msg))
			}
		}
	}
	// Validate the stack. If --force was passed, issue an error if validation fails. Otherwise, issue a warning.
	if err := snapshot.VerifyIntegrity(); err != nil {
		msg := fmt.Sprintf("state file contains errors: %v", err)
		if force {
			cmdutil.Diag().Warningf(diag.Message("", msg))
		} else {
			result = multierror.Append(result, errors.New(msg))
		}
	}
	if result != nil {
		return multierror.Append(result,
			errors.New("importing this file could be dangerous; rerun with --force to proceed anyway"))
	}

	// Explicitly clear-out any pending operations.
	if snapshot.PendingOperations != nil {
		for _, op := range snapshot.PendingOperations {
			msg := fmt.Sprintf(
				"removing pending operation '%s' on '%s' from snapshot", op.Type, op.Resource.URN)
			cmdutil.Diag().Warningf(diag.Message(op.Resource.URN, msg))
		}

		snapshot.PendingOperations = nil
	}
	sdp, err := stack.SerializeDeployment(snapshot, snapshot.SecretsManager, false /* showSecrets */)
	if err != nil {
		return fmt.Errorf("constructing deployment for upload: %w", err)
	}

	bytes, err := json.Marshal(sdp)
	if err != nil {
		return err
	}

	dep := apitype.UntypedDeployment{
		Version:    apitype.DeploymentSchemaVersionCurrent,
		Deployment: bytes,
	}

	// Now perform the deployment.
	if err = s.ImportDeployment(ctx, &dep); err != nil {
		return fmt.Errorf("could not import deployment: %w", err)
	}
	return nil
}
