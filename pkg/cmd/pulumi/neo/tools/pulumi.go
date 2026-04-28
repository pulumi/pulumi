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

package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	backendDisplay "github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/autonaming"
	cmdConfig "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/config"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/metadata"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Pulumi is the local handler for the Neo `pulumi` tool. It implements the two tool
// methods exposed by the cloud agent's pulumi_mcp server:
//
//	pulumi_preview — runs `pulumi preview` against the selected stack
//	pulumi_up      — runs `pulumi up` against the selected stack (mutating)
//
// The argument and return shapes mirror the upstream definitions in
// pulumi-service:cmd/agents/src/agents_py/mcp/pulumi_mcp.py. The upstream tool was
// designed for the cloud-run Deployment path; when running under Neo CLI mode we
// return empty strings for `deployment_id` and `branch_name` (there is no Deployment
// and no git branch is created), and populate `events_file` with a temp NDJSON file
// of engine events for the agent to read via the filesystem tool.
//
// Mutation safety: `pulumi_up` is gated upstream via NeoApprovalModeManual — the
// Neo service sends a user_approval_request before dispatching the tool, and the
// TUI renders it via UIApprovalRequest. This handler does no local pre-flight
// confirmation; it expects the approval contract to be honored upstream.
type Pulumi struct {
	// Cwd is the canonical sandbox root. LocalPulumiDir in args must resolve under it.
	Cwd string
	// Workspace is used to read the project file from the working directory.
	Workspace pkgWorkspace.Context
	// Backend is the (cloud) backend used to resolve stacks and execute operations.
	Backend backend.Backend
	// Sink, when non-nil, receives structured events that power the Neo TUI's
	// live preview/up block. See PulumiSink for the callback surface. All
	// fields are optional: a nil callback on any one method is silently skipped.
	Sink *PulumiSink
}

// PulumiSink is the structured callback surface for live preview/up progress.
// The neo package wires each callback to a matching UIEvent so the TUI can
// build a persistent block with resources and diagnostics. The callbacks run on
// the drain goroutine, so implementations must not block.
//
// toolName is the full wire name ("pulumi__pulumi_preview" / "pulumi__pulumi_up")
// used by the TUI to correlate events to the active block. Secret values from
// `environment_variables` never flow into any callback argument.
type PulumiSink struct {
	// OnStart opens a new block when the tool call starts. stackName is the
	// short stack name from args; isPreview differentiates preview from up.
	OnStart func(toolName, stackName string, isPreview bool)
	// OnResource reports one resource the engine touched. op is the typed
	// StepOp from the engine; status is one of "planned" (preview),
	// "running" (up, pre-event), "done" (up, outputs), or "failed" (up,
	// operation-failed). Duplicate URNs are accepted — the TUI dedupes
	// and upgrades status in place.
	OnResource func(toolName string, op display.StepOp, urn, typ, status string)
	// OnDiag reports one engine diagnostic. urn may be empty for stack-level
	// diagnostics.
	OnDiag func(toolName, severity, message, urn string)
	// OnEnd finalizes the block. err is empty on success, otherwise the
	// wrapped engine error string. counts is the typed ResourceChanges map
	// from the engine.
	OnEnd func(toolName, err string, counts display.ResourceChanges, elapsed string)
}

// NewPulumi creates a Pulumi handler sandboxed under cwd. The workspace and backend
// are captured at construction so tests can inject fakes. Sink may be nil when
// running outside the interactive TUI (non-interactive mode); in that case progress
// is silently dropped and the final result is still returned to the caller.
func NewPulumi(cwd string, ws pkgWorkspace.Context, be backend.Backend,
	sink *PulumiSink,
) (*Pulumi, error) {
	if ws == nil {
		return nil, errors.New("workspace is required")
	}
	if be == nil {
		return nil, errors.New("backend is required")
	}
	abs, err := canonicalRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("resolving pulumi cwd: %w", err)
	}
	return &Pulumi{Cwd: abs, Workspace: ws, Backend: be, Sink: sink}, nil
}

