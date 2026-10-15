// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	resourceStack "github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

// candidatePreviewManifest is written by the trusted delivery runner. Source checkout happens
// before this command starts; directories are relative to that immutable release workspace.
// Members holds only stacks that are runnable (every input resolved); UnpreviewableStacks holds
// stacks the server excluded because an input depends on an output no stage has produced yet.
// Members may be empty when UnpreviewableStacks is not.
type candidatePreviewManifest struct {
	CandidateID         string                             `json:"candidateId"`
	ShapeVersion        int                                `json:"shapeVersion"`
	ReleaseID           string                             `json:"releaseId"`
	ChangeRequestID     string                             `json:"changeRequestId"`
	RevisionNumber      int                                `json:"revisionNumber"`
	WorkflowRunID       string                             `json:"workflowRunId"`
	Workspace           string                             `json:"workspace"`
	Organization        string                             `json:"organization,omitempty"`
	Members             []candidatePreviewMember           `json:"members"`
	UnpreviewableStacks []candidatePreviewUnavailableStack `json:"unpreviewableStacks,omitempty"`
}

// candidatePreviewUnavailableStack is a stack the server excluded from this preview attempt
// because one of its inputs depends on an output no stage has produced yet. It is a terminal
// disposition for this preview attempt: never previewed, never reported as unchanged.
type candidatePreviewUnavailableStack struct {
	URN         string `json:"urn"`
	Name        string `json:"name"`
	Stage       string `json:"stage"`
	TargetStack string `json:"targetStack"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
}

type candidatePreviewMember struct {
	URN          string                                 `json:"urn"`
	Name         string                                 `json:"name"`
	Stage        string                                 `json:"stage"`
	TargetStack  string                                 `json:"targetStack"`
	Directory    string                                 `json:"directory"`
	Source       json.RawMessage                        `json:"source,omitempty"`
	Config       map[string]candidatePreviewConfigValue `json:"config,omitempty"`
	Environment  json.RawMessage                        `json:"environment,omitempty"`
	Dependencies []string                               `json:"dependencies,omitempty"`
	// Group and GroupOrder place the member in a stage group. Groups run one after another in
	// ascending GroupOrder; members of one group preview concurrently inside one coherence
	// window subgroup. A manifest without GroupOrder is a single group.
	Group      string `json:"group,omitempty"`
	GroupOrder int    `json:"groupOrder,omitempty"`
}

type candidatePreviewConfigValue struct {
	Value  string `json:"value"`
	Secret bool   `json:"secret,omitempty"`
}

type candidatePreviewEnvironment struct {
	Env            map[string]candidatePreviewConfigValue `json:"env,omitempty"`
	Environments   []string                               `json:"environments,omitempty"`
	PreRunCommands []string                               `json:"preRunCommands,omitempty"`
}

func (c *command) previewCandidateCommand() *cobra.Command {
	manifestPath := os.Getenv("PULUMI_DELIVERY_PREVIEW_MANIFEST")
	cmd := &cobra.Command{
		Use:    "preview-candidate",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			manifest, err := readCandidatePreviewManifest(manifestPath)
			if err != nil {
				return err
			}
			api, stack, err := c.client(cmd.Context())
			if err != nil {
				return err
			}
			if manifest.Organization == "" {
				manifest.Organization = stack.Owner
			}
			stacks, plan, events, previewErr := runCandidatePreview(cmd.Context(), manifest, api)
			if previewErr != nil {
				// The root command silences returned errors, and the executor only relays
				// the exit status, so print the failure (member stack + engine diagnostics
				// for a MemberError) before reporting it.
				fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", previewErr)
			}
			request := candidatePreviewRequest(manifest, stacks, plan, events, previewErr)
			if callbackErr := api.CompleteDeliveryCandidatePreview(cmd.Context(), stack,
				manifest.CandidateID, request); callbackErr != nil {
				return fmt.Errorf("reporting delivery candidate preview: %w", callbackErr)
			}
			return previewErr
		},
	}
	cmd.Flags().StringVar(&manifestPath, "manifest", manifestPath, "Path to the delivery candidate preview manifest")
	return cmd
}

// candidatePreviewRequest builds the service callback body. On failure, Stacks still carries the
// manifest's known unavailable rows (deterministic from the manifest alone) even though the run
// never got far enough to produce them via runCandidatePreview; a failed preview discards all results
// on error, so there is nothing else known about the runnable members to report.
func candidatePreviewRequest(
	manifest candidatePreviewManifest, stacks []client.DeliveryCandidatePreviewStack,
	plan json.RawMessage, events json.RawMessage, previewErr error,
) client.DeliveryCandidatePreviewRequest {
	request := client.DeliveryCandidatePreviewRequest{
		ShapeVersion: manifest.ShapeVersion, ReleaseID: manifest.ReleaseID,
		ChangeRequestID: manifest.ChangeRequestID, RevisionNumber: manifest.RevisionNumber,
		WorkflowRunID: manifest.WorkflowRunID,
	}
	if previewErr != nil {
		request.Status, request.Error = "failed", previewErr.Error()
		request.Stacks = unavailableStackRows(manifest.UnpreviewableStacks)
		return request
	}
	request.Status, request.Stacks, request.Plan, request.Events = "succeeded", stacks, plan, events
	return request
}

func readCandidatePreviewManifest(path string) (candidatePreviewManifest, error) {
	var manifest candidatePreviewManifest
	if path == "" {
		return manifest, errors.New("delivery candidate preview manifest is required")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return manifest, fmt.Errorf("reading delivery candidate preview manifest: %w", err)
	}
	if err = json.Unmarshal(contents, &manifest); err != nil {
		return manifest, fmt.Errorf("decoding delivery candidate preview manifest: %w", err)
	}
	if manifest.WorkflowRunID == "" {
		manifest.WorkflowRunID = os.Getenv("PULUMI_DELIVERY_WORKFLOW_RUN_ID")
	}
	if manifest.WorkflowRunID == "" {
		manifest.WorkflowRunID = os.Getenv("PULUMI_WORKFLOW_RUN_ID")
	}
	if manifest.CandidateID == "" || manifest.ShapeVersion <= 0 || manifest.ReleaseID == "" ||
		manifest.ChangeRequestID == "" || manifest.RevisionNumber < 0 || manifest.WorkflowRunID == "" {
		return manifest, errors.New("delivery candidate preview manifest is incomplete")
	}
	if len(manifest.Members) == 0 && len(manifest.UnpreviewableStacks) == 0 {
		return manifest, errors.New("delivery candidate preview manifest has no members or unpreviewable stacks")
	}
	// Workspace resolution and per-member directories are only needed when there is something to
	// actually preview; an all-unavailable manifest never touches the filesystem.
	if len(manifest.Members) > 0 && manifest.Workspace == "" {
		return manifest, errors.New("delivery candidate preview manifest is incomplete")
	}
	urns := make(map[string]bool, len(manifest.Members)+len(manifest.UnpreviewableStacks))
	for _, member := range manifest.Members {
		if member.URN == "" || member.TargetStack == "" || member.Directory == "" {
			return manifest, errors.New("delivery candidate preview member is incomplete")
		}
		if urns[member.URN] {
			return manifest, fmt.Errorf("duplicate delivery candidate preview URN %q", member.URN)
		}
		urns[member.URN] = true
	}
	for _, stack := range manifest.UnpreviewableStacks {
		if stack.URN == "" || stack.Name == "" || stack.Stage == "" || stack.TargetStack == "" {
			return manifest, errors.New("delivery candidate preview unavailable stack is incomplete")
		}
		if stack.Status != "not-previewable" {
			return manifest, fmt.Errorf("delivery candidate preview unavailable stack %q has invalid status %q",
				stack.URN, stack.Status)
		}
		if stack.Reason == "" {
			return manifest, fmt.Errorf("delivery candidate preview unavailable stack %q is missing a reason", stack.URN)
		}
		if urns[stack.URN] {
			return manifest, fmt.Errorf("duplicate delivery candidate preview URN %q", stack.URN)
		}
		urns[stack.URN] = true
	}
	return manifest, nil
}

// unavailableStackRows converts manifest dispositions into the client wire shape: status and
// reason carried through, Changes present but empty (unavailable is not "no changes"), and no
// environment revisions since the member never ran.
func unavailableStackRows(stacks []candidatePreviewUnavailableStack) []client.DeliveryCandidatePreviewStack {
	rows := make([]client.DeliveryCandidatePreviewStack, len(stacks))
	for i, stack := range stacks {
		rows[i] = client.DeliveryCandidatePreviewStack{
			URN: stack.URN, Name: stack.Name, Stage: stack.Stage,
			TargetStack: stack.TargetStack, Status: stack.Status, Reason: stack.Reason, Changes: map[string]int{},
		}
	}
	return rows
}

// coherenceAPI is the subgroup surface of the service client.
type coherenceAPI interface {
	CreateCoherenceWindowSubgroup(ctx context.Context, org string, windowID string, size int) (string, error)
	CloseCoherenceWindowSubgroup(ctx context.Context, org string, windowID string, subgroupID string) error
}

// executable returns the CLI binary that runs each member preview; tests replace it.
var executable = os.Executable

// memberRun is a manifest member resolved for execution.
type memberRun struct {
	member       candidatePreviewMember
	directory    string
	config       map[string]candidatePreviewConfigValue
	env          map[string]string
	environments []string
	preRun       []string
}

type memberResult struct {
	changes map[string]int
	plan    *deploy.Plan
	events  []apitype.EngineEvent
}

func runCandidatePreview(
	ctx context.Context, manifest candidatePreviewManifest, api coherenceAPI,
) ([]client.DeliveryCandidatePreviewStack, json.RawMessage, json.RawMessage, error) {
	if len(manifest.Members) == 0 {
		// Nothing runnable: skip filesystem resolution, pre-run commands and subprocesses
		// entirely. No plan and no summary -- never fabricate a zero-change result for stacks
		// that were never previewed.
		events, err := json.Marshal([]any{})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("encoding empty native multistack preview events: %w", err)
		}
		return unavailableStackRows(manifest.UnpreviewableStacks), nil, events, nil
	}
	workspace, err := filepath.EvalSymlinks(manifest.Workspace)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolving release workspace: %w", err)
	}
	runs := make([]memberRun, len(manifest.Members))
	secretValues := []string{}
	for i, member := range manifest.Members {
		directory := member.Directory
		if !filepath.IsAbs(directory) {
			directory = filepath.Join(manifest.Workspace, directory)
		}
		directory, err = filepath.EvalSymlinks(directory)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("resolving member %s directory: %w", member.Name, err)
		}
		relative, err := filepath.Rel(workspace, directory)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, nil, nil, fmt.Errorf("member %s directory is outside the release workspace", member.Name)
		}
		var environment candidatePreviewEnvironment
		if len(member.Environment) > 0 {
			if err = json.Unmarshal(member.Environment, &environment); err != nil {
				return nil, nil, nil, fmt.Errorf("decoding member %s environment: %w", member.Name, err)
			}
		}
		env := make(map[string]string, len(environment.Env))
		for key, value := range environment.Env {
			env[key] = value.Value
			if value.Secret {
				secretValues = append(secretValues, value.Value)
			}
		}
		for _, value := range member.Config {
			if value.Secret {
				secretValues = append(secretValues, value.Value)
			}
		}
		runs[i] = memberRun{
			member: member, directory: directory, config: member.Config, env: env,
			environments: environment.Environments, preRun: environment.PreRunCommands,
		}
	}
	cli, err := executable()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("locating the pulumi CLI binary: %w", err)
	}
	tmp, err := os.MkdirTemp("", "delivery-preview-")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating preview scratch directory: %w", err)
	}
	defer os.RemoveAll(tmp)

	results := make([]memberResult, len(runs))
	for _, group := range previewGroups(runs) {
		if err := previewGroup(ctx, api, manifest.Organization, cli, tmp, runs, group, secretValues, results); err != nil {
			return nil, nil, nil, err
		}
	}

	stacks := make([]client.DeliveryCandidatePreviewStack, len(results))
	plans := make([]*deploy.Plan, len(results))
	names := make([]string, len(results))
	for i, result := range results {
		member := manifest.Members[i]
		stacks[i] = client.DeliveryCandidatePreviewStack{
			URN: member.URN, Name: member.Name, Stage: member.Stage,
			TargetStack: member.TargetStack, Status: "succeeded", Changes: result.changes,
		}
		plans[i], names[i] = result.plan, member.Name
	}
	// Members in manifest order, then unavailable rows in manifest order.
	stacks = append(stacks, unavailableStackRows(manifest.UnpreviewableStacks)...)
	aggregatedPlan, err := aggregateNativePreviewPlan(plans, names)
	if err != nil {
		return nil, nil, nil, err
	}
	serialized, err := resourceStack.SerializePlan(aggregatedPlan, config.BlindingCrypter, false)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("serializing native multistack preview plan: %w", err)
	}
	plan, err := json.Marshal(serialized)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encoding native multistack preview plan: %w", err)
	}
	plan = redactJSONSecrets(plan, secretValues)
	perMember := make([][]apitype.EngineEvent, len(results))
	for i, result := range results {
		perMember[i] = result.events
	}
	events, err := json.Marshal(aggregateNativePreviewEvents(perMember))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encoding native multistack preview events: %w", err)
	}
	events = redactJSONSecrets(events, secretValues)
	return stacks, plan, events, nil
}

// previewGroups splits members into groups, ordered by ascending GroupOrder. Groups with equal
// order keep their first-appearance order in the manifest.
func previewGroups(runs []memberRun) [][]int {
	type key struct {
		order int
		group string
	}
	keys := []key{}
	members := map[key][]int{}
	for i, run := range runs {
		k := key{run.member.GroupOrder, run.member.Group}
		if _, seen := members[k]; !seen {
			keys = append(keys, k)
		}
		members[k] = append(members[k], i)
	}
	slices.SortStableFunc(keys, func(a key, b key) int { return a.order - b.order })
	groups := make([][]int, len(keys))
	for i, k := range keys {
		groups[i] = members[k]
	}
	return groups
}

// previewGroup previews every member of a group concurrently, each as its own stock-CLI preview,
// inside one fresh coherence window subgroup sized to the group. Member preparation (pre-run
// commands, config, ESC environments) happens before the subgroup exists so that a preparation
// failure never leaves a subgroup waiting for a member that will not join. The subgroup is closed
// when the group finishes and as soon as any member fails, which releases readers still waiting
// for a member that may never join.
func previewGroup(
	ctx context.Context, api coherenceAPI, org string, cli string, tmp string, runs []memberRun,
	group []int, secretValues []string, results []memberResult,
) error {
	for _, index := range group {
		if err := prepareMember(ctx, cli, runs[index]); err != nil {
			return err
		}
	}
	windowID := uuid.NewString()
	subgroupID, err := api.CreateCoherenceWindowSubgroup(ctx, org, windowID, len(group))
	if err != nil {
		return fmt.Errorf("creating coherence window subgroup: %w", err)
	}
	var closeOnce sync.Once
	closeSubgroup := func() {
		closeOnce.Do(func() {
			// The group's context may already be cancelled; the subgroup must still close.
			closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			_ = api.CloseCoherenceWindowSubgroup(closeCtx, org, windowID, subgroupID)
		})
	}
	defer closeSubgroup()

	errs := make([]error, len(group))
	var wg sync.WaitGroup
	for n, index := range group {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := previewMember(ctx, cli, tmp, index, runs[index], subgroupID, secretValues)
			if err != nil {
				errs[n] = err
				closeSubgroup()
				return
			}
			results[index] = result
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}

// prepareMember runs the member's pre-run commands, applies its config and imports its ESC
// environments. Config values go on the command line, never to logs. The stock CLI has no
// per-invocation environment override, so environments are appended to the stack's imports
// (`config env add`) in the release checkout.
func prepareMember(ctx context.Context, cli string, run memberRun) error {
	member := run.member
	environ := mergedEnvironment(os.Environ(), run.env)
	for _, command := range run.preRun {
		process := exec.CommandContext(ctx, "sh", "-c", command)
		process.Dir, process.Env = run.directory, environ
		if err := process.Run(); err != nil {
			return fmt.Errorf("running pre-command for member %s: %w", member.Name, err)
		}
	}
	base := []string{"--non-interactive", "--stack", member.TargetStack, "--cwd", run.directory}
	if len(run.environments) > 0 {
		args := append([]string{"config", "env", "add"}, run.environments...)
		args = append(args, "--yes")
		if err := runQuiet(ctx, cli, append(args, base...), run.directory, environ); err != nil {
			return fmt.Errorf("adding environments for member %s: %w", member.Name, err)
		}
	}
	keys := make([]string, 0, len(run.config))
	for key := range run.config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args := []string{"config", "set"}
		if run.config[key].Secret {
			args = append(args, "--secret")
		}
		args = append(args, base...)
		args = append(args, "--", key, run.config[key].Value)
		if err := runQuiet(ctx, cli, args, run.directory, environ); err != nil {
			return fmt.Errorf("setting config key %q for member %s: %w", key, member.Name, err)
		}
	}
	return nil
}

// runQuiet discards the command's output: it can echo configuration values.
func runQuiet(ctx context.Context, cli string, args []string, dir string, environ []string) error {
	process := exec.CommandContext(ctx, cli, args...)
	process.Dir, process.Env = dir, environ
	return process.Run()
}

func previewMember(
	ctx context.Context, cli string, tmp string, index int, run memberRun, subgroupID string,
	secretValues []string,
) (memberResult, error) {
	member := run.member
	planPath := filepath.Join(tmp, fmt.Sprintf("plan-%d.json", index))
	eventsPath := filepath.Join(tmp, fmt.Sprintf("events-%d.json", index))
	process := exec.CommandContext(ctx, cli, "preview", "--non-interactive",
		"--coherence-window", subgroupID, "--stack", member.TargetStack, "--cwd", run.directory,
		"--save-plan", planPath, "--event-log", eventsPath, "--json")
	var stdout, stderr bytes.Buffer
	process.Dir, process.Stdout, process.Stderr = run.directory, &stdout, &stderr
	// --event-log is registered only when debug commands are enabled.
	process.Env = append(mergedEnvironment(os.Environ(), run.env), "PULUMI_DEBUG_COMMANDS=true")
	if err := process.Run(); err != nil {
		tail := stderr.Bytes()
		if len(tail) > 4096 {
			tail = tail[len(tail)-4096:]
		}
		tail = redactJSONSecrets(bytes.TrimSpace(tail), secretValues)
		return memberResult{}, fmt.Errorf("previewing member %s (%s): %w: %s",
			member.Name, member.TargetStack, err, tail)
	}
	var digest struct {
		ChangeSummary map[string]int `json:"changeSummary"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &digest); err != nil {
		return memberResult{}, fmt.Errorf("decoding preview output for %s: %w", member.TargetStack, err)
	}
	if digest.ChangeSummary == nil {
		digest.ChangeSummary = map[string]int{}
	}
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		return memberResult{}, fmt.Errorf("native preview for %s returned no plan: %w", member.TargetStack, err)
	}
	var serialized apitype.DeploymentPlanV1
	if err = json.Unmarshal(planBytes, &serialized); err != nil {
		return memberResult{}, fmt.Errorf("decoding plan for %s: %w", member.TargetStack, err)
	}
	// Secrets stay ciphertext here: the aggregate re-serializes them blinded.
	plan, err := resourceStack.DeserializePlan(serialized, config.NopDecrypter)
	if err != nil {
		return memberResult{}, fmt.Errorf("reading plan for %s: %w", member.TargetStack, err)
	}
	events, err := readEngineEvents(eventsPath)
	if err != nil {
		return memberResult{}, fmt.Errorf("reading events for %s: %w", member.TargetStack, err)
	}
	return memberResult{changes: digest.ChangeSummary, plan: plan, events: events}, nil
}

