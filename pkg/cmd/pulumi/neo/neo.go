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
	"path/filepath"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/tools"
	displaytypes "github.com/pulumi/pulumi/pkg/v3/display"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// outboundEvent is the local envelope the TUI uses to dispatch user events to
// runNeo's dispatcher loop. It wraps the wire-level AgentUserEvent and tacks on
// planMode, which is only meaningful for the first user_message (the one that
// triggers CreateNeoTask). Wrapping keeps planMode out of apitype and avoids a
// second channel for a value that's always produced alongside a message.
type outboundEvent struct {
	event    apitype.AgentUserEvent
	planMode bool
}

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
		Use:   "neo [prompt]",
		Short: "Start a Pulumi Neo agent task with local tool execution",
		Long: "Creates a Pulumi Neo agent task in CLI tool execution mode and runs the local " +
			"tool loop. Filesystem and shell tool calls from the agent run on this machine, " +
			"in the working directory you select, instead of in the cloud agent container. " +
			"If no prompt is provided, the TUI starts and waits for your first message.",
		Hidden: !env.Experimental.Value(),
		Args:   cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var prompt string
			if len(args) > 0 {
				prompt = args[0]
			}
			return runNeo(ctx, prompt, stackName, orgFlag, cwdFlag)
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

	// Allow tools to read/write under temp directories in addition to cwd: the agent
	// stages scratch files there (downloads, intermediate state) and the CLI sandbox
	// would otherwise reject those paths. See pulumi/pulumi-service#42027.
	extraRoots := dedupeExistingRoots("/tmp", os.TempDir())
	fs, err := tools.NewFilesystem(cwdFlag, extraRoots...)
	if err != nil {
		return err
	}
	sh, err := tools.NewShell(cwdFlag, extraRoots...)
	if err != nil {
		return err
	}
	handlers := map[string]ToolHandler{
		"filesystem": fs,
		"shell":      sh,
	}

	// In non-interactive mode the sink stays nil and live events are dropped; the
	// interactive path below sets pu.Sink to push UIEvents onto uiCh.
	pu, err := tools.NewPulumi(cwdFlag, ws, cloudBe, nil)
	if err != nil {
		return err
	}
	handlers["pulumi"] = pu

	// Non-interactive mode requires a prompt — there's no input mechanism.
	if !cmdutil.Interactive() {
		if prompt == "" {
			return errors.New("a prompt argument is required in non-interactive mode")
		}
		resp, err := pc.CreateNeoTask(
			ctx, orgName, prompt, stackRefName, projectName, client.CreateNeoTaskOptions{
				ToolExecutionMode: "cli",
				ApprovalMode:      client.NeoApprovalModeManual,
			})
		if err != nil {
			return err
		}
		consoleURL := client.CloudConsoleURL(pc.URL(), orgName, "neo", "tasks", resp.TaskID)
		if consoleURL != "" {
			fmt.Println(consoleURL)
		} else {
			fmt.Printf("Neo task created (id %s)\n", resp.TaskID)
		}
		session := &Session{
			Client:   pc,
			Handlers: handlers,
			OrgName:  orgName,
			TaskID:   resp.TaskID,
			Log:      os.Stderr,
		}
		return session.Run(ctx)
	}

	uiCh := make(chan UIEvent, 64)
	outCh := make(chan outboundEvent, 8)

	pu.Sink = newPulumiSinkForUI(uiCh)

	username, _, _, _ := pc.GetPulumiAccountDetails(ctx)

	model := NewModel(ModelConfig{
		Org:           orgName,
		WorkDir:       cwdFlag,
		Username:      username,
		EventCh:       uiCh,
		OutCh:         outCh,
		Busy:          prompt != "",
		InitialPrompt: prompt,
	})

	// Inline (non-alt-screen) so the transcript stays in the user's terminal
	// scrollback after exit.
	p := tea.NewProgram(model)

	g, gctx := errgroup.WithContext(ctx)

	// taskState tracks the task ID once created (may be deferred if no prompt).
	type taskState struct {
		mu     sync.Mutex
		taskID string
	}
	ts := &taskState{}

	// createTask creates the Neo task with the given prompt and starts the session.
	// Called immediately if a prompt was provided, or on the first user message.
	// planMode is the value the TUI captured at the moment the first message was
	// sent; the CLI-prompt path always passes false.
	createTask := func(initialPrompt string, planMode bool) error {
		resp, err := pc.CreateNeoTask(
			gctx, orgName, initialPrompt, stackRefName, projectName, client.CreateNeoTaskOptions{
				ToolExecutionMode: "cli",
				ApprovalMode:      client.NeoApprovalModeManual,
				PlanMode:          planMode,
			})
		if err != nil {
			return err
		}

		ts.mu.Lock()
		ts.taskID = resp.TaskID
		ts.mu.Unlock()

		consoleURL := client.CloudConsoleURL(pc.URL(), orgName, "neo", "tasks", resp.TaskID)
		if consoleURL != "" {
			sendUI(uiCh, UISessionURL{URL: consoleURL})
		}

		session := &Session{
			Client:   pc,
			Handlers: handlers,
			OrgName:  orgName,
			TaskID:   resp.TaskID,
			UIEvents: uiCh,
		}
		return session.Run(gctx)
	}

	if prompt != "" {
		// The command-line prompt path always passes false for planMode.
		g.Go(func() error {
			return createTask(prompt, false)
		})
	}

	g.Go(func() error {
		_, err := p.Run()
		return err
	})

	// Post TUI-originated user events to the API. Chat messages may arrive before
	// any task exists — the first one creates it. Other event types (approvals
	// and anything we add later) only make sense once a task is live.
	g.Go(func() error {
		taskCreated := prompt != ""
		for {
			select {
			case <-gctx.Done():
				return nil
			case ob, ok := <-outCh:
				if !ok {
					return nil
				}
				if msg, isMsg := ob.event.(apitype.AgentUserEventUserMessage); isMsg && !taskCreated {
					taskCreated = true
					planMode := ob.planMode
					g.Go(func() error {
						return createTask(msg.Content, planMode)
					})
					continue
				}
				ts.mu.Lock()
				taskID := ts.taskID
				ts.mu.Unlock()
				if taskID == "" {
					// Unreachable in normal use: the TUI gates Enter on busy state
					// until UITaskIdle, and approvals only fire in response to backend
					// events that imply the task exists. Surface instead of silently
					// dropping.
					sendUI(uiCh, UIWarning{Message: "dropped event: task not ready"})
					continue
				}
				if err := pc.PostNeoTaskUserEvent(gctx, orgName, taskID, ob.event); err != nil {
					sendUI(uiCh, UIWarning{Message: "failed to send event: " + err.Error()})
				}
			}
		}
	})

	return g.Wait()
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

