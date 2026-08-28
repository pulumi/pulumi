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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/syntax/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// effectiveStackEnv decides which environment definition a stack resolves its configuration through.
//
// A stack that sets `mainEnvironment:` resolves a synthesized environment that imports exactly that one
// named environment, which keeps every downstream behaviour (imports, environment variables, secret
// handling, `pulumiConfig` merging) identical to an `environment:` stack. It returns the environment
// definition to resolve, the main environment when one is active, and any warnings the caller should
// surface to the user.
func effectiveStackEnv(
	s backend.Stack, ps *workspace.ProjectStack,
) (*workspace.Environment, *workspace.MainEnvironment, []string) {
	if ps == nil {
		return nil, nil, nil
	}
	if ps.MainEnvironment == nil {
		return ps.Environment, nil, nil
	}

	// Remote stack configuration is a separate, service-side mechanism that already owns the stack's
	// config. Leave it in charge and say so rather than silently resolving two sources.
	if s != nil && s.ConfigLocation().IsRemote {
		return ps.Environment, nil, []string{
			"'mainEnvironment' is ignored because this stack's configuration is stored remotely",
		}
	}

	var warnings []string
	if ps.Environment != nil {
		warnings = append(warnings,
			"'environment' is ignored because this stack sets 'mainEnvironment'")
	}
	return workspace.NewEnvironment([]string{ps.MainEnvironment.String()}), ps.MainEnvironment, warnings
}

// activeMainEnvironment returns the stack's main environment, or nil if it does not have one in effect,
// printing any warning that explains why a configured 'mainEnvironment' is not in effect.
func activeMainEnvironment(out io.Writer, s backend.Stack, ps *workspace.ProjectStack) *workspace.MainEnvironment {
	_, mainEnv, warnings := effectiveStackEnv(s, ps)
	printConfigWarnings(out, warnings)
	return mainEnv
}

func printConfigWarnings(out io.Writer, warnings []string) {
	color := cmdutil.GetGlobalColorization()
	for _, w := range warnings {
		fmt.Fprintln(out, color.Colorize(colors.SpecWarning+"warning: "+w+colors.Reset))
	}
}

// errMainEnvUnsupported reports that a config subcommand has not been taught to write through a main
// environment. Refusing is deliberate: silently writing the local `config:` block would shadow the
// environment's values instead of updating them.
func errMainEnvUnsupported(command string, mainEnv *workspace.MainEnvironment) error {
	return fmt.Errorf(
		"'pulumi config %s' is not supported yet on a stack that sets 'mainEnvironment: %s'; "+
			"use 'pulumi config set'/'pulumi config rm', or edit the environment directly with 'pulumi env'",
		command, mainEnv)
}

// mainEnvWriter performs read-modify-write updates of a stack's main environment.
//
// Every write carries the etag returned by the read that immediately preceded it, so a violation of the
// single-writer assumption surfaces as a hard error instead of clobbering someone else's change.
type mainEnvWriter struct {
	envs    backend.EnvironmentDefinitionsBackend
	org     string
	mainEnv *workspace.MainEnvironment
}

func newMainEnvWriter(s backend.Stack, mainEnv *workspace.MainEnvironment) (*mainEnvWriter, error) {
	// Writing to a pinned stack would create a revision the stack itself would never read, so the very
	// next `pulumi config` would not show the value that was just set. Refuse instead.
	if mainEnv.Version != "" {
		return nil, fmt.Errorf(
			"cannot modify environment %v: this stack pins it to version %q; "+
				"remove the '@%s' pin from 'mainEnvironment' to write to it, "+
				"or edit the environment directly with 'pulumi env set'",
			mainEnv.Ref(), mainEnv.Version, mainEnv.Version)
	}

	envs, ok := s.Backend().(backend.EnvironmentDefinitionsBackend)
	if !ok {
		return nil, errBackendNoEnvironments(s.Backend())
	}
	orgNamer, ok := s.(interface{ OrgName() string })
	if !ok {
		return nil, fmt.Errorf("cannot determine organization for stack %v", s.Ref())
	}
	return &mainEnvWriter{envs: envs, org: orgNamer.OrgName(), mainEnv: mainEnv}, nil
}