// pulumiArgs matches pulumi-service:cmd/agents/src/agents_py/mcp/pulumi_mcp.py's
// pulumi_preview/pulumi_up parameters.
type pulumiArgs struct {
	ProjectName          string            `json:"project_name"`
	StackName            string            `json:"stack_name"`
	LocalPulumiDir       string            `json:"local_pulumi_dir"`
	EnvironmentVariables map[string]envVal `json:"environment_variables,omitempty"`
}

// envVal decodes the dict value type `str | SecretValue` used by the upstream schema.
// Plain and Secret are mutually exclusive; the Value() accessor returns whichever is set.
// Secret values must never be echoed into logs, progress messages, or the events file.
type envVal struct {
	Plain  string
	Secret string
}

func (e *envVal) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		e.Plain = s
		return nil
	}
	var obj struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return errors.New(`environment_variables value must be a string or {"secret": "..."}`)
	}
	if obj.Secret == "" {
		return errors.New(`environment_variables secret form requires a non-empty "secret" field`)
	}
	e.Secret = obj.Secret
	return nil
}

// Value returns the effective environment variable value regardless of form.
func (e envVal) Value() string {
	if e.Secret != "" {
		return e.Secret
	}
	return e.Plain
}

// pulumiResult matches pulumi-service's PulumiOperationResult so tool consumers on the
// agent side don't care whether the call ran locally or in a Deployment.
type pulumiResult struct {
	DeploymentID  string `json:"deployment_id"`
	ConsoleURL    string `json:"console_url"`
	Logs          string `json:"logs"`
	Status        string `json:"status"`
	BranchName    string `json:"branch_name"`
	ProjectName   string `json:"project_name"`
	StackName     string `json:"stack_name"`
	UpdateSummary string `json:"update_summary,omitempty"`
	EventsFile    string `json:"events_file,omitempty"`
}

// Invoke dispatches a single pulumi method call.
func (p *Pulumi) Invoke(ctx context.Context, method string, args json.RawMessage) (any, error) {
	var a pulumiArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("decoding %s args: %w", method, err)
	}
	switch method {
	case "pulumi_preview":
		return p.run(ctx, a, true)
	case "pulumi_up":
		return p.run(ctx, a, false)
	default:
		return nil, fmt.Errorf("unknown pulumi method %q", method)
	}
}