// dedupeExistingRoots returns candidates with duplicates removed by canonical path,
// dropping any that don't resolve on the local filesystem. This handles macOS where
// /tmp and os.TempDir() are distinct canonical roots, Linux where they collapse to
// the same one, and Windows where /tmp typically doesn't exist.
func dedupeExistingRoots(candidates ...string) []string {
	seen := make(map[string]bool, len(candidates))
	var out []string
	for _, c := range candidates {
		if c == "" {
			continue
		}
		canon, err := filepath.EvalSymlinks(c)
		if err != nil {
			continue
		}
		if seen[canon] {
			continue
		}
		seen[canon] = true
		out = append(out, c)
	}
	return out
}

// newPulumiSinkForUI builds a tools.PulumiSink whose callbacks translate each
// progress signal into the matching UIEvent on uiCh. Pure mechanical
// translation
func newPulumiSinkForUI(uiCh chan<- UIEvent) *tools.PulumiSink {
	return &tools.PulumiSink{
		OnStart: func(toolName, stackName string, isPreview bool) {
			sendUI(uiCh, UIPulumiStart{ToolName: toolName, StackName: stackName, IsPreview: isPreview})
		},
		OnResource: func(toolName string, op displaytypes.StepOp, urn, typ, status string) {
			sendUI(uiCh, UIPulumiResource{ToolName: toolName, Op: op, URN: urn, Type: typ, Status: status})
		},
		OnDiag: func(toolName, severity, message, urn string) {
			sendUI(uiCh, UIPulumiDiag{ToolName: toolName, Severity: severity, Message: message, URN: urn})
		},
		OnEnd: func(toolName, errStr string, counts displaytypes.ResourceChanges, elapsed string) {
			sendUI(uiCh, UIPulumiEnd{ToolName: toolName, Err: errStr, Counts: counts, Elapsed: elapsed})
		},
	}
}