// read fetches the environment's definition along with the etag a subsequent write must carry.
func (w *mainEnvWriter) read(ctx context.Context) (*yaml.Node, string, error) {
	def, etag, _, err := w.envs.GetEnvironmentDefinition(ctx, w.org, w.mainEnv.Project, w.mainEnv.Name, "")
	if err != nil {
		if errors.Is(err, backend.ErrEnvironmentNotFound) {
			return nil, "", fmt.Errorf(
				"environment %v does not exist; create it with 'pulumi env init %v', "+
					"or migrate this stack's configuration with 'pulumi config env init --main'",
				w.mainEnv.Ref(), w.mainEnv.Ref())
		}
		return nil, "", fmt.Errorf("getting environment definition: %w", err)
	}

	docNode := &yaml.Node{}
	if err := yaml.Unmarshal(def, docNode); err != nil {
		return nil, "", fmt.Errorf("unmarshaling environment definition: %w", err)
	}
	if docNode.Kind != yaml.DocumentNode {
		docNode = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{}}}
	}
	return docNode, etag, nil
}

// write sends the edited definition back with the etag from the preceding read and returns the revision
// the update created.
func (w *mainEnvWriter) write(ctx context.Context, out io.Writer, docNode *yaml.Node, etag string) (int, error) {
	newYAML, err := yaml.Marshal(docNode.Content[0])
	if err != nil {
		return 0, fmt.Errorf("marshaling environment definition: %w", err)
	}

	diags, revision, err := w.envs.UpdateEnvironmentDefinition(
		ctx, w.org, w.mainEnv.Project, w.mainEnv.Name, newYAML, etag)
	if err != nil {
		if errors.Is(err, backend.ErrEnvironmentConflict) {
			return 0, fmt.Errorf(
				"environment %v changed since it was read, so the update was rejected rather than "+
					"overwriting it; re-run the command", w.mainEnv.Ref())
		}
		return 0, fmt.Errorf("updating environment: %w", err)
	}
	if len(diags) != 0 {
		printESCDiagnostics(out, diags)
		return 0, fmt.Errorf("updating environment %v: too many errors", w.mainEnv.Ref())
	}
	return revision, nil
}

// configValuePath is the location of a stack configuration key within an environment definition. The key
// is used verbatim as a single path element so that its `<namespace>:<name>` form is never parsed as a
// path expression.
func configValuePath(key config.Key) resource.PropertyPath {
	return resource.PropertyPath{"pulumiConfig", key.String()}
}

// setKey writes a single configuration value into the environment's `values.pulumiConfig` and returns the
// revision the write created.
func (w *mainEnvWriter) setKey(
	ctx context.Context, out io.Writer, key config.Key, value yaml.Node,
) (int, error) {
	docNode, etag, err := w.read(ctx)
	if err != nil {
		return 0, err
	}

	valuesNode, ok := encoding.YAMLSyntax{Node: docNode}.Get(resource.PropertyPath{"values"})
	if !ok {
		valuesNode, err = encoding.YAMLSyntax{Node: docNode}.Set(
			nil, resource.PropertyPath{"values"}, yaml.Node{Kind: yaml.MappingNode})
		if err != nil {
			return 0, fmt.Errorf("internal error: %w", err)
		}
	}
	if _, err = (encoding.YAMLSyntax{Node: valuesNode}).Set(nil, configValuePath(key), value); err != nil {
		return 0, err
	}

	return w.write(ctx, out, docNode, etag)
}

// removeKey deletes a single configuration value from the environment's `values.pulumiConfig`. It reports
// false if the key was not present, in which case no revision was created.
func (w *mainEnvWriter) removeKey(ctx context.Context, out io.Writer, key config.Key) (int, bool, error) {
	docNode, etag, err := w.read(ctx)
	if err != nil {
		return 0, false, err
	}

	valuesNode, ok := encoding.YAMLSyntax{Node: docNode}.Get(resource.PropertyPath{"values"})
	if !ok {
		return 0, false, nil
	}
	before, err := yaml.Marshal(docNode.Content[0])
	if err != nil {
		return 0, false, fmt.Errorf("marshaling environment definition: %w", err)
	}
	if err = (encoding.YAMLSyntax{Node: valuesNode}).Delete(nil, configValuePath(key)); err != nil {
		return 0, false, err
	}
	after, err := yaml.Marshal(docNode.Content[0])
	if err != nil {
		return 0, false, fmt.Errorf("marshaling environment definition: %w", err)
	}
	if bytes.Equal(before, after) {
		return 0, false, nil
	}

	revision, err := w.write(ctx, out, docNode, etag)
	return revision, true, err
}

