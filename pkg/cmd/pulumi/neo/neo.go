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
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/tools"
	displaytypes "github.com/pulumi/pulumi/pkg/v3/display"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// nonInteractivePromptPreamble nudges the agent away from follow-up questions
// in modes where there's no way to send another user message: stdin isn't
// wired and the caller (typically another agent or a script) is blocked
// reading stdout, so anything that needs more input would hang. Wrapped in
// <details> so it collapses in the rendered task view and doesn't clutter
// the user's prompt.
const nonInteractivePromptPreamble = "<details><summary>non-interactive mode</summary>\n\n" +
	"You are running in non-interactive mode: your final response will be written " +
	"to stdout. Do not ask follow-up questions; make any reasonable assumptions " +
	"explicit and return a complete final answer.\n\n" +
	"</details>"

// createNeoTaskWithEntityRetry creates a Neo task; if the backend rejects the
// attached stack with "invalid entities" (typically a permissions issue) it retries
// once without the stack so the task is still created. onEntityDropped, if non-nil,
// is invoked with the original error when the fallback path runs, so callers can
// surface a warning.
func createNeoTaskWithEntityRetry(
	ctx context.Context,
	pc *client.Client,
	orgName, prompt, stackName, projectName string,
	opts client.CreateNeoTaskOptions,
	onEntityDropped func(error),
) (*client.NeoTaskResponse, error) {
	resp, err := pc.CreateNeoTask(ctx, orgName, prompt, stackName, projectName, opts)
	if err != nil && stackName != "" && projectName != "" && isInvalidEntitiesError(err) {
		if onEntityDropped != nil {
			onEntityDropped(err)
		}
		return pc.CreateNeoTask(ctx, orgName, prompt, "", "", opts)
	}
	return resp, err
}

// isInvalidEntitiesError reports whether err is the Neo backend's "invalid entities"
// rejection. Matched on the message because the service doesn't expose a stable
// error code for this case.
func isInvalidEntitiesError(err error) bool {
	var errResp *apitype.ErrorResponse
	if !errors.As(err, &errResp) {
		return false
	}
	return strings.Contains(strings.ToLower(errResp.Message), "invalid entit")
}

// outboundEvent is the local envelope the TUI uses to dispatch user events to
// runNeo's dispatcher loop. It carries either a wire-level AgentUserEvent (chat
// messages, approval answers, cancels) or, when update is non-nil, a mid-session
// approval/permission mode change. Wrapping keeps these out of apitype and avoids
// a second channel for values produced alongside the user's keypresses.
//
// planMode, approvalMode, and permissionMode are only meaningful on the first
// user_message — the one that triggers CreateNeoTask. The dispatcher snapshots
// them into the create-task call. Later toggles surface through update instead.
type outboundEvent struct {
	event          apitype.AgentUserEvent
	planMode       bool
	approvalMode   client.NeoApprovalMode
	permissionMode client.NeoPermissionMode
	// update, when non-nil, requests a PATCH against the live task instead of
	// posting a user event. Used by Ctrl+A / Ctrl+R after the first message has
	// been sent, so cloud ApprovalHandler picks up the new mode immediately.
	update *client.UpdateNeoTaskOptions
}

// Indirection points for the integration test in neo_integration_test.go.
// Production behavior is unchanged: newTeaProgram defers to tea.NewProgram and
// isInteractive defers to cmdutil.Interactive. The test swaps both so it can
// run the interactive code path under `go test` (no TTY, no terminal renderer)
// and capture the bubbletea program reference to drive a clean shutdown.
var (
	newTeaProgram = func(m tea.Model) *tea.Program { return tea.NewProgram(m) }
	isInteractive = cmdutil.Interactive
)

