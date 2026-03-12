// Copyright 2016-2026, Pulumi Corporation.
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
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// ConfigEditor is a write-focused abstraction for mutating stack-owned configuration.
// Mutations are buffered in memory and persisted atomically on Save.
//
// For secrets, the caller passes a config.Value with Secure()=true and the
// plaintext in the value field. Each implementation encrypts eagerly in Set()
// so the buffered state always holds valid encrypted values, never plaintext secrets.
type ConfigEditor interface {
	// Set stores a config value under the given key.
	// If the value is marked secure, it is encrypted immediately.
	// If path is true, the key's name is treated as a property path.
	Set(ctx context.Context, key config.Key, value config.Value, path bool) error

	// Remove deletes the config key.
	// No-op if the key does not exist.
	// If path is true, removes a nested value within an object.
	Remove(ctx context.Context, key config.Key, path bool) error

	// Save flushes all buffered mutations to the backing store atomically.
	Save(ctx context.Context) error
}

// LocalConfigEditor implements ConfigEditor for stacks with local file-backed configuration.
type LocalConfigEditor struct {
	stack     backend.Stack
	ps        *workspace.ProjectStack
	encrypter config.Encrypter
}

// NewConfigEditor returns a ConfigEditor for the given stack.
// For stacks with remote (ESC-backed) config it returns an escConfigEditor that
// reads and writes the ESC environment definition directly.
// For stacks with local file-backed config it returns a LocalConfigEditor.
func NewConfigEditor(
	ctx context.Context,
	stack backend.Stack,
	ps *workspace.ProjectStack,
	encrypter config.Encrypter,
) (ConfigEditor, error) {
	loc := stack.ConfigLocation()
	if !loc.IsRemote || loc.EscEnv == nil {
		return &LocalConfigEditor{stack: stack, ps: ps, encrypter: encrypter}, nil
	}

	envBackend, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return nil, fmt.Errorf(
			"backend %q does not support ESC environments; cannot use service-backed config",
			stack.Backend().Name())
	}

	orgName, ok := stack.(interface{ OrgName() string })
	if !ok {
		return nil, fmt.Errorf("stack does not expose organization name; cannot use service-backed config")
	}

	// EscEnv is formatted as "<project>/<envname>".
	envProject, envName, found := strings.Cut(*loc.EscEnv, "/")
	if !found {
		return nil, fmt.Errorf("malformed ESC environment reference %q: expected \"<project>/<name>\"", *loc.EscEnv)
	}

	// Load the current environment definition YAML so we can modify it and send the updated version.
	// We pass decrypt=false so secrets remain wrapped in fn::secret — we write them back as-is.
	yamlBytes, etag, _, err := envBackend.GetEnvironment(ctx, orgName.OrgName(), envProject, envName, "", false)
	if err != nil {
		return nil, fmt.Errorf("loading ESC environment %s/%s: %w", envProject, envName, err)
	}

	var envDef map[string]any
	if len(yamlBytes) > 0 {
		if err := yaml.Unmarshal(yamlBytes, &envDef); err != nil {
			return nil, fmt.Errorf("parsing ESC environment %s/%s: %w", envProject, envName, err)
		}
	}

	return &escConfigEditor{
		envBackend: envBackend,
		orgName:    orgName.OrgName(),
		envProject: envProject,
		envName:    envName,
		envDef:     envDef,
		etag:       etag,
	}, nil
}

// Set encrypts the value if secure (eager encryption, not deferred to Save),
// then delegates to config.Map.Set which handles path navigation internally.
func (e *LocalConfigEditor) Set(ctx context.Context, k config.Key, v config.Value, path bool) error {
	if v.Secure() {
		plaintext, err := v.Value(config.NopDecrypter)
		if err != nil {
			return err
		}
		encrypted, err := e.encrypter.EncryptValue(ctx, plaintext)
		if err != nil {
			return err
		}
		v = config.NewSecureValue(encrypted)
	}
	return e.ps.Config.Set(k, v, path)
}

// Remove deletes the key from config. No-op if the key does not exist.
func (e *LocalConfigEditor) Remove(_ context.Context, k config.Key, path bool) error {
	return e.ps.Config.Remove(k, path)
}

