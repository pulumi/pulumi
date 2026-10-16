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
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
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
// A write does not move the environment: it creates a revision from the revision the stack file currently
// names, and then rewrites the stack file to name the revision it created. Nobody else's read of the
// environment changes because a stack ran `pulumi config set`.
type mainEnvWriter struct {
	envs       backend.EnvironmentDefinitionsBackend
	org        string
	mainEnv    *workspace.MainEnvironment
	stack      backend.Stack
	ps         *workspace.ProjectStack
	configFile string

	// save writes the stack file. It is a field so that a test can make the write fail after the
	// revision it names has already been created, which is the one ordering in this path where the two
	// writes can disagree.
	save func(context.Context, backend.Stack, *workspace.ProjectStack, string) error
}

// isRevisionNumber reports whether a `mainEnvironment` version pin names a revision number rather than a
// revision tag, using the same grammar the version-addressed ESC routes accept.
func isRevisionNumber(version string) bool {
	n, err := strconv.Atoi(version)
	return err == nil && n > 0
}

func newMainEnvWriter(
	s backend.Stack, ps *workspace.ProjectStack, mainEnv *workspace.MainEnvironment, configFile string,
) (*mainEnvWriter, error) {
	// A numeric pin is the ordinary state of a stack that has been written to before: it names the
	// revision the next write branches from. A tag pin is different -- rewriting `@stable` to `@8` would
	// silently un-tag the stack -- so refuse that one and say how to proceed.
	if mainEnv.Version != "" && !isRevisionNumber(mainEnv.Version) {
		return nil, fmt.Errorf(
			"cannot modify environment %v: this stack pins it to the tag %q; "+
				"pin a revision number instead (for example 'mainEnvironment: %s@8'), "+
				"or edit the environment directly with 'pulumi env set'",
			mainEnv.Ref(), mainEnv.Version, mainEnv.Ref())
	}

	envs, ok := s.Backend().(backend.EnvironmentDefinitionsBackend)
	if !ok {
		return nil, errBackendNoEnvironments(s.Backend())
	}
	orgNamer, ok := s.(interface{ OrgName() string })
	if !ok {
		return nil, fmt.Errorf("cannot determine organization for stack %v", s.Ref())
	}
	return &mainEnvWriter{
		envs:       envs,
		org:        orgNamer.OrgName(),
		mainEnv:    mainEnv,
		stack:      s,
		ps:         ps,
		configFile: configFile,
		save:       cmdStack.SaveProjectStack,
	}, nil
}

// read fetches the environment's definition at the revision this stack is pinned to, along with the
// revision number that read resolved to.
//
// The new revision is created from that number, so the parent is always the definition the caller just
// edited -- never a separately resolved `latest` that could have moved in between.
func (w *mainEnvWriter) read(ctx context.Context) (*yaml.Node, int, error) {
	def, _, revision, err := w.envs.GetEnvironmentDefinition(
		ctx, w.org, w.mainEnv.Project, w.mainEnv.Name, w.mainEnv.Version)
	if err != nil {
		if errors.Is(err, backend.ErrEnvironmentNotFound) {
			// A pinned read fails this way when the environment is there but the revision the stack
			// names is not -- a hand-edited or stale pin. Reporting the environment as missing would
			// send the user to 'pulumi env init', which would then fail because it does exist.
			if w.mainEnv.Version != "" {
				return nil, 0, fmt.Errorf(
					"cannot read environment %v at version %q: no such version, or the environment "+
						"does not exist; correct or remove the '@%s' suffix on 'mainEnvironment' in %s",
					w.mainEnv.Ref(), w.mainEnv.Version, w.mainEnv.Version, w.stackFileName())
			}
			return nil, 0, fmt.Errorf(
				"environment %v does not exist; create it with 'pulumi env init %v', "+
					"or migrate this stack's configuration with 'pulumi config env init --main'",
				w.mainEnv.Ref(), w.mainEnv.Ref())
		}
		return nil, 0, fmt.Errorf("getting environment definition: %w", err)
	}

	docNode := &yaml.Node{}
	if err := yaml.Unmarshal(def, docNode); err != nil {
		return nil, 0, fmt.Errorf("unmarshaling environment definition: %w", err)
	}
	if docNode.Kind != yaml.DocumentNode {
		docNode = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{}}}
	}
	return docNode, revision, nil
}

