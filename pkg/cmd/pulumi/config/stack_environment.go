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
	"maps"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
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
	project *workspace.Project,
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
	err := printStackConfigPreview(ctx, stdout, stack, project, workspaceStack, cfg.Environment, preview.Environment, sm)
	if err != nil {
		// The resolved-config diff is advisory: the operation itself reports anything that is wrong.
		slog.Debug("skipping the configuration diff", "err", err)
	}

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

// printStackConfigPreview prints how the configuration this run resolves to, from the stack file and
// its opened environment, differs from the configuration the stack's last update ran with. Nothing is
// printed when they match. Secrets are compared decrypted but always rendered as "[secret]". Each
// added or changed value names where it comes from unless the stack's own definition sets it.
func printStackConfigPreview(
	ctx context.Context,
	w io.Writer,
	stack backend.Stack,
	project *workspace.Project,
	workspaceStack *workspace.ProjectStack,
	env esc.Value,
	environmentName string,
	sm secrets.Manager,
) error {
	if project == nil || sm == nil {
		return nil
	}

	// Merge the environment into the stack config exactly the way `up` does before it runs.
	proposed := maps.Clone(workspaceStack.Config)
	if proposed == nil {
		proposed = config.Map{}
	}
	stackName := stack.Ref().Name().String()
	err := pkgWorkspace.ApplyProjectConfig(ctx, stackName, project, env, proposed, sm.Encrypter(), sm.Decrypter())
	if err != nil {
		return fmt.Errorf("resolving the proposed configuration: %w", err)
	}

	var previous config.Map
	latest, err := backend.GetLatestConfiguration(ctx, stack)
	switch {
	case errors.Is(err, backenderr.ErrNoPreviousDeployment):
		previous = config.Map{}
	case err != nil:
		return fmt.Errorf("reading the last update's configuration: %w", err)
	default:
		previous = latest.Config
	}

	decrypter := sm.Decrypter()
	previousValues, err := previous.Decrypt(decrypter)
	if err != nil {
		return fmt.Errorf("decrypting the last update's configuration: %w", err)
	}
	proposedValues, err := proposed.Decrypt(decrypter)
	if err != nil {
		return fmt.Errorf("decrypting the proposed configuration: %w", err)
	}

	sources := configValueSources(env, environmentName, workspaceStack.Config)
	lines := diffStackConfig(previous, previousValues, proposed, proposedValues, sources)
	if len(lines) == 0 {
		return nil
	}
	if len(previous) == 0 {
		fmt.Fprintln(w, "Configuration for this run (the stack has no previous update):")
	} else {
		fmt.Fprintln(w, "Configuration changes since the stack's last update:")
	}
	for _, line := range lines {
		fmt.Fprintf(w, "    %s\n", line)
	}
	fmt.Fprintln(w)
	return nil
}

// configValueSources reports, for each configuration key the proposed run resolves, where its value
// comes from: "" when the stack's own environment definition sets it, the importing environment's
// name when an import does, or the stack configuration file when its `config` block overrides it.
func configValueSources(env esc.Value, environmentName string, stackConfig config.Map) map[string]string {
	sources := map[string]string{}
	if entries, ok := env.Value.(map[string]esc.Value); ok {
		for key, value := range entries {
			definedIn := value.Trace.Def.Environment
			switch definedIn {
			case "", environmentName, esc.AnonymousEnvironmentName:
				// Defined by the stack's own definition, or the evaluator kept no trace.
			default:
				sources[key] = "from import " + definedIn
			}
		}
	}
	for key := range stackConfig {
		// Values in the config block shadow the environment's during merging.
		sources[key.String()] = "from the stack configuration file"
	}
	return sources
}

// diffStackConfig lists the differences between two decrypted configuration maps, one per line, as
// "+ key: value", "- key: value" or "~ key: old -> new". A value that is secret on either side is
// rendered as "[secret]"; the comparison itself uses the decrypted values. Added and changed lines
// end with the value's source from sources, when it has one.
func diffStackConfig(
	previous config.Map,
	previousValues map[config.Key]string,
	proposed config.Map,
	proposedValues map[config.Key]string,
	sources map[string]string,
) []string {
	keys := make([]config.Key, 0, len(previousValues)+len(proposedValues))
	for k := range previousValues {
		keys = append(keys, k)
	}
	for k := range proposedValues {
		if _, ok := previousValues[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.SortFunc(keys, func(a config.Key, b config.Key) int {
		return strings.Compare(a.String(), b.String())
	})

	render := func(m config.Map, k config.Key, value string) string {
		if m[k].Secure() {
			return "[secret]"
		}
		return value
	}

	source := func(k config.Key) string {
		if s := sources[k.String()]; s != "" {
			return " (" + s + ")"
		}
		return ""
	}

	var lines []string
	for _, k := range keys {
		oldValue, inOld := previousValues[k]
		newValue, inNew := proposedValues[k]
		switch {
		case !inOld:
			lines = append(lines, fmt.Sprintf("+ %s: %s%s", k, render(proposed, k, newValue), source(k)))
		case !inNew:
			lines = append(lines, fmt.Sprintf("- %s: %s", k, render(previous, k, oldValue)))
		case oldValue != newValue || previous[k].Secure() != proposed[k].Secure():
			lines = append(lines, fmt.Sprintf("~ %s: %s -> %s%s",
				k, render(previous, k, oldValue), render(proposed, k, newValue), source(k)))
		}
	}
	return lines
}
