// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newUpdateCmd() *cobra.Command {
	// Swap out which update command to use based on whether or not the Console API is available.
	_, err := pulumiConsoleAPI()
	if err != nil {
		return newFAFUpdateCmd()
	}
	return newCloudUpdateCmd()
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
			stackName, err := explicitOrCurrent(stack)
			if err != nil {
				return err
			}

			events := make(chan engine.Event)
			done := make(chan bool)

			go displayEvents(events, done, debug)

			if err = lumiEngine.Deploy(stackName, events, engine.DeployOptions{
				DryRun:               dryRun,
				Analyzers:            analyzers,
				Parallel:             parallel,
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSames:            showSames,
				Summary:              summary,
			}); err != nil {
				return err
			}

			<-done
			close(events)
			close(done)
			return nil
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
	var org string
	var cloud string
	var repo string
	var project string
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
			archive, err := archiveAndEncodePath(programFolder)
			if err != nil {
				return fmt.Errorf("creating archive: %v", err)
			}

			// Gather up configuration.
			stackToken := tokens.QName(stack)
			config, err := getConfiguration(stackToken)
			if err != nil {
				return fmt.Errorf("getting configuration: %v", err)
			}

			// Update the program in the Pulumi Cloud
			updateRequest := apitype.UpdateProgramRequest{
				ProgramArchive: archive,
				Config:         config,
			}
			var updateResponse apitype.UpdateProgramResponse
			path := fmt.Sprintf("/orgs/%s/clouds/%s/programs/%s/%s/%s/update", org, cloud, repo, project, stack)
			if err = pulumiRESTCall("POST", path, &updateRequest, &updateResponse); err != nil {
				return err
			}
			fmt.Printf("Updating Stack '%s' to version %d...\n", stack, updateResponse.Version)

			// Wait for the update to complete.
			result, err := waitForUpdate(path)
			if err != nil {
				return fmt.Errorf("waiting for update: %v", err)
			}
			fmt.Printf("Final result: %s\n", result)

			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&org, "organization", "o", "",
		"Target organization")
	cmd.PersistentFlags().StringVarP(
		&cloud, "cloud", "c", "",
		"Target cloud")
	cmd.PersistentFlags().StringVarP(
		&repo, "repo", "r", "",
		"Target Pulumi repo")
	cmd.PersistentFlags().StringVarP(
		&project, "project", "p", "",
		"Target Pulumi project")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Name of the stack to deploy")

	// We need all of these flags to be set. In the future we'll get some of these from the .pulumi folder, and have
	// a meaningful default for others. So in practice users won't need to specify all of these. (Ideally none.)
	contract.AssertNoError(cmd.MarkPersistentFlagRequired("organization"))
	contract.AssertNoError(cmd.MarkPersistentFlagRequired("cloud"))
	contract.AssertNoError(cmd.MarkPersistentFlagRequired("repo"))
	contract.AssertNoError(cmd.MarkPersistentFlagRequired("project"))
	contract.AssertNoError(cmd.MarkPersistentFlagRequired("stack"))

	return cmd
}

// waitForUpdate waits for the current update of a Pulumi program to reach a terminal state. Returns the state
// description. (e.g. "failed" or "succeeded".) path is the URL endpoint to poll for updates, events and done
// are channels to emit output events to.
func waitForUpdate(path string) (string, error) {
	time.Sleep(3 * time.Second)

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
		switch updateResults.Status {
		case apitype.StatusFailed:
			fallthrough
		case apitype.StatusSucceeded:
			return updateResults.Status, nil
		}
	}
}

func printEvent(event apitype.UpdateEvent) {
	stream := os.Stdout // Ignoring event.Kind which could be StderrEvent.
	text := event.Fields["text"].(string)
	if colorize, ok := event.Fields["colorize"].(bool); ok && colorize {
		text = colors.ColorizeText(text)
	}
	fmt.Fprint(stream, text)
}
