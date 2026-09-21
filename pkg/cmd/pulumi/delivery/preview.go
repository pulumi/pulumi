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
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/auto"
	backendDisplay "github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	resourceStack "github.com/pulumi/pulumi/pkg/v3/resource/stack"
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
			stacks, plan, events, previewErr := runCandidatePreview(cmd.Context(), manifest)
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
// never got far enough to produce them via runCandidatePreview; PreviewMany discards all results
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

// previewManyFunc is a seam over auto.PreviewMany so tests can exercise the members-present path
// without a real Pulumi workspace.
var previewManyFunc = auto.PreviewMany

func runCandidatePreview(ctx context.Context, manifest candidatePreviewManifest) (
	[]client.DeliveryCandidatePreviewStack, json.RawMessage, json.RawMessage, error,
) {
	if len(manifest.Members) == 0 {
		// Nothing runnable: skip filesystem resolution, pre-run commands and PreviewMany
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
	specs := make([]auto.Options, len(manifest.Members))
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
		plain, secret := map[string]string{}, map[string]string{}
		for key, value := range member.Config {
			if value.Secret {
				secret[key] = value.Value
				secretValues = append(secretValues, value.Value)
			} else {
				plain[key] = value.Value
			}
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
		for _, command := range environment.PreRunCommands {
			process := exec.CommandContext(ctx, "sh", "-c", command)
			process.Dir = directory
			process.Env = mergedEnvironment(os.Environ(), env)
			if runErr := process.Run(); runErr != nil {
				return nil, nil, nil, fmt.Errorf("running pre-command for member %s: %w", member.Name, runErr)
			}
		}
		specs[i] = auto.Options{
			WorkDir: directory, Stack: member.TargetStack, Config: plain,
			SecretConfig: secret, Environments: environment.Environments, EnvironmentVariables: env,
		}
	}
	results, err := previewManyFunc(ctx, specs)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(results) != len(manifest.Members) || len(results) == 0 {
		return nil, nil, nil, errors.New("native multistack preview returned an incomplete result set")
	}
	stacks := make([]client.DeliveryCandidatePreviewStack, len(results))
	for i, result := range results {
		if result.Plan == nil {
			return nil, nil, nil, fmt.Errorf("native preview for %s returned no plan", manifest.Members[i].TargetStack)
		}
		changes := make(map[string]int, len(result.Changes))
		for operation, count := range result.Changes {
			changes[string(operation)] = count
		}
		member := manifest.Members[i]
		stacks[i] = client.DeliveryCandidatePreviewStack{
			URN: member.URN, Name: member.Name, Stage: member.Stage,
			TargetStack: member.TargetStack, Status: "succeeded", Changes: changes,
			EnvironmentRevisions: environmentRevisions(result.EnvironmentImports),
		}
	}
	// Members in manifest order, then unavailable rows in manifest order.
	stacks = append(stacks, unavailableStackRows(manifest.UnpreviewableStacks)...)
	aggregatedPlan, err := aggregateNativePreviewPlan(results, manifest.Members)
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
	aggregatedEvents := aggregateNativePreviewEvents(results)
	nativeEvents := make([]any, 0, len(aggregatedEvents))
	for _, event := range aggregatedEvents {
		converted, err := backendDisplay.ConvertEngineEvent(event, false)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("encoding native multistack preview event: %w", err)
		}
		nativeEvents = append(nativeEvents, converted)
	}
	events, err := json.Marshal(nativeEvents)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encoding native multistack preview events: %w", err)
	}
	events = redactJSONSecrets(events, secretValues)
	return stacks, plan, events, nil
}

// aggregateNativePreviewPlan combines every member's own native preview plan into one
// display-only union of ResourcePlans keyed by URN, for the Delivery approval preview artifact.
// Each member now runs as its own real Deployment (see backend.MultistackPreview), so its
// ResourcePlans already carry that member's own project/stack in every URN; a URN collision
// here would mean two members produced the exact same resource, which is rejected rather than
// silently letting one overwrite the other. Plan.Config is deliberately left empty rather than
// merged: two members can use the same config key with different values, and this aggregate is
// not a replayable per-stack deployment plan.
func aggregateNativePreviewPlan(
	results []auto.Result, members []candidatePreviewMember,
) (*deploy.Plan, error) {
	merged := &deploy.Plan{ResourcePlans: map[resource.URN]*deploy.ResourcePlan{}}
	owner := map[resource.URN]string{}
	for i, result := range results {
		if result.Plan == nil {
			continue
		}
		if merged.Manifest.Version == "" {
			merged.Manifest = result.Plan.Manifest
		}
		for urn, rp := range result.Plan.ResourcePlans {
			if existing, exists := owner[urn]; exists {
				return nil, fmt.Errorf(
					"duplicate resource URN %q across delivery members %q and %q",
					urn, existing, members[i].Name)
			}
			owner[urn] = members[i].Name
			merged.ResourcePlans[urn] = rp
		}
	}
	return merged, nil
}

// aggregateNativePreviewEvents combines every member's native preview event stream into one
// ordered stream for the Delivery approval preview artifact: each member's own diffs and
// diagnostics are preserved in full, and the per-member summary events (one native SummaryEvent
// per member, each scoped to that member's own changes) collapse into a single coherent summary
// covering every member's changes -- the shape the console's approval preview expects.
func aggregateNativePreviewEvents(results []auto.Result) []engine.Event {
	changes := display.ResourceChanges{}
	combined := make([]engine.Event, 0)
	for _, result := range results {
		for _, event := range result.Events {
			if event.Type == engine.SummaryEvent {
				if payload, ok := event.Payload().(engine.SummaryEventPayload); ok {
					for op, count := range payload.ResourceChanges {
						changes[op] += count
					}
				}
				continue
			}
			combined = append(combined, event)
		}
	}
	return append(combined, engine.NewEvent(engine.SummaryEventPayload{
		IsPreview:       true,
		ResourceChanges: changes,
	}))
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

func environmentRevisions(imports []string) map[string]string {
	if len(imports) == 0 {
		return nil
	}
	result := make(map[string]string, len(imports))
	for _, ref := range imports {
		name, _, _ := strings.Cut(ref, "@")
		result[name] = ref
	}
	return result
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
