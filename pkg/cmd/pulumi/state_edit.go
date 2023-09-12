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
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"
	"github.com/google/shlex"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/spf13/cobra"
)

func newStateEditCommand() *cobra.Command {
	var stackName string
	stateEdit := &stateEditCmd{
		Ctx:       commandContext(),
		Colorizer: cmdutil.GetGlobalColorization(),
	}
	cmd := &cobra.Command{
		Use: "edit",
		// TODO(dixler) Add test for unicode round-tripping before unhiding.
		Hidden: !hasExperimentalCommands(),
		Short:  "Edit the current stack's state in your EDITOR",
		Long: `[EXPERIMENTAL] Edit the current stack's state in your EDITOR

This command can be used to surgically edit a stack's state in the editor
specified by the EDITOR environment variable and will provide the user with
a preview showing a diff of the altered state.`,
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			if !cmdutil.Interactive() {
				return result.Error("pulumi state edit must be run in interactive mode")
			}
			s, err := requireStack(commandContext(), stackName, stackLoadOnly, display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			})
			if err != nil {
				return result.FromError(err)
			}
			if err := stateEdit.Run(s); err != nil {
				return result.FromError(err)
			}
			return nil
		}),
	}
	cmd.PersistentFlags().StringVar(
		&stackName, "stack", "",
		"Remove the stack and its config file after all resources in the stack have been deleted")
	return cmd
}

type stateEditCmd struct {
	Stdin     io.Reader
	Stdout    io.Writer
	Colorizer colors.Colorization
	Ctx       context.Context
}

type snapshotBuffer struct {
	Name     func() string
	Snapshot func() (*deploy.Snapshot, error)
	Reset    func() error
	Cleanup  func()
}

func newSnapshotBuffer(fileExt string, sf snapshotEncoder, snap *deploy.Snapshot) (*snapshotBuffer, error) {
	tempFile, err := os.CreateTemp("", "pulumi-state-edit-*"+fileExt)
	if err != nil {
		return nil, err
	}
	tempFile.Close()

	originalText, err := sf.SnapshotToText(snap)
	if err != nil {
		// Warn that the snapshot is already hosed.
		cmdutil.Diag().Errorf(diag.RawMessage("", fmt.Sprintf("initial state unable to be serialized: %v", err)))
	}
	t := &snapshotBuffer{
		Name: func() string { return tempFile.Name() },
		Snapshot: func() (*deploy.Snapshot, error) {
			b, err := os.ReadFile(tempFile.Name())
			if err != nil {
				return nil, err
			}
			return sf.TextToSnapshot(snapshotText(b))
		},
		Reset: func() error {
			return os.WriteFile(tempFile.Name(), originalText, 0o600)
		},
		Cleanup: func() {
			os.Remove(tempFile.Name())
		},
	}
	if err := t.Reset(); err != nil {
		t.Cleanup()
		return nil, err
	}
	return t, nil
}

func (cmd *stateEditCmd) Run(s backend.Stack) error {
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}

	snap, err := s.Snapshot(cmd.Ctx, stack.DefaultSecretsProvider)
	if err != nil {
		return err
	}
	if snap == nil {
		return fmt.Errorf("old snapshot expected to be non-nil")
	}

	sf := &jsonSnapshotEncoder{
		ctx: cmd.Ctx,
	}
	f, err := newSnapshotBuffer(".json", sf, snap)
	if err != nil {
		return err
	}
	defer f.Cleanup()

	for {
		err = openInEditor(f.Name())
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.Stdout, cmd.Colorizer.Colorize(
			colors.SpecHeadline+"Previewing state edit (%s)"+colors.Reset+"\n\n"), s.Ref().FullyQualifiedName())

		accept := "accept"
		edit := "edit"
		reset := "reset"
		cancel := "cancel"

		var msg string
		var options []string
		news, err := cmd.validateAndPrintState(f)
		if err != nil {
			cmdutil.Diag().Errorf(diag.Message("", "provided state is not valid: %v"), err)
			msg = "Received invalid state. What would you like to do?"
			options = []string{
				// No accept option as the state is invalid.
				edit,
				reset,
				cancel,
			}
		} else {
			msg = "Do you want to perform this edit?"
			options = []string{
				accept,
				edit,
				reset,
				cancel,
			}
		}

		switch response := cmd.promptStateEdit(msg, options, edit); response {
		case accept:
			return saveSnapshot(cmd.Ctx, s, news, false /* force */)
		case edit:
			continue
		case reset:
			if err := f.Reset(); err != nil {
				return err
			}
			continue
		default:
			return fmt.Errorf("confirmation cancelled, not proceeding with the state edit")
		}
	}
}

func (cmd *stateEditCmd) promptStateEdit(msg string, options []string, defaultOption string) string {
	prompt := "\b" + cmd.Colorizer.Colorize(colors.SpecPrompt+msg+colors.Reset)
	surveycore.DisableColor = true
	surveyIcons := survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question = survey.Icon{}
		icons.SelectFocus = survey.Icon{Text: cmd.Colorizer.Colorize(colors.BrightGreen + ">" + colors.Reset)}
	})

	var response string
	if err := survey.AskOne(&survey.Select{
		Message: prompt,
		Options: options,
		Default: defaultOption,
	}, &response, surveyIcons); err != nil {
		return ""
	}
	return response
}

func (cmd *stateEditCmd) validateAndPrintState(f *snapshotBuffer) (*deploy.Snapshot, error) {
	news, err := f.Snapshot()
	if err != nil {
		return nil, err
	}

	err = news.VerifyIntegrity()
	if err != nil {
		return nil, err
	}

	// Display state in JSON to match JSON-like diffs in the update display.
	json := &jsonSnapshotEncoder{
		ctx: cmd.Ctx,
	}
	previewText, err := json.SnapshotToText(news)
	if err != nil {
		// This should not fail as we have already verified the integrity of the snapshot.
		return nil, err
	}

	fmt.Fprint(cmd.Stdout, cmd.Colorizer.Colorize(
		colors.SpecHeadline+"New state:"+colors.Reset+"\n"))

	fmt.Fprintln(cmd.Stdout, string(previewText))
	return news, nil
}

func openInEditor(filename string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return fmt.Errorf("no EDITOR environment variable set")
	}
	return openInEditorInternal(editor, filename)
}

func openInEditorInternal(editor, filename string) error {
	contract.Requiref(editor != "", "editor", "must not be empty")

	args, err := shlex.Split(editor)
	if err != nil {
		return err
	}
	args = append(args, filename)

	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(cmd.Stdout, "Failed to exec EDITOR: %v\n", err)
		return err
	}
	return nil
}
