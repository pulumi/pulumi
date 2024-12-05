// Copyright 2024, Pulumi Corporation.
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
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/version"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type stateRepairCmd struct {
	// A string containing the set of flags passed to the command, for use in error messages.
	FlagsString string
	// Parsed arguments to the command.
	Args *stateRepairArgs

	// The command's standard input.
	Stdin terminal.FileReader
	// The command's standard output.
	Stdout terminal.FileWriter
	// The command's standard error.
	Stderr io.Writer

	// The workspace to operate on.
	Workspace pkgWorkspace.Context
	// The login manager to use for authenticating with and loading backends.
	LoginManager backend.LoginManager
}

// A set of arguments for the `state repair` command.
type stateRepairArgs struct {
	Stack     string
	Colorizer colors.Colorization
	Yes       bool
}

func newStateRepairCommand() *cobra.Command {
	stateRepair := &stateRepairCmd{
		Args: &stateRepairArgs{
			Colorizer: cmdutil.GetGlobalColorization(),
		},
		Stdin:        os.Stdin,
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
		Workspace:    pkgWorkspace.Instance,
		LoginManager: DefaultLoginManager,
	}

	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Repair an invalid state",
		Long: `Repair an invalid state,

This command can be used to repair an invalid state file. It will attempt to
sort resources that appear out of order and remove references to resources that
are no longer present in the state. If the state is already valid, this command
will not attempt to make or write any changes. If the state is not already
valid, and remains invalid after repair has been attempted, this command will
not write any changes.
`,
		Args: cmdutil.NoArgs,
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			cmd.Flags().Visit(func(f *pflag.Flag) {
				stateRepair.FlagsString += fmt.Sprintf(" --%s=%q", f.Name, f.Value)
			})

			ctx := cmd.Context()
			err := stateRepair.run(ctx)

			return err
		}),
	}

	cmd.Flags().StringVarP(&stateRepair.Args.Stack,
		"stack", "s", "", "The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVarP(&stateRepair.Args.Yes,
		"yes", "y", false, "Automatically approve and perform the repair")

	return cmd
}

