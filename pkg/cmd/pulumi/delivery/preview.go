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
	resourceStack "github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

// candidatePreviewManifest is written by the trusted delivery runner. Source checkout happens
// before this command starts; directories are relative to that immutable release workspace.
type candidatePreviewManifest struct {
	CandidateID     string                   `json:"candidateId"`
	ShapeVersion    int                      `json:"shapeVersion"`
	ReleaseID       string                   `json:"releaseId"`
	ChangeRequestID string                   `json:"changeRequestId"`
	RevisionNumber  int                      `json:"revisionNumber"`
	WorkflowRunID   string                   `json:"workflowRunId"`
	Workspace       string                   `json:"workspace"`
	Members         []candidatePreviewMember `json:"members"`
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
			request := client.DeliveryCandidatePreviewRequest{
				ShapeVersion: manifest.ShapeVersion, ReleaseID: manifest.ReleaseID,
				ChangeRequestID: manifest.ChangeRequestID, RevisionNumber: manifest.RevisionNumber,
				WorkflowRunID: manifest.WorkflowRunID,
			}
			stacks, plan, events, previewErr := runCandidatePreview(cmd.Context(), manifest)
			if previewErr != nil {
				request.Status, request.Error = "failed", previewErr.Error()
			} else {
				request.Status, request.Stacks, request.Plan, request.Events = "succeeded", stacks, plan, events
			}
			if callbackErr := api.CompleteDeliveryCandidatePreview(cmd.Context(), stack.Owner,
				manifest.CandidateID, request); callbackErr != nil {
				return fmt.Errorf("reporting delivery candidate preview: %w", callbackErr)
			}
			return previewErr
		},
	}
	cmd.Flags().StringVar(&manifestPath, "manifest", manifestPath, "Path to the delivery candidate preview manifest")
	return cmd
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
		manifest.ChangeRequestID == "" || manifest.RevisionNumber < 0 || manifest.WorkflowRunID == "" ||
		manifest.Workspace == "" || len(manifest.Members) == 0 {
		return manifest, errors.New("delivery candidate preview manifest is incomplete")
	}
	for _, member := range manifest.Members {
		if member.URN == "" || member.TargetStack == "" || member.Directory == "" {
			return manifest, errors.New("delivery candidate preview member is incomplete")
		}
	}
	return manifest, nil
}

func runCandidatePreview(ctx context.Context, manifest candidatePreviewManifest) (
	[]client.DeliveryCandidatePreviewStack, json.RawMessage, json.RawMessage, error,
) {
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
		specs[i] = auto.Options{WorkDir: directory, Stack: member.TargetStack, Config: plain,
			SecretConfig: secret, Environments: environment.Environments, EnvironmentVariables: env}
	}
	results, err := auto.PreviewMany(ctx, specs)
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
		stacks[i] = client.DeliveryCandidatePreviewStack{URN: member.URN, Name: member.Name, Stage: member.Stage,
			TargetStack: member.TargetStack, Changes: changes,
			EnvironmentRevisions: environmentRevisions(result.EnvironmentImports)}
	}
	serialized, err := resourceStack.SerializePlan(results[0].Plan, config.BlindingCrypter, false)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("serializing native multistack preview plan: %w", err)
	}
	plan, err := json.Marshal(serialized)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encoding native multistack preview plan: %w", err)
	}
	plan = redactJSONSecrets(plan, secretValues)
	nativeEvents := make([]any, 0, len(results[0].Events))
	for _, event := range results[0].Events {
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