func (p *Pulumi) run(ctx context.Context, a pulumiArgs, isPreview bool) (pulumiResult, error) {
	if a.StackName == "" {
		return pulumiResult{}, errors.New("stack_name is required")
	}
	if a.LocalPulumiDir == "" {
		return pulumiResult{}, errors.New("local_pulumi_dir is required")
	}
	if !filepath.IsAbs(a.LocalPulumiDir) {
		return pulumiResult{}, errors.New("local_pulumi_dir must be an absolute path")
	}

	// Confine local_pulumi_dir to the Session sandbox.
	dir, err := resolveUnderRoot(p.Cwd, a.LocalPulumiDir, false)
	if err != nil {
		return pulumiResult{}, err
	}
	if _, err := os.Stat(filepath.Join(dir, "Pulumi.yaml")); err != nil {
		return pulumiResult{}, fmt.Errorf("local_pulumi_dir %q: Pulumi.yaml not found", a.LocalPulumiDir)
	}

	// Apply environment variables for the engine run. We snapshot and restore the
	// values we touch so subsequent tool calls aren't affected. Secret values are
	// unwrapped here but never flow into logs/progress/events_file.
	restoreEnv := applyEnvVars(a.EnvironmentVariables)
	defer restoreEnv()

	// cmdStack.LoadProjectStack (called via GetStackConfiguration below) walks up
	// from os.Getwd() to find Pulumi.<stack>.yaml, so we chdir into the project
	// directory for the duration of the call. The engine itself derives its own
	// working directory from op.Root and doesn't depend on cwd. Session.runBatch
	// dispatches tool calls serially so we don't lock here, but os.Chdir is
	// process-global — concurrent callers from outside the Session would race.
	prevDir, err := os.Getwd()
	if err != nil {
		return pulumiResult{}, fmt.Errorf("recording working directory: %w", err)
	}
	if err := os.Chdir(dir); err != nil {
		return pulumiResult{}, fmt.Errorf("chdir %q: %w", dir, err)
	}
	defer func() { _ = os.Chdir(prevDir) }()

	proj, root, err := p.Workspace.ReadProject()
	if err != nil {
		return pulumiResult{}, fmt.Errorf("reading project: %w", err)
	}

	stackRef, err := p.Backend.ParseStackReference(a.StackName)
	if err != nil {
		return pulumiResult{}, fmt.Errorf("parsing stack reference: %w", err)
	}
	s, err := p.Backend.GetStack(ctx, stackRef)
	if err != nil {
		return pulumiResult{}, fmt.Errorf("getting stack: %w", err)
	}
	if s == nil {
		return pulumiResult{}, fmt.Errorf("stack %q not found", a.StackName)
	}

	ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()
	cfg, sm, err := cmdConfig.GetStackConfiguration(ctx, cmdutil.Diag(), ssml, s, proj)
	if err != nil {
		return pulumiResult{}, fmt.Errorf("getting stack configuration: %w", err)
	}

	decrypter := sm.Decrypter()
	encrypter := sm.Encrypter()
	if err := workspace.ValidateStackConfigAndApplyProjectConfig(
		ctx, s.Ref().Name().String(), proj, cfg.Environment, cfg.Config, encrypter, decrypter,
	); err != nil {
		return pulumiResult{}, fmt.Errorf("validating stack config: %w", err)
	}

	autonamer, err := autonaming.ParseAutonamingConfig(
		autonamingStackContextFor(proj, s), cfg.Config, decrypter)
	if err != nil {
		return pulumiResult{}, fmt.Errorf("getting autonaming config: %w", err)
	}

	// Pass nil for flags: GetUpdateMetadata only uses them to record
	// "pulumi.flag.<name>" entries, and Neo has no CLI flags to record.
	m, err := metadata.GetUpdateMetadata("" /*message*/, root,
		"neo" /*execKind*/, "" /*execAgent*/, false /*updatePlan*/, cfg, nil)
	if err != nil {
		return pulumiResult{}, fmt.Errorf("gathering metadata: %w", err)
	}

	opts := backend.UpdateOptions{
		AutoApprove: true, // Upstream approval already gates pulumi_up before dispatch.
		Engine: engine.UpdateOptions{
			Experimental: true,
			Autonamer:    autonamer,
		},
		Display: backendDisplay.Options{
			// Mute the backend's own progress renderer so it doesn't fight the Neo TUI.
			// Stdout / Stderr go to io.Discard; the Neo tool handler consumes events via
			// the events channel instead.
			Color:            colors.Never,
			SuppressProgress: true,
			SuppressOutputs:  true,
			IsInteractive:    false,
			Type:             backendDisplay.DisplayProgress,
			Stdout:           io.Discard,
			Stderr:           io.Discard,
		},
	}

	eventsFile, err := os.CreateTemp("", "pulumi-neo-events-*.ndjson")
	if err != nil {
		return pulumiResult{}, fmt.Errorf("creating events file: %w", err)
	}
	// We deliberately do NOT delete the file on exit — the agent may read it via
	// the filesystem tool after the handler returns. Callers accept the OS temp-dir
	// lifecycle. This matches the upstream Pulumi Cloud behavior.
	eventsPath := eventsFile.Name()

	toolName := "pulumi__pulumi_up"
	if isPreview {
		toolName = "pulumi__pulumi_preview"
	}

	if p.Sink != nil && p.Sink.OnStart != nil {
		p.Sink.OnStart(toolName, a.StackName, isPreview)
	}

	eventsCh := make(chan engine.Event, 128)
	var diagLines []string
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		diagLines = p.drainEvents(toolName, isPreview, eventsCh, eventsFile)
		_ = eventsFile.Close()
	}()

	startedAt := time.Now()
	op := backend.UpdateOperation{
		Proj:               proj,
		Root:               root,
		M:                  m,
		Opts:               opts,
		StackConfiguration: cfg,
		SecretsManager:     sm,
		SecretsProvider:    secrets.DefaultProvider,
		Scopes:             ctxOnlyCancellationSource{},
	}

	// The engine and backend print a handful of headers/messages directly to
	// os.Stdout/os.Stderr (e.g. the "Previewing update (stack)" banner in
	// pkg/backend/httpstate/backend.go) that bypass Display.Stdout. Under the
	// Neo TUI this corrupts bubbletea's alt-screen, so redirect both streams to
	// /dev/null for the duration of the backend call. bubbletea holds a
	// captured reference to the original stdout from tea.NewProgram, so this
	// swap doesn't disturb rendering. Deferred so a panic in the engine
	// can't leave the CLI permanently muted.
	restoreStd := silenceStd()
	defer restoreStd()

	var runErr error
	var changes display.ResourceChanges
	if isPreview {
		_, changes, runErr = backend.PreviewStack(ctx, s, op, eventsCh)
	} else {
		changes, runErr = backend.UpdateStack(ctx, s, op, eventsCh)
	}
	// Closing eventsCh lets the drain goroutine return. The backend does not close
	// the channel for us; once the backend call returns, no more events will arrive.
	close(eventsCh)
	<-drainDone

	res := pulumiResult{
		ProjectName: a.ProjectName,
		StackName:   a.StackName,
		EventsFile:  eventsPath,
	}
	if res.ProjectName == "" {
		res.ProjectName = proj.Name.String()
	}

	switch {
	case runErr != nil && errors.Is(ctx.Err(), context.Canceled):
		res.Status = "cancelled"
	case runErr != nil:
		res.Status = "failed"
	default:
		res.Status = "succeeded"
	}

	elapsed := time.Since(startedAt)
	res.Logs = formatLogs(changes, diagLines)
	res.UpdateSummary = formatUpdateSummary(a.StackName, changes, elapsed)

	if p.Sink != nil && p.Sink.OnEnd != nil {
		errStr := ""
		if runErr != nil {
			errStr = runErr.Error()
		}
		p.Sink.OnEnd(toolName, errStr, changes, elapsed.Round(time.Second).String())
	}

	if runErr != nil {
		label := "up"
		if isPreview {
			label = "preview"
		}
		return res, fmt.Errorf("pulumi %s: %w", label, runErr)
	}
	return res, nil
}