// NewNeoCmd creates the `pulumi neo` command. This first slice of the command starts a
// Neo task in `cli` tool execution mode, prints a console URL the user can open in a
// browser, and runs the local tool-execution loop in the foreground until the task ends.
// There is no interactive UI yet — the chat happens in the web console.
func NewNeoCmd() *cobra.Command {
	var (
		stackName           string
		orgFlag             string
		cwdFlag             string
		approvalModeFlag    string
		permissionModeFlag  string
		printMode           bool
		debugUpdateFlag     string
		debugPreviewFlag    string
		disableIntegrations bool
	)

	cmd := &cobra.Command{
		Use:   "neo [prompt]",
		Short: "Start a Pulumi Neo agent task with local tool execution",
		Long: "Creates a Pulumi Neo agent task in CLI tool execution mode and runs the local " +
			"tool loop. Filesystem and shell tool calls from the agent run on this machine, " +
			"in the working directory you select, instead of in the cloud agent container. " +
			"If no prompt is provided, the TUI starts and waits for your first message.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var prompt string
			if len(args) > 0 {
				prompt = args[0]
			}
			approvalMode, err := parseApprovalMode(approvalModeFlag)
			if err != nil {
				return err
			}
			permissionMode, err := parsePermissionMode(permissionModeFlag)
			if err != nil {
				return err
			}
			// --print has no UI; manual approval would deadlock. Reject if explicit,
			// otherwise upgrade the default.
			if printMode {
				switch {
				case cmd.Flags().Changed("approval-mode") && approvalMode == client.NeoApprovalModeManual:
					return errors.New(
						"--approval-mode=manual is incompatible with --print: there is no UI to approve from")
				case !cmd.Flags().Changed("approval-mode"):
					approvalMode = client.NeoApprovalModeAuto
				}
			}
			// --debug-update/--debug-preview seed Neo to investigate a failed operation. A bare
			// flag infers the latest of that kind; =<id> targets a specific run. They are mutually
			// exclusive, so at most one is ever Changed.
			var debugKind debugKind
			var debugID string
			switch {
			case cmd.Flags().Changed("debug-update"):
				debugKind, debugID = debugUpdate, valueOrEmpty(debugUpdateFlag)
			case cmd.Flags().Changed("debug-preview"):
				debugKind, debugID = debugPreview, valueOrEmpty(debugPreviewFlag)
			}
			return runNeo(ctx, cmd.OutOrStdout(), cmd.ErrOrStderr(), neoRunOptions{
				prompt:              prompt,
				stackName:           stackName,
				orgFlag:             orgFlag,
				cwdFlag:             cwdFlag,
				approvalMode:        approvalMode,
				permissionMode:      permissionMode,
				printMode:           printMode,
				debugKind:           debugKind,
				debugID:             debugID,
				disableIntegrations: disableIntegrations,
			})
		},
	}

	cmd.Flags().StringVarP(&stackName, "stack", "s", "",
		"The name of the stack to attach to the Neo task")
	cmd.Flags().StringVar(&orgFlag, "org", "",
		"The organization that owns the Neo task (defaults to the user's default org)")
	cmd.Flags().StringVar(&cwdFlag, "cwd", "",
		"Working directory for local tool execution (defaults to the current directory)")
	cmd.Flags().StringVar(&approvalModeFlag, "approval-mode", string(client.NeoApprovalModeManual),
		"Approval mode for tool calls: 'manual' prompts on every call, 'balanced' "+
			"auto-approves low-risk calls, 'auto' executes everything without prompting")
	cmd.Flags().StringVar(&permissionModeFlag, "permission-mode", string(client.NeoPermissionModeDefault),
		"Permission mode for the agent: 'default' grants full role-based capabilities, "+
			"'read-only' blocks state-mutating operations")
	cmd.Flags().BoolVarP(&printMode, "print", "p", false,
		"Run a single prompt non-interactively, print the agent's final response to "+
			"stdout, and exit. Intended for use with other AI agents and scripts.")
	cmd.Flags().BoolVar(&disableIntegrations, "disable-integrations", false,
		"Run the Neo task with no integration credentials, ignoring any org-enabled "+
			"integrations.")
	cmd.Flags().StringVar(&debugUpdateFlag, "debug-update", "",
		"Debug a failed update. With no value, targets the stack's latest update; "+
			"pass =<version> (e.g. --debug-update=42) to target a specific one")
	cmd.Flags().StringVar(&debugPreviewFlag, "debug-preview", "",
		"Debug a failed preview. With no value, targets the stack's latest preview; "+
			"pass =<preview-id> to target a specific one")
	// A non-empty NoOptDefVal makes the flag's value optional: a bare flag records the sentinel
	// ("infer latest"), while =<id> records the explicit id.
	cmd.Flags().Lookup("debug-update").NoOptDefVal = debugLatestSentinel
	cmd.Flags().Lookup("debug-preview").NoOptDefVal = debugLatestSentinel
	cmd.MarkFlagsMutuallyExclusive("debug-update", "debug-preview")

	return cmd
}