func (cmd *stateRepairCmd) run(ctx context.Context) error {
	// We need to disable integrity checking since this command exists solely to repair invalid states and enabling
	// integrity checking would prevent us from even loading them in the first place. Currently integrity checking is
	// controlled by a hacky global variable, so this is the best we can do, but hopefully it and consequently this code
	// can be refactored.
	backend.DisableIntegrityChecking = true
	defer func() { backend.DisableIntegrityChecking = false }()

	displayOpts := display.Options{
		Color: cmd.Args.Colorizer,
	}
	s, err := requireStack(
		ctx,
		cmd.Workspace,
		cmd.LoginManager,
		cmd.Args.Stack,
		stackOfferNew,
		displayOpts,
	)
	if err != nil {
		return err
	}

	snap, err := s.Snapshot(ctx, stack.DefaultSecretsProvider)
	if err != nil {
		return err
	} else if snap == nil {
		return nil
	}

	sink := diag.DefaultSink(cmd.Stdout, cmd.Stderr, diag.FormatOptions{
		Color: cmd.Args.Colorizer,
	})

	// If the snapshot is already valid, we won't touch it.
	initialErr := snap.VerifyIntegrity()
	if initialErr == nil {
		sink.Infof(diag.RawMessage("" /*urn*/, "The snapshot is already valid; skipping repair"))
		return nil
	}

	beforeSort := snap.Resources

	// Sorting the snapshot could fail due to cycles or e.g. unparseable provider references. In those cases, manual
	// repair is likely the only option, so we'll print a help banner to guide the user through that and invite them
	// to file a report so that we can learn about how they ended up with such a state.
	err = snap.Toposort()
	if err != nil {
		sink.Errorf(diag.RawMessage("" /*urn*/, cmd.manualRepairError(initialErr, err)))

		// We've already taken care of printing an error message, so we'll wrap the error we return in a Bail so that
		// callers higher up the stack don't print anything of their own.
		return result.BailError(err)
	}

	afterSort := snap.Resources
	reorderings := computeStateRepairReorderings(beforeSort, afterSort)

	pruneResults := snap.Prune()

	// In the case that we complete repairs (sorting, pruning and so on) but the snapshot is still invalid, we'll
	// produce a banner that helps the user conduct a manual repair but also includes both errors, so that if they
	// file a report we can hopefully diagnose both what caused the invalid snapshot but also why we failed to repair
	// it (assuming a repair was possible).
	err = snap.VerifyIntegrity()
	if err != nil {
		sink.Errorf(diag.RawMessage("" /*urn*/, cmd.manualRepairError(initialErr, err)))

		// We've already taken care of printing an error message, so we'll wrap the error we return in a Bail so that
		// callers higher up the stack don't print anything of their own.
		return result.BailError(initialErr)
	}

	// We've managed to repair the invalid snapshot. If the user has passed the --yes flag, we'll just write the changes.
	// If not, we'll render a summary of the operations we've performed and ask for confirmation before writing.
	if !cmd.Args.Yes {
		sink.Infof(diag.RawMessage(
			"", /*urn*/
			renderStateRepairOperations(cmd.Args.Colorizer, reorderings, pruneResults),
		))

		yes := "yes"
		no := "no"
		msg := "This command will edit your stack's state directly. Confirm?"
		options := []string{yes, no}

		switch response := promptUser(
			msg, options, no, cmd.Args.Colorizer,
			survey.WithStdio(cmd.Stdin, cmd.Stdout, cmd.Stderr),
		); response {
		case yes:
			// Continue.
		case no:
			sink.Infof(diag.RawMessage("" /*urn*/, "Confirmation denied, not proceeding with state repair"))
			return nil
		}
	}

	// We've managed to repair the snapshot -- import it back into the backend.
	sdep, err := stack.SerializeDeployment(ctx, snap, false /*showSecrets*/)
	if err != nil {
		return fmt.Errorf("serializing deployment: %w", err)
	}

	bytes, err := json.Marshal(sdep)
	if err != nil {
		return err
	}

	err = s.ImportDeployment(ctx, &apitype.UntypedDeployment{
		Version:    apitype.DeploymentSchemaVersionCurrent,
		Deployment: bytes,
	})
	if err != nil {
		return err
	}

	sink.Infof(diag.RawMessage("" /*urn*/, "State repaired successfully"))
	return nil
}

// Returns a help banner detailing the given error and providing instructions for manual state repair.
func (cmd *stateRepairCmd) manualRepairError(initialErr error, err error) string {
	stateFile := "state.json"
	stateFileBackup := "state.json.backup"

	stateExportCommand := fmt.Sprintf("pulumi stack%s export > %s", cmd.FlagsString, stateFile)

	var copyBinary string
	if runtime.GOOS == "windows" {
		copyBinary = "copy"
	} else {
		copyBinary = "cp"
	}
	copyCommand := fmt.Sprintf("%s %s %s", copyBinary, stateFile, stateFileBackup)

	stateImportCommand := fmt.Sprintf("pulumi stack%s import < %s", cmd.FlagsString, stateFile)

	return fmt.Sprintf(`Failed to repair the snapshot automatically: %[1]v

Pulumi is unable to automatically repair the snapshot. This can happen if the
snapshot contains cycles or corrupted or unparseable data. You may be able to
manually repair your stack as follows:

1. Manually export your stack's state to a file named %[2]s:

   %[3]s

2. Make a copy of the exported %[2]s file so that you have a backup:

   %[4]s

3. Edit the exported %[2]s file to fix any issues, such as cyclic
   dependencies or corrupted data.

4. Import the repaired %[2]s file back into your stack:

   %[5]s


================================================================================
We would appreciate a report: https://github.com/pulumi/pulumi/issues/

Please provide all of the text below in your report.
================================================================================
Pulumi Version:    %[6]s
Go Version:        %[7]s
Go Compiler:       %[8]s
Architecture:      %[9]s
Operating System:  %[10]s
Command:           %[11]s
Initial Error:     %[12]s
`,
		err,
		stateFile,
		stateExportCommand,
		copyCommand,
		stateImportCommand,
		version.Version,
		runtime.Version(),
		runtime.Compiler,
		runtime.GOARCH,
		runtime.GOOS,
		strings.Join(os.Args, " "),
		initialErr,
	)
}