// OpSortRank orders StepOps for human-readable output: creates first, then
// updates, replaces, deletes, reads, refreshes, imports, then everything else.
// Exported so the TUI side can sort UI-counts the same way the agent-facing
// summary does.
func OpSortRank(op display.StepOp) int {
	switch op {
	case deploy.OpCreate, deploy.OpCreateReplacement:
		return 0
	case deploy.OpUpdate:
		return 1
	case deploy.OpReplace:
		return 2
	case deploy.OpDelete, deploy.OpDeleteReplaced:
		return 3
	case deploy.OpRead, deploy.OpReadReplacement:
		return 4
	case deploy.OpRefresh:
		return 5
	case deploy.OpImport, deploy.OpImportReplacement:
		return 6
	case deploy.OpSame:
		return 9
	default:
		return 7
	}
}

// FormatChangeCounts produces a "3 create<joiner>1 update" string from a
// ResourceChanges map. OpSame and zero entries are filtered. Returns "" when
// no non-trivial changes remain.
func FormatChangeCounts(changes display.ResourceChanges, joiner string) string {
	if len(changes) == 0 {
		return ""
	}
	type kv struct {
		op display.StepOp
		n  int
	}
	kvs := make([]kv, 0, len(changes))
	for op, n := range changes {
		if op == deploy.OpSame || n == 0 {
			continue
		}
		kvs = append(kvs, kv{op: op, n: n})
	}
	sort.Slice(kvs, func(i, j int) bool { return OpSortRank(kvs[i].op) < OpSortRank(kvs[j].op) })
	parts := make([]string, 0, len(kvs))
	for _, e := range kvs {
		parts = append(parts, fmt.Sprintf("%d %s", e.n, e.op))
	}
	return strings.Join(parts, joiner)
}