// ConfigValueNode renders a `pulumi config set` value as the YAML node to store in the environment.
//
// Plain values follow `pulumi config set`'s own typing rules, so an untyped "6" stays the string "6".
// Secrets are wrapped in `fn::secret` and encrypted by ESC: the stack's secrets manager is never involved
// on this path, and the plaintext exists only in this in-memory node.
func ConfigValueNode(value string, typ string, secret bool) (yaml.Node, error) {
	var node yaml.Node
	if secret {
		node.SetString(value)
		wrapped, err := yaml.Marshal(map[string]yaml.Node{"fn::secret": node})
		if err != nil {
			return yaml.Node{}, fmt.Errorf("internal error: marshaling secret: %w", err)
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(wrapped, &doc); err != nil {
			return yaml.Node{}, fmt.Errorf("internal error: marshaling secret: %w", err)
		}
		return *doc.Content[0], nil
	}

	switch typ {
	case "", "string":
		node.SetString(value)
	case "int":
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return yaml.Node{}, fmt.Errorf("invalid int value %q", value)
		}
		node = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: value}
	case "float":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return yaml.Node{}, fmt.Errorf("invalid float value %q", value)
		}
		node = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: value}
	case "bool":
		if _, err := strconv.ParseBool(value); err != nil {
			return yaml.Node{}, fmt.Errorf("invalid bool value %q", value)
		}
		node = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: value}
	default:
		return yaml.Node{}, fmt.Errorf("invalid type %q; must be one of string, int, bool, or float", typ)
	}
	return node, nil
}

//
// Attribution.
//

// unmigratedSource is the SOURCE shown for values that still live in the stack file even though the stack
// has a main environment. Those values win over the environment, so making them visible is the point.
func unmigratedSource(stackName string) string {
	return fmt.Sprintf("Pulumi.%s.yaml (unmigrated)", stackName)
}

// sourceIndex attributes each configuration key to the environment revision that defined it.
type sourceIndex struct {
	sources    map[config.Key]string
	unmigrated []config.Key
}

func (i *sourceIndex) get(key config.Key) string {
	if i == nil {
		return ""
	}
	return i.sources[key]
}

// revisionResolver resolves environment references to revision numbers, caching one lookup per distinct
// environment. Under the single-writer assumption a latest-revision lookup is race-free.
type revisionResolver struct {
	ctx   context.Context
	envs  backend.EnvironmentDefinitionsBackend
	org   string
	cache map[string]int
}

// describe renders `<project>/<env>@<revision>`, degrading to `<project>/<env>` when the revision cannot
// be resolved (for instance when an environment reference cannot be split into a project and a name).
func (r *revisionResolver) describe(ref string, version string) string {
	if r == nil || r.envs == nil {
		return ref
	}
	if rev, ok := r.cache[ref+"@"+version]; ok {
		if rev == 0 {
			return ref
		}
		return fmt.Sprintf("%s@%d", ref, rev)
	}

	rev := 0
	if envProject, envName, ok := strings.Cut(ref, "/"); ok && envProject != "" && envName != "" {
		if n, err := r.envs.GetEnvironmentRevision(r.ctx, r.org, envProject, envName, version); err == nil {
			rev = n
		}
	}
	if r.cache == nil {
		r.cache = map[string]int{}
	}
	r.cache[ref+"@"+version] = rev

	if rev == 0 {
		return ref
	}
	return fmt.Sprintf("%s@%d", ref, rev)
}

