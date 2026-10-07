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

package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"reflect"
	"slices"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// StackEnvironmentMode selects what a command does with an inline environment definition in the
// stack's configuration file, beyond opening it to read configuration.
type StackEnvironmentMode int

const (
	// StackEnvironmentOpenOnly opens the definition anonymously and nothing more. Commands that only
	// read configuration use it.
	StackEnvironmentOpenOnly StackEnvironmentMode = iota
	// StackEnvironmentPreview also compares the definition with the environment the stack manages and
	// prints what `pulumi up` would publish.
	StackEnvironmentPreview
	// StackEnvironmentSync is StackEnvironmentPreview plus a SyncEnvironment hook on the returned
	// configuration that publishes the definition once the operation is confirmed and re-reads the
	// configuration from the published revision.
	StackEnvironmentSync
)

// StackConfigurationOptions tunes GetStackConfigurationWithOptions.
type StackConfigurationOptions struct {
	// EnvironmentMode selects what to do with an inline environment definition.
	EnvironmentMode StackEnvironmentMode
	// Stdout receives the environment summary. Nil means os.Stdout.
	Stdout io.Writer
}

func (opts StackConfigurationOptions) stdout() io.Writer {
	if opts.Stdout != nil {
		return opts.Stdout
	}
	// This is a non-command helper with no *cobra.Command writer to thread through, so like the rest
	// of this package it falls back to the process stream.
	return os.Stdout //nolint:forbidigo
}

// stackEnvironmentOpenDuration is how long the environment published by `pulumi up` stays open: long
// enough for any update, mirroring the duration anonymous opens use.
const stackEnvironmentOpenDuration = 2 * time.Hour

// attachStackEnvironment compares the stack's inline environment definition with the environment the
// stack manages, prints the pending changes, and, in StackEnvironmentSync mode, arms cfg.SyncEnvironment
// so the backend publishes the definition once the operation is confirmed.
func attachStackEnvironment(
	ctx context.Context,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
	sm secrets.Manager,
	envOverrides []string,
	preview *backend.StackEnvironmentSync,
	opts StackConfigurationOptions,
	cfg *backend.StackConfiguration,
) error {
	if opts.EnvironmentMode == StackEnvironmentOpenOnly || !workspaceStack.Environment.IsDefinition() {
		return nil
	}
	syncer, ok := stack.Backend().(backend.StackEnvironmentsBackend)
	if !ok {
		return nil
	}

	definition := workspaceStack.EnvironmentBytes()
	if preview == nil {
		// The open did not go through the sync (for instance because of --override-env), so compare now.
		var err error
		preview, err = syncer.SyncStackEnvironment(ctx, stack, definition, backend.StackEnvironmentSyncOptions{DryRun: true})
		if errors.Is(err, backend.ErrStackEnvironmentSyncUnsupported) {
			slog.Debug("backend does not support stack-managed environments; opening the inline definition only")
			return nil
		}
		if err != nil {
			return fmt.Errorf("comparing the environment definition with the stack's environment: %w", err)
		}
	}

	stdout := opts.stdout()
	printStackEnvironmentPreview(stdout, preview, definition)

	if opts.EnvironmentMode != StackEnvironmentSync {
		return nil
	}
	if len(envOverrides) != 0 {
		// Overrides are for this run only and must not be published, yet the configuration in use has to
		// reflect them; so this run keeps the anonymous open and leaves the environment alone.
		fmt.Fprintf(stdout, "warning: --override-env is set, so environment %s is not synchronized by this operation\n\n",
			preview.Environment)
		return nil
	}

	expectedRevision := preview.Revision
	cfg.SyncEnvironment = func(ctx context.Context) (backend.StackConfiguration, error) {
		return syncStackEnvironment(ctx, stdout, syncer, stack, workspaceStack, sm, definition, expectedRevision)
	}
	return nil
}

// syncStackEnvironment publishes definition and returns the configuration read from the published revision.
func syncStackEnvironment(
	ctx context.Context,
	stdout io.Writer,
	syncer backend.StackEnvironmentsBackend,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
	sm secrets.Manager,
	definition []byte,
	expectedRevision int,
) (backend.StackConfiguration, error) {
	res, err := syncer.SyncStackEnvironment(ctx, stack, definition, backend.StackEnvironmentSyncOptions{
		ExpectedRevision: &expectedRevision,
		Duration:         stackEnvironmentOpenDuration,
	})
	if err != nil {
		return backend.StackConfiguration{}, fmt.Errorf("synchronizing the stack's environment: %w", err)
	}
	if len(res.Diagnostics) != 0 {
		printESCDiagnostics(os.Stderr, res.Diagnostics) //nolint:forbidigo
		return backend.StackConfiguration{}, errors.New("synchronizing the stack's environment: too many errors")
	}
	if res.Opened == nil {
		return backend.StackConfiguration{}, fmt.Errorf(
			"synchronizing the stack's environment: the backend did not open %s@%d", res.Environment, res.Revision)
	}

	switch {
	case res.Created:
		fmt.Fprintf(stdout, "Created environment %s from the stack's definition (revision %d)\n",
			res.Environment, res.Revision)
	case res.Changed:
		fmt.Fprintf(stdout, "Published the stack's definition to environment %s (revision %d -> %d)\n",
			res.Environment, res.PreviousRevision, res.Revision)
	default:
		fmt.Fprintf(stdout, "Environment %s is up to date (revision %d)\n", res.Environment, res.Revision)
	}

	cfg, err := stackConfigurationFromEnvironment(res.Opened, workspaceStack, sm)
	if err != nil {
		return backend.StackConfiguration{}, err
	}
	cfg.StackEnvironment = &backend.StackEnvironmentRef{
		Name:          res.Environment,
		Revision:      res.Revision,
		OpenSessionID: res.OpenSessionID,
	}
	return cfg, nil
}