// Save writes the in-memory config state to the local Pulumi.<stack>.yaml file.
func (e *LocalConfigEditor) Save(ctx context.Context) error {
	return cmdStack.SaveProjectStack(ctx, e.stack, e.ps)
}

// escConfigEditor implements ConfigEditor for stacks whose configuration is stored in an ESC environment.
// Mutations are buffered as modifications to an in-memory parsed YAML map and flushed atomically on Save.
// Secrets are wrapped in the ESC fn::secret construct; the ESC service encrypts them at rest.
type escConfigEditor struct {
	envBackend backend.EnvironmentsBackend
	orgName    string
	envProject string
	envName    string
	// envDef holds the parsed environment YAML as a generic map so we can modify it
	// structurally without losing non-config sections (imports, environmentVariables, etc).
	envDef map[string]any
	etag   string
}

// getPulumiConfig returns the values.pulumiConfig map, creating all intermediate nodes as needed.
func (e *escConfigEditor) getPulumiConfig() map[string]any {
	if e.envDef == nil {
		e.envDef = map[string]any{}
	}
	values, _ := e.envDef["values"].(map[string]any)
	if values == nil {
		values = map[string]any{}
		e.envDef["values"] = values
	}
	pc, _ := values["pulumiConfig"].(map[string]any)
	if pc == nil {
		pc = map[string]any{}
		values["pulumiConfig"] = pc
	}
	return pc
}

// getPulumiConfigIfExists returns the values.pulumiConfig map or nil if it does not exist.
func (e *escConfigEditor) getPulumiConfigIfExists() map[string]any {
	if e.envDef == nil {
		return nil
	}
	values, _ := e.envDef["values"].(map[string]any)
	if values == nil {
		return nil
	}
	pc, _ := values["pulumiConfig"].(map[string]any)
	return pc
}

// Set stores a config value in the ESC environment YAML buffer.
// Secrets are wrapped in {"fn::secret": plaintext} — the ESC service encrypts them.
func (e *escConfigEditor) Set(_ context.Context, k config.Key, v config.Value, path bool) error {
	// Extract the raw string value. For secrets the caller passes plaintext via NewSecureValue.
	raw, err := v.Value(config.NopDecrypter)
	if err != nil {
		return err
	}

	var yamlValue any
	if v.Secure() {
		yamlValue = map[string]any{"fn::secret": raw}
	} else {
		yamlValue = raw
	}

	pc := e.getPulumiConfig()

	if !path {
		pc[k.Namespace()+":"+k.Name()] = yamlValue
		return nil
	}

	// path=true: the key name is a dotted property path (e.g. "db.host").
	// We parse it and map the first segment to a namespaced YAML key, then
	// navigate (or create) nested maps for any remaining segments.
	segments, err := resource.ParsePropertyPathStrict(k.Name())
	if err != nil {
		return fmt.Errorf("invalid config path %q: %w", k.Name(), err)
	}
	if len(segments) == 0 {
		return fmt.Errorf("empty config path for key %v", k)
	}

	rootKey := k.Namespace() + ":" + fmt.Sprint(segments[0])
	return setNestedYAMLValue(pc, rootKey, segments[1:], yamlValue)
}

// setNestedYAMLValue sets a value at rootKey[remainingPath] inside parent,
// creating intermediate maps as needed. Returns an error if any path segment
// is an integer (array index), which is not supported for service-backed stacks.
func setNestedYAMLValue(parent map[string]any, rootKey string, remainingPath resource.PropertyPath, value any) error {
	if len(remainingPath) == 0 {
		parent[rootKey] = value
		return nil
	}
	if _, isInt := remainingPath[0].(int); isInt {
		return fmt.Errorf(
			"array index paths are not supported for service-backed config; " +
				"use `pulumi config edit` to modify array values directly")
	}
	existing, _ := parent[rootKey].(map[string]any)
	if existing == nil {
		existing = map[string]any{}
	}
	nextKey := fmt.Sprint(remainingPath[0])
	if err := setNestedYAMLValue(existing, nextKey, remainingPath[1:], value); err != nil {
		return err
	}
	parent[rootKey] = existing
	return nil
}