// debugLatestSentinel is the NoOptDefVal for --debug-update/--debug-preview: pflag requires a
// non-empty NoOptDefVal to make a flag's value optional, so a bare flag records this sentinel
// ("infer the latest"). It is untypeable as a real id so it can't collide with a user-passed value.
const debugLatestSentinel = "\x00latest"

// valueOrEmpty maps the bare-flag sentinel back to "" so callers see an explicit id or nothing.
func valueOrEmpty(v string) string {
	if v == debugLatestSentinel {
		return ""
	}
	return v
}

// parseApprovalMode validates the --approval-mode flag value against the
// NeoApprovalMode enum. The cloud rejects unknown values too, but a CLI-side
// check produces a clearer error before any network round-trip.
func parseApprovalMode(s string) (client.NeoApprovalMode, error) {
	switch client.NeoApprovalMode(s) {
	case client.NeoApprovalModeManual,
		client.NeoApprovalModeBalanced,
		client.NeoApprovalModeAuto:
		return client.NeoApprovalMode(s), nil
	}
	return "", fmt.Errorf("invalid --approval-mode %q: expected one of manual, balanced, auto", s)
}

// parsePermissionMode validates the --permission-mode flag value against the
// NeoPermissionMode enum.
func parsePermissionMode(s string) (client.NeoPermissionMode, error) {
	switch client.NeoPermissionMode(s) {
	case client.NeoPermissionModeDefault, client.NeoPermissionModeReadOnly:
		return client.NeoPermissionMode(s), nil
	}
	return "", fmt.Errorf("invalid --permission-mode %q: expected one of default, read-only", s)
}

// neoRunOptions carries everything runNeo needs to start a Neo session.
type neoRunOptions struct {
	prompt         string
	stackName      string
	orgFlag        string
	cwdFlag        string
	approvalMode   client.NeoApprovalMode
	permissionMode client.NeoPermissionMode
	printMode      bool
	// debugKind/debugID make this a debug session: runNeo seeds a prompt targeting a failed
	// operation of that kind and appends the stack context. debugKind is debugNone for a normal
	// session; debugID is "" to infer the latest of debugKind.
	debugKind           debugKind
	debugID             string
	disableIntegrations bool
}

