// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/archive"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newPreviewCmd() *cobra.Command {
	if usePulumiCloudCommands() {
		return newCloudPreviewCmd()
	}
	return newFAFPreviewCmd()
}

func newFAFPreviewCmd() *cobra.Command {
	var analyzers []string
	var debug bool
	var stack string
	var parallel int
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var summary bool
	var cmd = &cobra.Command{
		Use:        "preview",
		SuggestFor: []string{"plan"},
		Short:      "Show a preview of updates to an stack's resources",
		Long: "Show a preview of updates an stack's resources\n" +
			"\n" +
			"This command displays a preview of the updates to an existing stack whose state is\n" +
			"represented by an existing snapshot file. The new desired state is computed by compiling\n" +
			"and evaluating an executable package, and extracting all resource allocations from its\n" +
			"resulting object graph. These allocations are then compared against the existing state to\n" +
			"determine what operations must take place to achieve the desired state. No changes to the\n" +
			"stack will actually take place.\n" +
			"\n" +
			"The package to execute is loaded from the current directory. Use the `-C` or `--cwd` flag to\n" +
			"use a different directory.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			var backend pulumiBackend = &localPulumiBackend{}

			stackName, err := explicitOrCurrent(stack)
			if err != nil {
				return err
			}

			return backend.Preview(stackName, debug, engine.PreviewOptions{
				Analyzers:            analyzers,
				Parallel:             parallel,
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSames:            showSames,
				Summary:              summary,
			})
		}),
	}

	cmd.PersistentFlags().StringSliceVar(
		&analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this preview")
	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose an stack other than the currently selected one")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&summary, "summary", false,
		"Only display summarization of resources and operations")

	return cmd
}

func newCloudPreviewCmd() *cobra.Command {
	var stack string

	var cmd = &cobra.Command{
		Use:        "preview",
		SuggestFor: []string{"plan"},
		Short:      "Show a preview of updates to an stack's resources",
		Long: "Show a preview of updates an stack's resources\n" +
			"\n" +
			"This command displays a preview of the updates to an existing stack. The new desired state" +
			"is computed by compiling and evaluating an executable package, and extracting all resource" +
			"allocations from its resulting object graph. These allocations are then compared against" +
			"the existing state to determine what operations must take place to achieve the desired state." +
			"No changes to the stack will actually take place.\n" +
			"\n" +
			"The package to execute is loaded from the current directory. Use the `-C` or `--cwd` flag to\n" +
			"use a different directory.",
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

			// Gather up configuration.
			// TODO(pulumi-service/issues/221): Have pulumi.com handle the encryption/decryption.
			textConfig, err := getDecryptedConfig(stackName)
			if err != nil {
				return errors.Wrap(err, "getting decrypted configuration")
			}

			// Preview the program in the Pulumi Cloud. Uses same API shape as update.
			updateRequest := apitype.UpdateProgramRequest{
				ProgramArchive: archive,
				Config:         textConfig,
			}
			var updateResponse apitype.UpdateProgramResponse
			path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s/preview",
				projID.Owner, projID.Repository, projID.Project, string(stackName))
			if err = pulumiRESTCall("POST", path, &updateRequest, &updateResponse); err != nil {
				return err
			}
			fmt.Printf("Previewing update to Stack '%s'...\n", string(stackName))

			// Wait for the update to complete.
			status, err := waitForUpdate(path)
			fmt.Println() // The PPC's final message we print to STDOUT doesn't include a newline.

			if err != nil {
				return fmt.Errorf("waiting for preview: %v", err)
			}
			if status == apitype.StatusSucceeded {
				fmt.Println("Preview resulted in success.")
				return nil
			}
			return fmt.Errorf("preview result was unsuccessful: status %v", status)
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose an stack other than the currently selected one")

	return cmd
}