// Remove deletes a config key from the ESC environment YAML buffer.
func (e *escConfigEditor) Remove(_ context.Context, k config.Key, path bool) error {
	pc := e.getPulumiConfigIfExists()
	if pc == nil {
		return nil // no pulumiConfig section — nothing to remove
	}

	if !path {
		delete(pc, k.Namespace()+":"+k.Name())
		return nil
	}

	segments, err := resource.ParsePropertyPathStrict(k.Name())
	if err != nil {
		return fmt.Errorf("invalid config path %q: %w", k.Name(), err)
	}
	if len(segments) == 0 {
		return nil
	}

	rootKey := k.Namespace() + ":" + fmt.Sprint(segments[0])
	return deleteNestedYAMLValue(pc, rootKey, segments[1:])
}

// deleteNestedYAMLValue deletes the leaf value at rootKey[remainingPath] inside parent.
// Returns an error if any path segment is an integer (array index), which is not
// supported for service-backed stacks.
func deleteNestedYAMLValue(parent map[string]any, rootKey string, remainingPath resource.PropertyPath) error {
	if len(remainingPath) == 0 {
		delete(parent, rootKey)
		return nil
	}
	if _, isInt := remainingPath[0].(int); isInt {
		return fmt.Errorf(
			"array index paths are not supported for service-backed config; " +
				"use `pulumi config edit` to modify array values directly")
	}
	nested, _ := parent[rootKey].(map[string]any)
	if nested == nil {
		return nil // path does not exist; no-op
	}
	nextKey := fmt.Sprint(remainingPath[0])
	return deleteNestedYAMLValue(nested, nextKey, remainingPath[1:])
}

// Save serialises the modified environment definition and persists it via the ESC API.
// The etag is used for optimistic concurrency — if another writer has modified the
// environment since the editor was created, Save returns a descriptive conflict error.
func (e *escConfigEditor) Save(ctx context.Context) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(e.envDef); err != nil {
		return fmt.Errorf("serialising ESC environment definition: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("serialising ESC environment definition: %w", err)
	}

	diags, err := e.envBackend.UpdateEnvironmentWithProject(
		ctx, e.orgName, e.envProject, e.envName, buf.Bytes(), e.etag)
	if err != nil {
		if isHTTPConflict(err) {
			return fmt.Errorf(
				"the ESC environment %s/%s was modified concurrently; please retry: %w",
				e.envProject, e.envName, err)
		}
		return fmt.Errorf("saving config to ESC environment %s/%s: %w", e.envProject, e.envName, err)
	}
	if len(diags) > 0 {
		return fmt.Errorf("ESC environment %s/%s validation failed:\n%s",
			e.envProject, e.envName, formatEnvDiags(diags))
	}
	return nil
}

// isHTTPConflict reports whether err is an HTTP 409 Conflict response.
func isHTTPConflict(err error) bool {
	var r *apitype.ErrorResponse
	return errors.As(err, &r) && r.Code == http.StatusConflict
}

// isHTTPNotFound reports whether err is an HTTP 404 Not Found response.
func isHTTPNotFound(err error) bool {
	var r *apitype.ErrorResponse
	return errors.As(err, &r) && r.Code == http.StatusNotFound
}

// stackOrgName returns the organization name for a stack using the optional OrgName() interface.
// Returns an error if the stack implementation does not expose one.
func stackOrgName(s backend.Stack) (string, error) {
	type orgNamer interface{ OrgName() string }
	on, ok := s.(orgNamer)
	if !ok {
		return "", fmt.Errorf("stack %q does not expose an organization name", s.Ref().Name())
	}
	return on.OrgName(), nil
}

// parseEscEnvRef splits an ESC environment reference in "<project>/<name>" format.
// Returns an error for malformed references.
func parseEscEnvRef(ref string) (project, name string, err error) {
	project, name, found := strings.Cut(ref, "/")
	if !found {
		return "", "", fmt.Errorf("malformed ESC environment reference %q: expected \"<project>/<name>\"", ref)
	}
	return project, name, nil
}

// formatEnvDiags formats ESC environment diagnostics into a human-readable string.
func formatEnvDiags(diags apitype.EnvironmentDiagnostics) string {
	var sb strings.Builder
	for _, d := range diags {
		sb.WriteString("  - ")
		sb.WriteString(d.Summary)
		sb.WriteString("\n")
	}
	return sb.String()
}