func runNeo(ctx context.Context, stdout, stderr io.Writer, opts neoRunOptions) error {
	// nil lets the server inherit the org's enabled integrations; the empty slice opts out.
	var enabledIntegrations *[]string
	if opts.disableIntegrations {
		enabledIntegrations = &[]string{}
	}

	if opts.cwdFlag == "" {
		var err error
		opts.cwdFlag, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("resolving working directory: %w", err)
		}
	}

	ws := pkgWorkspace.Instance
	displayOpts := display.Options{Color: cmdutil.GetGlobalColorization()}

	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, displayOpts)
	if err != nil {
		return err
	}
	cloudBe, ok := be.(httpstate.Backend)
	if !ok {
		return errors.New("`pulumi neo` requires the Pulumi Cloud backend")
	}
	pc := cloudBe.Client()

	if msg := neoUpgradeMessage(cloudBe.Capabilities(ctx), version.Version); msg != "" {
		return result.FprintBailf(stderr, "%s", msg)
	}

	target, err := resolveTaskTarget(ctx, ws, cloudBe, project, opts.stackName, opts.orgFlag)
	if err != nil {
		return err
	}
	orgName, projectName, stackRefName := target.org, target.project, target.stackName()

	// In a debug session, prepend the seed trigger line (so Neo's skill evaluator matches) and
	// append the stack context. Any positional prompt sits between them as extra guidance.
	if opts.debugKind != debugNone {
		id := opts.debugID
		if id == "" {
			id = opts.debugKind.latestID(ctx, cloudBe, target.ref)
		}
		seed := debugSeedPrompt(opts.debugKind, id)
		if opts.prompt != "" {
			seed += "\n\n" + opts.prompt
		}
		opts.prompt = seed + "\n\n" + debugStackContext(cloudBe, target, opts.debugKind, id)
	}

	// Allow tools to read/write under temp directories in addition to cwd: the agent
	// stages scratch files there (downloads, intermediate state) and the CLI sandbox
	// would otherwise reject those paths. See pulumi/pulumi-service#42027.
	extraRoots := dedupeExistingRoots("/tmp", os.TempDir())
	fs, err := tools.NewFilesystem(opts.cwdFlag, extraRoots...)
	if err != nil {
		return err
	}
	sh, err := tools.NewShell(opts.cwdFlag, extraRoots...)
	if err != nil {
		return err
	}
	handlers := map[string]ToolHandler{
		"filesystem": fs,
		"shell":      sh,
	}

	// In non-interactive mode the sink stays nil and live events are dropped; the
	// interactive path below sets pu.Sink to push UIEvents onto uiCh.
	pu, err := tools.NewPulumi(opts.cwdFlag, ws, nil)
	if err != nil {
		return err
	}
	handlers["pulumi"] = pu

	if opts.printMode || !isInteractive() {
		if opts.prompt == "" {
			return errors.New("a prompt argument is required in non-interactive mode")
		}
		taskPrompt := nonInteractivePromptPreamble + "\n\n" + opts.prompt
		resp, err := createNeoTaskWithEntityRetry(
			ctx, pc, orgName, taskPrompt, stackRefName, projectName, client.CreateNeoTaskOptions{
				ToolExecutionMode:   "cli",
				ApprovalMode:        opts.approvalMode,
				PermissionMode:      opts.permissionMode,
				EnabledIntegrations: enabledIntegrations,
			}, nil)
		if err != nil {
			return err
		}
		if !opts.printMode {
			consoleURL := client.CloudConsoleURL(pc.URL(), orgName, "neo", "tasks", resp.TaskID)
			if consoleURL != "" {
				fmt.Fprintln(stderr, consoleURL)
			} else {
				fmt.Fprintf(stderr, "Neo task created (id %s)\n", resp.TaskID)
			}
		}
		session := &Session{
			Client:   pc,
			Handlers: handlers,
			OrgName:  orgName,
			TaskID:   resp.TaskID,
			Log:      stderr,
		}
		if opts.printMode {
			session.Output = stdout
		}
		return session.Run(ctx)
	}

	uiCh := make(chan UIEvent, 64)
	defer close(uiCh)
	outCh := make(chan outboundEvent, 8)

	pu.Sink = newPulumiSinkForUI(uiCh)

	username, _, _, _ := pc.GetPulumiAccountDetails(ctx)

	// Detect the terminal background once, before bubbletea takes over stdin.
	// Querying in-band (via tea.RequestBackgroundColor or glamour's auto-style)
	// races bubbletea's own input reader for the terminal's reply, which leaks
	// the response into the textarea; lipgloss queries synchronously here and
	// consumes its own response. Defaults to dark on any error.
	//
	//nolint:forbidigo // needs the real terminal fds to query the background; cmd.OutOrStdout() is an io.Writer
	hasDarkBackground := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)

	model := NewModel(ModelConfig{
		Org:                   orgName,
		WorkDir:               opts.cwdFlag,
		Username:              username,
		Version:               version.Version,
		EventCh:               uiCh,
		OutCh:                 outCh,
		Busy:                  opts.prompt != "",
		InitialPrompt:         opts.prompt,
		InitialApprovalMode:   opts.approvalMode,
		InitialPermissionMode: opts.permissionMode,
		HasDarkBackground:     hasDarkBackground,
	})

	// Inline (non-alt-screen) so the transcript stays in the user's terminal
	// scrollback after exit.
	p := newTeaProgram(model)

	return runWithTUI(
		ctx,
		func() error {
			_, err := p.Run()
			return err
		},
		func(g *errgroup.Group, gctx context.Context) {
			// taskState tracks the task ID once created (may be deferred if no prompt).
			type taskState struct {
				mu     sync.Mutex
				taskID string
			}
			ts := &taskState{}

			// createTask creates the Neo task with the given prompt and starts the session.
			// Called immediately if a prompt was provided, or on the first user message.
			// approvalMode / permissionMode / planMode are the values the TUI captured at
			// the moment the first message was sent; the CLI-prompt path passes the values
			// from the --approval-mode / --permission-mode flags and false for plan mode.
			createTask := func(
				initialPrompt string,
				createApprovalMode client.NeoApprovalMode,
				createPermissionMode client.NeoPermissionMode,
				planMode bool,
			) error {
				resp, err := createNeoTaskWithEntityRetry(
					gctx, pc, orgName, initialPrompt, stackRefName, projectName, client.CreateNeoTaskOptions{
						ToolExecutionMode:   "cli",
						ApprovalMode:        createApprovalMode,
						PermissionMode:      createPermissionMode,
						PlanMode:            planMode,
						EnabledIntegrations: enabledIntegrations,
					}, func(originalErr error) {
						sendUI(uiCh, UIWarning{Message: fmt.Sprintf(
							"could not attach stack %s/%s/%s to Neo task: %s; "+
								"creating task without stack context",
							orgName, projectName, stackRefName, originalErr,
						)})
					})
				if err != nil {
					sendUI(uiCh, UIError{Message: "failed to create Neo task: " + err.Error()})
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

			if opts.prompt != "" {
				// The command-line prompt path always passes false for planMode and
				// uses the modes parsed from the CLI flags (which the TUI also seeds
				// into its model). A subsequent toggle still routes through the TUI.
				g.Go(func() error {
					return createTask(opts.prompt, opts.approvalMode, opts.permissionMode, false)
				})
			}

			// Tear the bubbletea program down when any errgroup goroutine fails.
			// Without this, p.Run() blocks until the user manually quits, so a
			// CreateNeoTask failure would leave the user staring at a TUI with
			// no backing session. runWithTUI cancels gctx when runTUI itself
			// returns, so this goroutine is a no-op on clean TUI exits and only
			// fires p.Quit when a worker (createTask, dispatcher, session) errors.
			g.Go(func() error {
				<-gctx.Done()
				p.Quit()
				return nil
			})

			// Post TUI-originated user events to the API. Chat messages may arrive before
			// any task exists — the first one creates it. Other event types (approvals
			// and anything we add later) only make sense once a task is live.
			g.Go(func() error {
				return dispatchUserEvents(
					gctx, outCh, uiCh,
					opts.prompt != "",
					func() string {
						ts.mu.Lock()
						defer ts.mu.Unlock()
						return ts.taskID
					},
					func(message string, am client.NeoApprovalMode, pm client.NeoPermissionMode, planMode bool) {
						g.Go(func() error {
							return createTask(message, am, pm, planMode)
						})
					},
					func(ctx context.Context, taskID string, body any) error {
						return pc.PostNeoTaskUserEvent(ctx, orgName, taskID, body)
					},
					func(ctx context.Context, taskID string, opts client.UpdateNeoTaskOptions) error {
						return pc.UpdateNeoTask(ctx, orgName, taskID, opts)
					},
				)
			})
		},
	)
}

// runWithTUI runs the bubbletea program alongside caller-registered worker
// goroutines under a shared errgroup. When runTUI returns the shared context
// is cancelled, so any worker watching gctx.Done can unblock and return.
//
// errgroup only cancels its derived context on a non-nil error, but tea.Quit
// returns nil from p.Run — without this explicit cancellation, the dispatcher
// and any active session.Run loop would block on gctx.Done forever, g.Wait
// would never return, and `pulumi neo` would require a third Ctrl+C to exit.
//
// register is invoked synchronously before the TUI goroutine starts so callers
// can stage their workers; it may also retain g to spawn additional workers
// later (the dispatcher does this when the first user message arrives).
func runWithTUI(
	ctx context.Context,
	runTUI func() error,
	register func(g *errgroup.Group, gctx context.Context),
) error {
	ctx, cancelAll := context.WithCancel(ctx)
	defer cancelAll()

	g, gctx := errgroup.WithContext(ctx)

	register(g, gctx)

	g.Go(func() error {
		err := runTUI()
		cancelAll()
		return err
	})

	return g.Wait()
}

// dispatchUserEvents drives the runNeo dispatcher loop: it reads TUI-originated
// user events from outCh, posts them to the backend once a task exists, and
// lazily creates the task on the first user_message when no initial prompt was
// provided. The function returns when ctx is cancelled or outCh is closed.
//
// Extracted from runNeo so each branch (lazy creation, taskID-not-ready
// warning, post error) can be unit-tested without standing up the full
// interactive machinery.
//
// initialTaskCreated reflects whether the caller has already kicked off
// CreateNeoTask (true when runNeo was given a CLI prompt, false when it must
// wait for the user's first chat message). spawnCreateTask is invoked when the
// lazy path triggers; the caller wires it to its own goroutine orchestration
// (the production caller spawns into the runWithTUI errgroup).
func dispatchUserEvents(
	ctx context.Context,
	outCh <-chan outboundEvent,
	uiCh chan<- UIEvent,
	initialTaskCreated bool,
	getTaskID func() string,
	spawnCreateTask func(message string, approvalMode client.NeoApprovalMode,
		permissionMode client.NeoPermissionMode, planMode bool),
	postEvent func(ctx context.Context, taskID string, body any) error,
	updateTask func(ctx context.Context, taskID string, opts client.UpdateNeoTaskOptions) error,
) error {
	taskCreated := initialTaskCreated
	for {
		select {
		case <-ctx.Done():
			return nil
		case ob, ok := <-outCh:
			if !ok {
				return nil
			}
			if msg, isMsg := ob.event.(apitype.AgentUserEventUserMessage); isMsg && !taskCreated {
				taskCreated = true
				spawnCreateTask(msg.Content, ob.approvalMode, ob.permissionMode, ob.planMode)
				continue
			}
			taskID := getTaskID()
			if taskID == "" {
				// A pre-task-creation mode toggle is a no-op: CreateNeoTask hasn't
				// fired yet, so the next createTask call will pick up the latest
				// values from the model snapshot. Drop silently — this is the
				// expected path when the user toggles before sending a message.
				if ob.update != nil {
					continue
				}
				// Unreachable in normal use: the TUI gates Enter on busy state
				// until UITaskIdle, and approvals only fire in response to backend
				// events that imply the task exists. Surface instead of silently
				// dropping.
				sendUI(uiCh, UIWarning{Message: "dropped event: task not ready"})
				continue
			}
			if ob.update != nil {
				if err := updateTask(ctx, taskID, *ob.update); err != nil {
					sendUI(uiCh, UIWarning{Message: "failed to update Neo task: " + err.Error()})
				}
				continue
			}
			if err := postEvent(ctx, taskID, ob.event); err != nil {
				sendUI(uiCh, UIWarning{Message: "failed to send event: " + err.Error()})
			}
		}
	}
}

// stackRefWithOrg is the subset of backend.StackReference that carries the
// owning organization. cloud/diy/mock stack references all implement it; we
// type-assert to it so the Neo task is created in the same org as the
// resolved stack rather than silently retargeting to the user's default org.
type stackRefWithOrg interface {
	Organization() (string, bool)
}

// taskTarget is the resolved org, project, and stack a Neo task attaches to. The stack name comes
// from ref; ref is nil when no stack could be resolved, so the task runs without stack context.
type taskTarget struct {
	org     string
	project string
	ref     backend.StackReference
}

// stackName returns the resolved stack's name, or "" when no stack was resolved.
func (t taskTarget) stackName() string {
	if t.ref == nil {
		return ""
	}
	return t.ref.Name().String()
}

// resolveTaskTarget figures out the org, project, and stack to attach to the new Neo task. The
// stack flag is optional — if it's empty we try the currently selected stack and fall back to a
// project-only attachment if there isn't one.
//
// Org resolution: --org wins if provided; otherwise we use the owner carried
// by the stack reference (so a workspace-selected `otherorg/proj/dev` keeps
// `otherorg` instead of silently retargeting to the user's default org, the
// way `pulumi preview` would); only when neither is set do we fall back to
// the backend's default org.
func resolveTaskTarget(
	ctx context.Context,
	ws pkgWorkspace.Context,
	be httpstate.Backend,
	project *workspace.Project,
	stackName, orgFlag string,
) (taskTarget, error) {
	var t taskTarget
	if project != nil {
		t.project = string(project.Name)
	}

	var stackOwner string
	if stackName != "" {
		ref, err := be.ParseStackReference(stackName)
		if err != nil {
			return taskTarget{}, err
		}
		t.ref = ref
		if owned, ok := ref.(stackRefWithOrg); ok {
			if o, has := owned.Organization(); has {
				stackOwner = o
			}
		}
	} else {
		s, err := state.CurrentStack(ctx, ws, be)
		if err == nil && s != nil {
			t.ref = s.Ref()
			if owned, ok := s.Ref().(stackRefWithOrg); ok {
				if o, has := owned.Organization(); has {
					stackOwner = o
				}
			}
		}
	}

	switch {
	case orgFlag != "":
		t.org = orgFlag
	case stackOwner != "":
		t.org = stackOwner
	default:
		org, err := be.GetDefaultOrg(ctx)
		if err != nil {
			return taskTarget{}, fmt.Errorf("determining default organization: %w", err)
		}
		t.org = org
	}
	if t.org == "" {
		return taskTarget{}, errors.New("could not determine an organization for the Neo task; pass --org")
	}
	return t, nil
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