// drainEvents consumes the engine event channel: writes each event as one
// NDJSON line to the events file (the canonical record the agent reads via
// the filesystem tool) and dispatches structured sink callbacks for the
// TUI's live preview block. It returns the accumulated diagnostic lines so
// they can be folded into pulumiResult.Logs at the end of the run; per-resource
// lines are intentionally not duplicated in memory because the events file
// already holds them.
func (p *Pulumi) drainEvents(
	toolName string, isPreview bool, events <-chan engine.Event, ndjson io.Writer,
) []string {
	var diags []string

	for e := range events {
		// Best-effort: skip events that fail to convert.
		if apiEv, err := backendDisplay.ConvertEngineEvent(e, false /*showSecrets*/); err == nil {
			if b, err := json.Marshal(apiEv); err == nil {
				_, _ = ndjson.Write(b)
				_, _ = ndjson.Write([]byte("\n"))
			}
		}

		switch payload := e.Payload().(type) {
		case engine.ResourcePreEventPayload:
			if payload.Internal {
				continue
			}
			op := payload.Metadata.Op
			if op == deploy.OpSame {
				continue
			}
			urn := payload.Metadata.URN
			status := "running"
			if isPreview {
				status = "planned"
			}
			if p.Sink != nil && p.Sink.OnResource != nil {
				p.Sink.OnResource(toolName, op, string(urn), string(urn.Type()), status)
			}
		case engine.ResourceOutputsEventPayload:
			if payload.Internal {
				continue
			}
			if isPreview {
				// In preview we don't run resources, so there's no status
				// upgrade to do. Skip to avoid duplicating "planned" rows.
				continue
			}
			if payload.Metadata.Op == deploy.OpSame {
				continue
			}
			urn := payload.Metadata.URN
			if p.Sink != nil && p.Sink.OnResource != nil {
				p.Sink.OnResource(toolName, payload.Metadata.Op,
					string(urn), string(urn.Type()), "done")
			}
		case engine.ResourceOperationFailedPayload:
			urn := payload.Metadata.URN
			if p.Sink != nil && p.Sink.OnResource != nil {
				p.Sink.OnResource(toolName, payload.Metadata.Op,
					string(urn), string(urn.Type()), "failed")
			}
		case engine.DiagEventPayload:
			if payload.Ephemeral {
				continue
			}
			if payload.Severity != diag.Warning && payload.Severity != diag.Error {
				continue
			}
			// The engine embeds Pulumi color markers (e.g. <{%reset%}>) in
			// diagnostic messages and expects the display layer to substitute
			// them. Run through colors.Never to strip to plain text; the TUI
			// paints its own severity color on the row, so we don't need ANSI.
			msg := strings.TrimSpace(colors.Never.Colorize(payload.Message))
			if p.Sink != nil && p.Sink.OnDiag != nil {
				p.Sink.OnDiag(toolName, string(payload.Severity), msg, string(payload.URN))
			}
			diags = append(diags, fmt.Sprintf("%s: %s", payload.Severity, msg))
		}
	}
	return diags
}