// writeResult reports the revision a write created, the revision it was created from, and where the
// environment's `latest` tag points. Latest is zero when the lookup that reports it failed, which is not
// an error: by then the revision has been created and the stack file rewritten, so the write succeeded.
type writeResult struct {
	Revision int
	Parent   int
	Latest   int
}

// write creates a revision of the environment from parent and repoints the stack file at it.
//
// The two writes happen in that order deliberately. Saving first would leave the stack file naming a
// revision that may not exist, which breaks every subsequent read of the stack and cannot be diagnosed
// from the file. Creating first leaves, at worst, a real revision the file does not name -- which the
// error below tells the user how to adopt in one line.
func (w *mainEnvWriter) write(
	ctx context.Context, out io.Writer, docNode *yaml.Node, parent int,
) (writeResult, error) {
	newYAML, err := yaml.Marshal(docNode.Content[0])
	if err != nil {
		return writeResult{}, fmt.Errorf("marshaling environment definition: %w", err)
	}

	diags, revision, err := w.envs.CreateEnvironmentRevisionFromParent(
		ctx, w.org, w.mainEnv.Project, w.mainEnv.Name, newYAML, parent)
	if err != nil {
		switch {
		case errors.Is(err, backend.ErrEnvironmentConflict):
			return writeResult{}, fmt.Errorf(
				"environment %v changed since it was read, so the new revision was rejected rather than "+
					"overwriting it; re-run the command", w.mainEnv.Ref())
		case errors.Is(err, backend.ErrEnvironmentNotFound):
			// The service answers a closed rollout gate exactly as it answers a missing environment, so
			// name both possibilities rather than asserting one. There is deliberately no probe and no
			// fallback to the ordinary environment-update route: that would move `latest` invisibly, on
			// the very path where the user believed they were branching from their own pin.
			return writeResult{}, fmt.Errorf(
				"creating a revision of environment %v: revision branching is not available for "+
					"organization %q, or the environment does not exist; 'pulumi config set' cannot "+
					"write to a 'mainEnvironment' stack without it", w.mainEnv.Ref(), w.org)
		}
		return writeResult{}, fmt.Errorf("creating a revision of environment %v: %w", w.mainEnv.Ref(), err)
	}
	// Diagnostics accompany a successful create as well as a rejected one: the service returns the
	// definition's check diagnostics alongside the revision it created, and ESC warns about conditions
	// that are not errors -- a value that cannot be overridden, a duplicate top-level key, an unknown
	// field. So the revision, not the presence of diagnostics, decides whether the write failed: a
	// rejected definition is the one case that comes back without a revision.
	if len(diags) != 0 {
		printESCDiagnostics(out, diags)
	}
	if revision == 0 {
		return writeResult{}, fmt.Errorf("creating a revision of environment %v: too many errors", w.mainEnv.Ref())
	}

	// The revision exists; point the stack at it. Nothing above this line touches the stack file, so
	// every failure path so far leaves it exactly as it was.
	next := workspace.MainEnvironment{
		Project: w.mainEnv.Project,
		Name:    w.mainEnv.Name,
		Version: strconv.Itoa(revision),
	}
	// The pin has to be in place before the save, because the save is what serializes it. But `w.mainEnv`
	// is the very value the caller holds as `ps.MainEnvironment`, so a failed save must put it back: an
	// in-memory pointer at a revision the file does not name is exactly the state that would mislead any
	// caller that retried, re-saved, or kept reading the stack after the error.
	prevEnv, prevPS := *w.mainEnv, w.ps.MainEnvironment
	*w.mainEnv = next
	w.ps.MainEnvironment = w.mainEnv
	if err := w.save(ctx, w.stack, w.ps, w.configFile); err != nil {
		*w.mainEnv, w.ps.MainEnvironment = prevEnv, prevPS
		return writeResult{}, fmt.Errorf(
			"created environment revision %v@%d, but saving %s failed: %w; "+
				"set 'mainEnvironment: %v@%d' in %s by hand to use it",
			w.mainEnv.Ref(), revision, w.stackFileName(), err,
			w.mainEnv.Ref(), revision, w.stackFileName())
	}

	res := writeResult{Revision: revision, Parent: parent}
	// Cosmetic and last: reporting where `latest` still points is worth one GET, but the write has
	// already succeeded, so a failure here must not turn it into an error.
	if latest, err := w.envs.GetEnvironmentRevision(
		ctx, w.org, w.mainEnv.Project, w.mainEnv.Name, ""); err == nil {
		res.Latest = latest
	}
	return res, nil
}

