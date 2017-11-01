// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/archive"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newUpdateCmd() *cobra.Command {
	// Swap out which update command to use based on whether or not the Console API is available.
	if usePulumiCloudCommands() {
		return newCloudUpdateCmd()
	}
	return newFAFUpdateCmd()
}

// newFAFUpdateCmd returns the fire-and-forget version of the update command.
func newFAFUpdateCmd() *cobra.Command {
	var analyzers []string
	var debug bool
	var dryRun bool
	var stack string
	var parallel int
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var summary bool
	var cmd = &cobra.Command{
		Use:        "update",
		Aliases:    []string{"up"},
		SuggestFor: []string{"deploy", "push"},
		Short:      "Update the resources in an stack",
		Long: "Update the resources in an stack\n" +
			"\n" +
			"This command updates an existing stack whose state is represented by the\n" +
			"existing snapshot file. The new desired state is computed by compiling and evaluating an\n" +
			"executable package, and extracting all resource allocations from its resulting object graph.\n" +
			"These allocations are then compared against the existing state to determine what operations\n" +
			"must take place to achieve the desired state. This command results in a full snapshot of the\n" +
			"stack's new resource state, so that it may be updated incrementally again later.\n" +
			"\n" +
			"The package to execute is loaded from the current directory. Use the `-C` or `--cwd` flag to\n" +
			"use a different directory.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			var backend pulumiBackend = &localPulumiBackend{}

			stackName, err := explicitOrCurrent(stack)
			if err != nil {
				return err
			}

			return backend.Update(stackName, debug, engine.DeployOptions{
				DryRun:               dryRun,
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
		"Run one or more analyzers as part of this update")
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
		&showReplacementSteps, "show-replacement-steps", true,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&summary, "summary", false,
		"Only display summarization of resources and operations")

	return cmd
}

func newCloudUpdateCmd() *cobra.Command {
	var stack string

	var cmd = &cobra.Command{
		Use:        "update",
		Aliases:    []string{"up"},
		SuggestFor: []string{"deploy", "push"},
		Short:      "Update the resources in an stack",
		Long: "Update a Pulumi Program\n" +
			"\n" +
			"This command creates or updates a Stack hosted in a Pulumi Cloud.",
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
			packagePath, err := workspace.DetectPackage(cwd)
			if err != nil {
				return fmt.Errorf("looking for Pulumi package: %v", err)
			}
			if packagePath == "" {
				return fmt.Errorf("no Pulumi package found")
			}
			// packagePath is the path to the pulumi.yaml file. Need its parent folder.
			packageFolder := filepath.Dir(packagePath)
			archive, err := archive.EncodePath(packageFolder)
			if err != nil {
				return fmt.Errorf("creating archive: %v", err)
			}

			// Load the package, since we now require passing the Runtime with the update request.
			pkg, err := pack.Load(packagePath)
			if err != nil {
				return err
			}

			// Gather up configuration.
			// TODO(pulumi-service/issues/221): Have pulumi.com handle the encryption/decryption.
			textConfig, err := getDecryptedConfig(stackName)
			if err != nil {
				return errors.Wrap(err, "getting decrypted configuration")
			}

			// Update the program in the Pulumi Cloud
			updateRequest := apitype.UpdateProgramRequest{
				Name:           pkg.Name,
				Runtime:        pkg.Runtime,
				ProgramArchive: archive,
				Config:         textConfig,
			}
			var updateResponse apitype.UpdateProgramResponse
			path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s/update",
				projID.Owner, projID.Repository, projID.Project, string(stackName))
			if err = pulumiRESTCall("POST", path, &updateRequest, &updateResponse); err != nil {
				return err
			}
			fmt.Printf("Updating Stack '%s' to version %d...\n", string(stackName), updateResponse.Version)

			// Wait for the update to complete.
			status, err := waitForUpdate(path)
			fmt.Println() // The PPC's final message we print to STDOUT doesn't include a newline.

			if err != nil {
				return fmt.Errorf("waiting for update: %v", err)
			}
			if status == apitype.StatusSucceeded {
				fmt.Println("Update completed successfully.")
				return nil
			}
			return fmt.Errorf("update unsuccessful: status %v", status)
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose an stack other than the currently selected one")

	return cmd
}

// waitForUpdate waits for the current update of a Pulumi program to reach a terminal state. Returns the
// final state. "path" is the URL endpoint to poll for updates.
func waitForUpdate(path string) (apitype.UpdateStatus, error) {
	time.Sleep(5 * time.Second)

	// Events occur in sequence, filter out all the ones we have seen before in each request.
	eventIndex := 0
	for {
		time.Sleep(2 * time.Second)

		var updateResults apitype.UpdateResults
		pathWithIndex := fmt.Sprintf("%s?afterIndex=%d", path, eventIndex)
		if err := pulumiRESTCall("GET", pathWithIndex, nil, &updateResults); err != nil {
			return "", err
		}

		for _, event := range updateResults.Events {
			printEvent(event)
			eventIndex = event.Index
		}

		// Check if in termal state.
		updateStatus := apitype.UpdateStatus(updateResults.Status)
		switch updateStatus {
		case apitype.StatusFailed:
			fallthrough
		case apitype.StatusSucceeded:
			return updateStatus, nil
		}
	}
}

func printEvent(event apitype.UpdateEvent) {
	stream := os.Stdout // Ignoring event.Kind which could be StderrEvent.
	rawEntry, ok := event.Fields["text"]
	if !ok {
		return
	}
	text := rawEntry.(string)
	if colorize, ok := event.Fields["colorize"].(bool); ok && colorize {
		text = colors.ColorizeText(text)
	}
	fmt.Fprint(stream, text)
}

// getDecryptedConfig returns the stack's configuration with any secrets in plain-text.
func getDecryptedConfig(stackName tokens.QName) (map[tokens.ModuleMember]string, error) {
	cfg, err := getConfiguration(stackName)
	if err != nil {
		return nil, errors.Wrap(err, "getting configuration")
	}

	var decrypter config.ValueDecrypter = panicCrypter{}
	if hasSecureValue(cfg) {
		decrypter, err = getSymmetricCrypter()
		if err != nil {
			return nil, errors.Wrap(err, "getting symmetric crypter")
		}
	}

	textConfig := make(map[tokens.ModuleMember]string)
	for key := range cfg {
		decrypted, err := cfg[key].Value(decrypter)
		if err != nil {
			return nil, errors.Wrap(err, "could not decrypt configuration value")
		}
		textConfig[key] = decrypted
	}
	return textConfig, nil
}