// formatLogs builds the agent-facing pulumiResult.Logs string: a counts line
// followed by any non-ephemeral diagnostics. Full per-resource detail lives in
// EventsFile.
func formatLogs(changes display.ResourceChanges, diags []string) string {
	var buf strings.Builder
	if counts := FormatChangeCounts(changes, ", "); counts != "" {
		buf.WriteString("summary: " + counts + "\n")
	}
	for _, d := range diags {
		buf.WriteString(d + "\n")
	}
	return buf.String()
}

// formatUpdateSummary renders the human-readable summary returned in
// pulumiResult.UpdateSummary: stack + elapsed header followed by the op counts.
// Per-resource detail is in EventsFile so we don't duplicate it here.
func formatUpdateSummary(stackName string, changes display.ResourceChanges,
	elapsed time.Duration,
) string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "stack: %s (%s)\n", stackName, elapsed.Round(time.Second))
	if counts := FormatChangeCounts(changes, ", "); counts != "" {
		fmt.Fprintf(&buf, "changes: %s\n", counts)
	} else {
		buf.WriteString("changes: none\n")
	}
	return buf.String()
}

// applyEnvVars sets the given environment variables on the process, returning a
// function that restores the prior values. Variables previously unset are unset on
// restore. Secret values are applied normally; restoreEnv does not log or return them.
func applyEnvVars(vars map[string]envVal) func() {
	if len(vars) == 0 {
		return func() {}
	}
	type prev struct {
		value string
		had   bool
	}
	saved := make(map[string]prev, len(vars))
	for k, v := range vars {
		old, ok := os.LookupEnv(k)
		saved[k] = prev{value: old, had: ok}
		_ = os.Setenv(k, v.Value())
	}
	return func() {
		for k, pv := range saved {
			if pv.had {
				_ = os.Setenv(k, pv.value)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}
}

// ctxOnlyCancellationSource is a minimal backend.CancellationScopeSource that observes
// only the caller's context — no SIGINT/SIGTERM handler is installed. Under Neo the TUI
// owns the terminal and posts AgentUserEventCancel on ESC; the backend's stock
// CancellationScopes would install its own SIGINT handler and conflict with bubbletea.
type ctxOnlyCancellationSource struct{}

func (ctxOnlyCancellationSource) NewScope(
	ctx context.Context, _ chan<- engine.Event, _ bool,
) backend.CancellationScope {
	cctx, src := cancel.NewContext(ctx)
	return &ctxOnlyCancellationScope{ctx: cctx, src: src}
}

type ctxOnlyCancellationScope struct {
	ctx *cancel.Context
	src *cancel.Source
}

func (c *ctxOnlyCancellationScope) Context() *cancel.Context { return c.ctx }
func (c *ctxOnlyCancellationScope) Close()                   { c.src.Cancel() }

// silenceStd redirects os.Stdout and os.Stderr to /dev/null and returns a
// restore func. Calls to fmt.Printf / fmt.Println / fmt.Print during the
// redirection go nowhere. Safe even if opening /dev/null fails — in that case
// the originals remain in place. bubbletea's tea.NewProgram captures a stable
// reference to the terminal at construction, so this swap does not affect the
// TUI's rendering; it only catches writes that look up os.Stdout dynamically.
func silenceStd() func() {
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return func() {}
	}
	origStdout, origStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = null, null
	return func() {
		os.Stdout, os.Stderr = origStdout, origStderr
		_ = null.Close()
	}
}

// autonamingStackContextFor mirrors operations.autonamingStackContext without pulling in
// the cobra-coupled operations package.
func autonamingStackContextFor(proj *workspace.Project, s backend.Stack) autonaming.StackContext {
	organization := "organization"
	if cs, ok := s.(httpstate.Stack); ok {
		organization = cs.OrgName()
	}
	return autonaming.StackContext{
		Organization: organization,
		Project:      proj.Name.String(),
		Stack:        s.Ref().Name().String(),
	}
}