// stackFileName names the stack file in messages, matching how the config commands already name it.
func (w *mainEnvWriter) stackFileName() string {
	if w.configFile != "" {
		return filepath.Base(w.configFile)
	}
	return fmt.Sprintf("Pulumi.%s.yaml", w.stack.Ref().Name())
}

// printWriteResult reports what a write created and what it left alone. Both `config set` and `config rm`
// print through this so the two cannot drift.
func printWriteResult(out io.Writer, mainEnv *workspace.MainEnvironment, stackFile string, res writeResult) {
	fmt.Fprintf(out, "Created %s@%d (parent @%d).\n", mainEnv.Ref(), res.Revision, res.Parent)
	if res.Latest != 0 {
		fmt.Fprintf(out, "%s now points at @%d; latest is still @%d.\n", stackFile, res.Revision, res.Latest)
		return
	}
	fmt.Fprintf(out, "%s now points at @%d.\n", stackFile, res.Revision)
}

// configValuePath is the location of a stack configuration key within an environment definition. The key
// is used verbatim as a single path element so that its `<namespace>:<name>` form is never parsed as a
// path expression.
func configValuePath(key config.Key) resource.PropertyPath {
	return resource.PropertyPath{"pulumiConfig", key.String()}
}

// setKey writes a single configuration value into the environment's `values.pulumiConfig` and reports the
// revision the write created.
func (w *mainEnvWriter) setKey(
	ctx context.Context, out io.Writer, key config.Key, value yaml.Node,
) (writeResult, error) {
	docNode, parent, err := w.read(ctx)
	if err != nil {
		return writeResult{}, err
	}

	valuesNode, ok := encoding.YAMLSyntax{Node: docNode}.Get(resource.PropertyPath{"values"})
	if !ok {
		valuesNode, err = encoding.YAMLSyntax{Node: docNode}.Set(
			nil, resource.PropertyPath{"values"}, yaml.Node{Kind: yaml.MappingNode})
		if err != nil {
			return writeResult{}, fmt.Errorf("internal error: %w", err)
		}
	}
	if _, err = (encoding.YAMLSyntax{Node: valuesNode}).Set(nil, configValuePath(key), value); err != nil {
		return writeResult{}, err
	}

	return w.write(ctx, out, docNode, parent)
}

// removeKey deletes a single configuration value from the environment's `values.pulumiConfig`. It reports
// false if the key was not present, in which case no revision was created and the stack file was not
// touched.
func (w *mainEnvWriter) removeKey(
	ctx context.Context, out io.Writer, key config.Key,
) (writeResult, bool, error) {
	docNode, parent, err := w.read(ctx)
	if err != nil {
		return writeResult{}, false, err
	}

	valuesNode, ok := encoding.YAMLSyntax{Node: docNode}.Get(resource.PropertyPath{"values"})
	if !ok {
		return writeResult{}, false, nil
	}
	before, err := yaml.Marshal(docNode.Content[0])
	if err != nil {
		return writeResult{}, false, fmt.Errorf("marshaling environment definition: %w", err)
	}
	if err = (encoding.YAMLSyntax{Node: valuesNode}).Delete(nil, configValuePath(key)); err != nil {
		return writeResult{}, false, err
	}
	after, err := yaml.Marshal(docNode.Content[0])
	if err != nil {
		return writeResult{}, false, fmt.Errorf("marshaling environment definition: %w", err)
	}
	if bytes.Equal(before, after) {
		return writeResult{}, false, nil
	}

	res, err := w.write(ctx, out, docNode, parent)
	return res, true, err
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
