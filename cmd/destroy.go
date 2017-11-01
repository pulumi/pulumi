// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/archive"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newDestroyCmd() *cobra.Command {
	if usePulumiCloudCommands() {
		return newCloudDestroyCmd()
	}
	return newFAFDestroyCmd()
}

func newFAFDestroyCmd() *cobra.Command {
	var debug bool
	var preview bool
	var stack string
	var parallel int
	var summary bool
	var yes bool
	var cmd = &cobra.Command{
		Use:        "destroy",
		SuggestFor: []string{"down", "remove"},
		Short:      "Destroy an existing stack and its resources",
		Long: "Destroy an existing stack and its resources\n" +
			"\n" +
			"This command deletes an entire existing stack by name.  The current state is\n" +
			"loaded from the associated snapshot file in the workspace.  After running to completion,\n" +
			"all of this stack's resources and associated state will be gone.\n" +
			"\n" +
			"Warning: although old snapshots can be used to recreate an stack, this command\n" +
			"is generally irreversable and should be used with great care.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			var backend pulumiBackend = &localPulumiBackend{}

			stackName, err := explicitOrCurrent(stack)
			if err != nil {
				return err
			}

			if preview || yes ||
				confirmPrompt("This will permanently destroy all resources in the '%v' stack!", stackName.String()) {

				return backend.Destroy(stackName, debug, engine.DestroyOptions{
					DryRun:   preview,
					Parallel: parallel,
					Summary:  summary,
				})
			}

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().BoolVarP(
		&preview, "preview", "n", false,
		"Don't actually delete resources; just preview the planned deletions")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose an stack other than the currently selected one")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
	cmd.PersistentFlags().BoolVar(
		&summary, "summary", false,
		"Only display summarization of resources and plan operations")
	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Skip confirmation prompts, and proceed with the destruction anyway")

	return cmd
}

func newCloudDestroyCmd() *cobra.Command {
	var stack string
	var yes bool

	var cmd = &cobra.Command{
		Use:        "destroy",
		SuggestFor: []string{"down", "remove"},
		Short:      "Destroy an existing stack and its resources",
		Long: "Destroy an existing stack and its resources\n" +
			"\n" +
			"This command deletes an entire existing stack by name.  After running to\n" +
			" completion, all of this stack's resources and associated state will be gone.\n" +
			"\n" +
			"Warning: this command is irreversable and should be used with great care.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Look up the owner, repository, and project from the workspace and nearest package.
			w, err := newWorkspace()
			if err != nil {
				return err
			}
			projID, err := getCloudProjectIdentifier(w)
			if err != nil {
				return err
			}

			// Default to the workspace settings if stack isn't provided.
			stackName := tokens.QName(stack)
			if stackName == "" {
				stackName = w.Settings().Stack
			}
			if stackName == "" {
				return errors.New("stack argument not set and workspace does not have selected stack")
			}

			// Zip up the Pulumi program's directory, which may be a parent of CWD.
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %v", err)
			}
			programPath, err := workspace.DetectPackage(cwd)
			if err != nil {
				return fmt.Errorf("looking for Pulumi package: %v", err)
			}
			if programPath == "" {
				return fmt.Errorf("no Pulumi package found")
			}
			// programPath is the path to the pulumi.yaml file. Need its parent folder.
			programFolder := filepath.Dir(programPath)
			archive, err := archive.EncodePath(programFolder)
			if err != nil {
				return fmt.Errorf("creating archive: %v", err)
			}

			if yes ||
				confirmPrompt("This will permanently destroy all resources in the '%v' stack!", stackName.String()) {

				// Gather up configuration.
				// TODO(pulumi-service/issues/221): Have pulumi.com handle the encryption/decryption.
				textConfig, err := getDecryptedConfig(stackName)
				if err != nil {
					return errors.Wrap(err, "getting decrypted configuration")
				}

				// Destroy the program in the Pulumi Cloud. Uses same API shape as update.
				updateRequest := apitype.UpdateProgramRequest{
					ProgramArchive: archive,
					Config:         textConfig,
				}
				var updateResponse apitype.UpdateProgramResponse
				path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s/destroy",
					projID.Owner, projID.Repository, projID.Project, string(stackName))
				if err = pulumiRESTCall("POST", path, &updateRequest, &updateResponse); err != nil {
					return err
				}
				fmt.Printf("Destroying Stack '%s'...\n", string(stackName))

				// Wait for the update to complete.
				status, err := waitForUpdate(path)
				fmt.Println() // The PPC's final message we print to STDOUT doesn't include a newline.

				if err != nil {
					return fmt.Errorf("waiting for destroy: %v", err)
				}
				if status == apitype.StatusSucceeded {
					fmt.Println("destroy complete.")
					return nil
				}
				return fmt.Errorf("destroy unsuccessful: status %v", status)
			}
			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose an stack other than the currently selected one")

	return cmd
}
