// Copyright 2026, Pulumi Corporation.
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

package neo

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/tools"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// NewNeoCmd creates the `pulumi neo` command. This first slice of the command starts a
// Neo task in `cli` tool execution mode, prints a console URL the user can open in a
// browser, and runs the local tool-execution loop in the foreground until the task ends.
// There is no interactive UI yet — the chat happens in the web console.
func NewNeoCmd() *cobra.Command {
	var (
		stackName string
		orgFlag   string
		cwdFlag   string
	)

	cmd := &cobra.Command{
		Use:   "neo \"<initial prompt>\"",
		Short: "Start a Pulumi Neo agent task with local tool execution",
		Long: "Creates a Pulumi Neo agent task in CLI tool execution mode and runs the local " +
			"tool loop. Filesystem and shell tool calls from the agent run on this machine, " +
			"in the working directory you select, instead of in the cloud agent container. " +
			"The interactive chat happens in the Pulumi Console — this command prints the URL " +
			"to open after starting the task.",
		Hidden: !env.Experimental.Value(),
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return runNeo(ctx, args[0], stackName, orgFlag, cwdFlag)
		},
	}

	cmd.Flags().StringVarP(&stackName, "stack", "s", "",
		"The name of the stack to attach to the Neo task")
	cmd.Flags().StringVar(&orgFlag, "org", "",
		"The organization that owns the Neo task (defaults to the user's default org)")
	cmd.Flags().StringVar(&cwdFlag, "cwd", "",
		"Working directory for local tool execution (defaults to the current directory)")

	return cmd
}

func runNeo(ctx context.Context, prompt, stackName, orgFlag, cwdFlag string) error {
	if cwdFlag == "" {
		var err error
		cwdFlag, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("resolving working directory: %w", err)
		}
	}

	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, opts)
	if err != nil {
		return err
	}
	cloudBe, ok := be.(httpstate.Backend)
	if !ok {
		return errors.New("`pulumi neo` requires the Pulumi Cloud backend")
	}
	pc := cloudBe.Client()

	orgName, projectName, stackRefName, err := resolveTaskTarget(ctx, ws, cloudBe, project, stackName, orgFlag)
	if err != nil {
		return err
	}

	resp, err := pc.CreateNeoTask(ctx, orgName, prompt, stackRefName, projectName, "cli")
	if err != nil {
		return err
	}

	consoleURL := client.CloudConsoleURL(pc.URL(), orgName, "neo", "tasks", resp.TaskID)
	if consoleURL != "" {
		fmt.Fprintf(os.Stderr, "Neo task: %s\n", consoleURL)
	} else {
		fmt.Fprintf(os.Stderr, "Neo task created (id %s)\n", resp.TaskID)
	}
	fmt.Fprintf(os.Stderr, "Running local tool loop in %s. Press Ctrl-C to exit.\n", cwdFlag)

	fs, err := tools.NewFilesystem(cwdFlag)
	if err != nil {
		return err
	}
	sh, err := tools.NewShell(cwdFlag)
	if err != nil {
		return err
	}
	session := &Session{
		Client: pc,
		Handlers: map[string]ToolHandler{
			"filesystem": fs,
			"shell":      sh,
		},
		OrgName: orgName,
		TaskID:  resp.TaskID,
		Log:     os.Stderr,
	}
	return session.Run(ctx)
}

// resolveTaskTarget figures out the org, project, and stack name to attach to the new Neo
// task. The stack flag is optional — if it's empty we try the currently selected stack and
// fall back to a project-only attachment if there isn't one.
func resolveTaskTarget(
	ctx context.Context,
	ws pkgWorkspace.Context,
	be httpstate.Backend,
	project *workspace.Project,
	stackName, orgFlag string,
) (org, projectName, stack string, err error) {
	if project != nil {
		projectName = string(project.Name)
	}

	if stackName != "" {
		ref, err := be.ParseStackReference(stackName)
		if err != nil {
			return "", "", "", err
		}
		stack = ref.Name().String()
	} else {
		s, err := state.CurrentStack(ctx, ws, be)
		if err == nil && s != nil {
			stack = s.Ref().Name().String()
		}
	}

	if orgFlag != "" {
		org = orgFlag
	} else {
		org, err = pkgBackend.GetDefaultOrg(ctx, be, project)
		if err != nil {
			return "", "", "", fmt.Errorf("determining default organization: %w", err)
		}
	}
	if org == "" {
		return "", "", "", errors.New("could not determine an organization for the Neo task; pass --org")
	}
	return org, projectName, stack, nil
}