func readEngineEvents(path string) ([]apitype.EngineEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	events := []apitype.EngineEvent{}
	decoder := json.NewDecoder(file)
	for decoder.More() {
		var event apitype.EngineEvent
		if err := decoder.Decode(&event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

// aggregateNativePreviewPlan combines every member's own native preview plan into one
// display-only union of ResourcePlans keyed by URN, for the Delivery approval preview artifact.
// Each member previews as its own stack, so its ResourcePlans already carry that member's own
// project/stack in every URN; a URN collision here would mean two members produced the exact
// same resource, which is rejected rather than silently letting one overwrite the other.
// Plan.Config is deliberately left empty rather than merged: two members can use the same
// config key with different values, and this aggregate is not a replayable per-stack plan.
func aggregateNativePreviewPlan(plans []*deploy.Plan, names []string) (*deploy.Plan, error) {
	merged := &deploy.Plan{ResourcePlans: map[resource.URN]*deploy.ResourcePlan{}}
	owner := map[resource.URN]string{}
	for i, plan := range plans {
		if plan == nil {
			continue
		}
		if merged.Manifest.Version == "" {
			merged.Manifest = plan.Manifest
		}
		for urn, rp := range plan.ResourcePlans {
			if existing, exists := owner[urn]; exists {
				return nil, fmt.Errorf(
					"duplicate resource URN %q across delivery members %q and %q",
					urn, existing, names[i])
			}
			owner[urn] = names[i]
			merged.ResourcePlans[urn] = rp
		}
	}
	return merged, nil
}

// aggregateNativePreviewEvents combines every member's preview event stream into one ordered
// stream for the Delivery approval preview artifact: each member's own diffs and diagnostics are
// preserved in full, and the per-member summary events collapse into a single summary covering
// every member's changes -- the shape the console's approval preview expects. Sequence numbers
// are reassigned across the combined stream.
func aggregateNativePreviewEvents(perMember [][]apitype.EngineEvent) []apitype.EngineEvent {
	changes := map[apitype.OpType]int{}
	combined := []apitype.EngineEvent{}
	for _, events := range perMember {
		for _, event := range events {
			if event.SummaryEvent != nil {
				for op, count := range event.SummaryEvent.ResourceChanges {
					changes[op] += count
				}
				continue
			}
			combined = append(combined, event)
		}
	}
	combined = append(combined, apitype.EngineEvent{
		Timestamp: int(time.Now().Unix()),
		SummaryEvent: &apitype.SummaryEvent{
			ResourceChanges: changes, PolicyPacks: map[string]string{}, IsPreview: true,
		},
	})
	for i := range combined {
		combined[i].Sequence = i + 1
	}
	return combined
}

// redactJSONSecrets is a final defense for diagnostics and provider messages. Config secrets are
// already represented as secret values in the plan, but arbitrary programs can print an ESC or
// environment secret into a diagnostic string where the engine has no property metadata to mark it.
func redactJSONSecrets(value json.RawMessage, secrets []string) json.RawMessage {
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		encoded, err := json.Marshal(secret)
		if err != nil || len(encoded) < 2 {
			continue
		}
		value = bytes.ReplaceAll(value, encoded[1:len(encoded)-1], []byte("[secret]"))
	}
	return value
}

func mergedEnvironment(base []string, values map[string]string) []string {
	merged := make(map[string]string, len(base)+len(values))
	for _, entry := range base {
		key, value, found := strings.Cut(entry, "=")
		if found {
			merged[key] = value
		}
	}
	for key, value := range values {
		merged[key] = value
	}
	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, len(keys))
	for index, key := range keys {
		result[index] = key + "=" + merged[key]
	}
	return result
}