// traceEnvironment returns the reference of the environment that defined a value, normalized against the
// main environment, along with the version it was pinned to (empty when unpinned). ESC reports the defining
// environment's name, which may or may not be qualified with its project or pinned to a version.
func traceEnvironment(v esc.Value, mainEnv *workspace.MainEnvironment) (string, string) {
	name, version, _ := strings.Cut(v.Trace.Def.Environment, "@")
	switch name {
	case "", "yaml", mainEnv.Name, mainEnv.Ref():
		// An empty or synthesized environment name means the value came from the stack's own main
		// environment rather than one of its imports.
		return mainEnv.Ref(), mainEnv.Version
	}
	return name, version
}

// buildSourceIndex attributes every configuration value visible to the stack to the environment revision
// (or the stack file) that defined it.
func buildSourceIndex(
	ctx context.Context,
	s backend.Stack,
	mainEnv *workspace.MainEnvironment,
	projectName tokens.PackageName,
	stackName string,
	pulumiEnv esc.Value,
	localCfg config.Map,
) *sourceIndex {
	index := &sourceIndex{sources: map[config.Key]string{}}

	resolver := &revisionResolver{ctx: ctx, org: "", cache: map[string]int{}}
	if envs, ok := s.Backend().(backend.EnvironmentDefinitionsBackend); ok {
		if orgNamer, ok := s.(interface{ OrgName() string }); ok {
			resolver.envs, resolver.org = envs, orgNamer.OrgName()
		}
	}
	mainSource := resolver.describe(mainEnv.Ref(), mainEnv.Version)

	if envMap, ok := pulumiEnv.Value.(map[string]esc.Value); ok {
		for rawKey, value := range envMap {
			key, err := ParseConfigKeyForProject(projectName, rawKey, false /*path*/)
			if err != nil {
				continue
			}
			// An import pinned to a version defined the value at that version, not at the
			// environment's latest revision, so the pin has to survive into the lookup.
			if ref, version := traceEnvironment(value, mainEnv); ref != mainEnv.Ref() {
				index.sources[key] = resolver.describe(ref, version) + " (imported)"
			} else {
				index.sources[key] = mainSource
			}
		}
	}

	for key := range localCfg {
		index.sources[key] = unmigratedSource(stackName)
		index.unmigrated = append(index.unmigrated, key)
	}
	sort.Sort(config.KeyArray(index.unmigrated))

	return index
}

// showSource prints where a single configuration value came from and what it overrode.
func showSource(
	out io.Writer,
	index *sourceIndex,
	mainEnv *workspace.MainEnvironment,
	key config.Key,
	pulumiEnv esc.Value,
) {
	source := index.get(key)
	if source == "" {
		return
	}
	fmt.Fprintf(out, "Source: %s\n", source)

	var value *esc.Value
	if envMap, ok := pulumiEnv.Value.(map[string]esc.Value); ok {
		if v, ok := envMap[key.String()]; ok {
			value = &v
		} else if v, ok := envMap[key.Name()]; ok {
			value = &v
		}
	}
	if value == nil {
		return
	}

	// A value still in the stack file wins over the environment's definition of the same key.
	envRef, _ := traceEnvironment(*value, mainEnv)
	if strings.HasSuffix(source, "(unmigrated)") {
		fmt.Fprintf(out, "  overrides %s\n", envRef)
	}
	if def := value.Trace.Def; def.Begin.Line != 0 {
		fmt.Fprintf(out, "  defined at %s:%d:%d\n", envRef, def.Begin.Line, def.Begin.Column)
	}
	for base := value.Trace.Base; base != nil; base = base.Trace.Base {
		baseRef, _ := traceEnvironment(*base, mainEnv)
		fmt.Fprintf(out, "  overrides %s\n", baseRef)
	}
}

// printConfigSource reports the environment revision a command resolved its configuration against.
func printConfigSource(
	ctx context.Context, out io.Writer, s backend.Stack, mainEnv *workspace.MainEnvironment,
) {
	resolver := &revisionResolver{ctx: ctx}
	if envs, ok := s.Backend().(backend.EnvironmentDefinitionsBackend); ok {
		if orgNamer, ok := s.(interface{ OrgName() string }); ok {
			resolver.envs, resolver.org = envs, orgNamer.OrgName()
		}
	}
	fmt.Fprintf(out, "Config source: %s\n", resolver.describe(mainEnv.Ref(), mainEnv.Version))
}