// printStackEnvironmentPreview prints the changes `pulumi up` would publish to the stack's environment.
// Nothing is printed when the published definition already matches.
func printStackEnvironmentPreview(w io.Writer, preview *backend.StackEnvironmentSync, definition []byte) {
	if !preview.Created && !preview.Changed {
		return
	}

	var current any
	if len(preview.CurrentDefinition) != 0 {
		if err := yaml.Unmarshal(preview.CurrentDefinition, &current); err != nil {
			current = nil
		}
	}
	var proposed any
	if err := yaml.Unmarshal(definition, &proposed); err != nil {
		proposed = nil
	}

	if preview.Created {
		fmt.Fprintf(w, "Environment %s will be created from the stack's definition:\n", preview.Environment)
	} else {
		fmt.Fprintf(w, "Environment %s will be updated (revision %d):\n", preview.Environment, preview.Revision)
	}
	for _, line := range diffEnvironmentDefinitions(current, proposed) {
		fmt.Fprintf(w, "    %s\n", line)
	}
	fmt.Fprintln(w)
}

// diffEnvironmentDefinitions lists the differences between two decoded environment definitions, one
// per line, as "+ path: value", "- path: value" or "~ path: old -> new". Secrets are redacted.
func diffEnvironmentDefinitions(current any, proposed any) []string {
	var lines []string
	diffDefinitionValues("", redactDefinitionSecrets(current), redactDefinitionSecrets(proposed), &lines)
	return lines
}

func diffDefinitionValues(path string, old any, new any, lines *[]string) {
	if reflect.DeepEqual(old, new) {
		return
	}

	oldMap, oldIsMap := old.(map[string]any)
	newMap, newIsMap := new.(map[string]any)
	// A mapping that appears or disappears is listed entry by entry, so a brand-new definition reads
	// like an edit of an empty one.
	if oldIsMap && new == nil {
		newMap, newIsMap = map[string]any{}, true
	}
	if newIsMap && old == nil {
		oldMap, oldIsMap = map[string]any{}, true
	}
	if oldIsMap && newIsMap {
		keys := make([]string, 0, len(oldMap)+len(newMap))
		for k := range oldMap {
			keys = append(keys, k)
		}
		for k := range newMap {
			if _, ok := oldMap[k]; !ok {
				keys = append(keys, k)
			}
		}
		slices.Sort(keys)
		for _, k := range keys {
			childPath := joinDefinitionPath(path, k)
			oldValue, inOld := oldMap[k]
			newValue, inNew := newMap[k]
			switch {
			case !inOld:
				*lines = append(*lines, fmt.Sprintf("+ %s: %s", childPath, renderDefinitionValue(newValue)))
			case !inNew:
				*lines = append(*lines, fmt.Sprintf("- %s: %s", childPath, renderDefinitionValue(oldValue)))
			default:
				diffDefinitionValues(childPath, oldValue, newValue, lines)
			}
		}
		return
	}

	oldSeq, oldIsSeq := old.([]any)
	newSeq, newIsSeq := new.([]any)
	if oldIsSeq && newIsSeq {
		for i := 0; i < max(len(oldSeq), len(newSeq)); i++ {
			childPath := path + "[" + strconv.Itoa(i) + "]"
			switch {
			case i >= len(oldSeq):
				*lines = append(*lines, fmt.Sprintf("+ %s: %s", childPath, renderDefinitionValue(newSeq[i])))
			case i >= len(newSeq):
				*lines = append(*lines, fmt.Sprintf("- %s: %s", childPath, renderDefinitionValue(oldSeq[i])))
			default:
				diffDefinitionValues(childPath, oldSeq[i], newSeq[i], lines)
			}
		}
		return
	}

	switch {
	case old == nil:
		*lines = append(*lines, fmt.Sprintf("+ %s: %s", path, renderDefinitionValue(new)))
	case new == nil:
		*lines = append(*lines, fmt.Sprintf("- %s: %s", path, renderDefinitionValue(old)))
	default:
		*lines = append(*lines, fmt.Sprintf("~ %s: %s -> %s", path, renderDefinitionValue(old), renderDefinitionValue(new)))
	}
}

func joinDefinitionPath(path string, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// renderDefinitionValue renders a scalar as is and a composite value as compact JSON.
func renderDefinitionValue(v any) string {
	switch v := v.(type) {
	case nil:
		return "null"
	case string:
		return v
	case map[string]any, []any:
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
	}
	return fmt.Sprint(v)
}

// redactDefinitionSecrets replaces every `fn::secret` call with "[secret]" so neither plaintext nor
// ciphertext reaches the terminal.
func redactDefinitionSecrets(v any) any {
	switch v := v.(type) {
	case map[string]any:
		if _, ok := v["fn::secret"]; ok {
			return "[secret]"
		}
		out := make(map[string]any, len(v))
		for k, child := range v {
			out[k] = redactDefinitionSecrets(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = redactDefinitionSecrets(child)
		}
		return out
	default:
		return v
	}
}
