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
	"net/http"
	"os"
	"sort"
	"strings"

	escEncoding "github.com/pulumi/esc/syntax/encoding"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// runMigrate writes a local-config stack's configuration into a backing ESC environment (secrets
// wrapped as fn::secret) and links the stack to it. The decrypted plaintext is held only in memory;
// it is never written to a file, log, preview, or diagnostic.
func (cmd *configEnvInitCmd) runMigrate(ctx context.Context) error {
	if !cmd.yes && !cmd.parent.interactive {
		return backenderr.ErrNonInteractiveRequiresYes
	}

	opts := display.Options{Color: cmd.parent.color}

	project, _, err := cmd.parent.ws.ReadProject()
	if err != nil {
		return err
	}

	stack, err := cmd.parent.requireStack(
		ctx,
		cmd.parent.diags,
		cmd.parent.ws,
		cmdBackend.DefaultLoginManager,
		*cmd.parent.stackRef,
		cmdStack.OfferNew|cmdStack.SetCurrent,
		opts,
		*cmd.parent.configFile,
	)
	if err != nil {
		return err
	}

	if stack.ConfigLocation().IsRemote {
		return errors.New("this stack already uses remote configuration; migration is not needed")
	}

	envBackend, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return fmt.Errorf("backend %v does not support environments", stack.Backend().Name())
	}
	orgNamer, ok := stack.(interface{ OrgName() string })
	if !ok {
		return errors.New("internal error: stack does not provide an organization name")
	}
	orgName := orgNamer.OrgName()

	envProject := project.Name.String()
	envName := stack.Ref().Name().String()
	first, second, found := strings.Cut(cmd.envName, "/")
	if found {
		envProject = first
		envName = second
	} else if first != "" {
		envName = first
	}
	fullEnvName := envProject + "/" + envName

	ps, decryptedConfig, err := cmd.getStackConfig(ctx, cmd.parent.diags, project, stack)
	if err != nil {
		return err
	}

	// The stack's `environment` block is itself an inline ESC definition. Seed the migrated definition
	// from it so inline values survive, then overlay stack config on top (config wins), matching today's
	// resolution order.
	pulumiConfig, ok := renderConfigValueForESC(property.New(decryptedConfig)).(map[string]any)
	if !ok {
		return errors.New("internal error: rendered stack configuration is not a map")
	}
	sourceDef, err := buildMigratedDefinition(ps.Environment.Definition(), pulumiConfig, fullEnvName)
	if err != nil {
		return err
	}

	if !cmd.yes && cmd.parent.interactive {
		if !ui.ConfirmPrompt(
			fmt.Sprintf("Migrate the configuration of stack %v to environment %s and link the stack to it?",
				stack.Ref().Name(), fullEnvName),
			"yes", opts) {
			return errors.New("migration canceled")
		}
	}

	// Write the environment before linking so a transient link failure reconciles on re-run (the env
	// exists, so the retry merges into it). A persistent link failure leaves the env orphaned and the
	// stack local; the env is not rolled back.
	if err := cmd.writeMigratedEnvironment(ctx, envBackend, orgName, envProject, envName, sourceDef); err != nil {
		return err
	}

	ps.Environment = workspace.NewEnvironment([]string{fullEnvName})
	ps.Config = nil
	// saveProjectStack dispatches on the stack's current config location, which is still local here,
	// so it would write the local file. Link via SaveRemoteConfig directly instead.
	if err := stack.SaveRemoteConfig(ctx, ps); err != nil {
		return fmt.Errorf("linking stack to environment %s: %w", fullEnvName, err)
	}

	fmt.Fprintf(cmd.parent.stdout, "Migrated stack %v to remote configuration in environment %s\n",
		stack.Ref().Name(), fullEnvName)

	// The migration is durable once the stack is linked, so a local-file cleanup failure must not mask
	// that success; report it as a warning rather than returning an error.
	if err := cmd.cleanupLocalConfig(stack, opts); err != nil {
		fmt.Fprintf(cmd.parent.stdout, "Warning: cleaning up the local configuration file failed: %v\n", err)
	}
	return nil
}

// buildMigratedDefinition renders the stack config as an ESC environment definition, seeded from the
// inline `environment` block (config overlaid on top, config wins).
func buildMigratedDefinition(inlineDef []byte, pulumiConfig map[string]any, self string) (*yaml.Node, error) {
	var doc yaml.Node
	if len(inlineDef) != 0 {
		if err := yaml.Unmarshal(inlineDef, &doc); err != nil {
			return nil, fmt.Errorf("unmarshaling stack environment definition: %w", err)
		}
	}
	if doc.Kind != yaml.DocumentNode {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{}}}
	}
	syntax := escEncoding.YAMLSyntax{Node: &doc}

	if _, ok := syntax.Get(resource.PropertyPath{"values", "pulumiConfig"}); !ok {
		if _, err := syntax.Set(
			nil, resource.PropertyPath{"values", "pulumiConfig"}, yaml.Node{Kind: yaml.MappingNode}); err != nil {
			return nil, fmt.Errorf("internal error: %w", err)
		}
	}

	// Sort for deterministic output.
	keys := make([]string, 0, len(pulumiConfig))
	for k := range pulumiConfig {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		valueNode, err := valueToYAMLNode(pulumiConfig[key])
		if err != nil {
			return nil, err
		}
		if _, err := syntax.Set(nil, resource.PropertyPath{"values", "pulumiConfig", key}, valueNode); err != nil {
			return nil, fmt.Errorf("setting key %q: %w", key, err)
		}
	}

	if err := removeSelfImport(&doc, self); err != nil {
		return nil, err
	}
	forceBlockStyle(&doc)
	return &doc, nil
}

// forceBlockStyle clears flow style across a node tree. ProjectStack.Environment.Definition() returns
// JSON for an imports-list environment, and yaml.Unmarshal tags JSON nodes as flow style; without this
// the seed's root would re-emit as inline JSON ({imports: [...], values: {...}}) instead of block YAML.
func forceBlockStyle(n *yaml.Node) {
	n.Style = 0
	for _, c := range n.Content {
		forceBlockStyle(c)
	}
}

// removeSelfImport strips an import equal to the migrated stack's own environment (a prior non-remote
// `config env init` may have added one), deleting the now-empty `imports` key entirely.
func removeSelfImport(doc *yaml.Node, self string) error {
	syntax := escEncoding.YAMLSyntax{Node: doc}
	node, ok := syntax.Get(resource.PropertyPath{"imports"})
	if !ok || node.Kind != yaml.SequenceNode {
		return nil
	}
	kept := node.Content[:0]
	for _, n := range node.Content {
		if name, ok := importEntryName(n); ok && name == self {
			continue
		}
		kept = append(kept, n)
	}
	node.Content = kept
	if len(node.Content) == 0 {
		if err := syntax.Delete(nil, resource.PropertyPath{"imports"}); err != nil {
			return fmt.Errorf("removing empty imports: %w", err)
		}
	}
	return nil
}

// writeMigratedEnvironment creates the target environment, or merges into it if it already exists.
func (cmd *configEnvInitCmd) writeMigratedEnvironment(
	ctx context.Context,
	envBackend backend.EnvironmentsBackend,
	orgName, envProject, envName string,
	sourceDef *yaml.Node,
) error {
	def, etag, _, err := envBackend.GetEnvironment(ctx, orgName, envProject, envName, "", false)
	if err != nil {
		var errResp *apitype.ErrorResponse
		if errors.As(err, &errResp) && errResp.Code == http.StatusNotFound {
			return cmd.createMigratedEnvironment(ctx, envBackend, orgName, envProject, envName, sourceDef)
		}
		return fmt.Errorf("getting environment %s/%s: %w", envProject, envName, err)
	}
	// The env exists; mergeMigratedEnvironment handles an empty existing definition.
	return cmd.mergeMigratedEnvironment(ctx, envBackend, orgName, envProject, envName, etag, def, sourceDef)
}

func (cmd *configEnvInitCmd) createMigratedEnvironment(
	ctx context.Context,
	envBackend backend.EnvironmentsBackend,
	orgName, envProject, envName string,
	sourceDef *yaml.Node,
) error {
	newYAML, err := yaml.Marshal(sourceDef.Content[0])
	if err != nil {
		return fmt.Errorf("encoding environment definition: %w", err)
	}

	diags, err := envBackend.CreateEnvironment(ctx, orgName, envProject, envName, newYAML)
	if err != nil {
		return fmt.Errorf("creating environment %s/%s: %w", envProject, envName, err)
	}
	if diags.HasErrors() {
		return fmt.Errorf("creating environment %s/%s: %w", envProject, envName, diags)
	}
	return nil
}

// mergeMigratedEnvironment merges the migrated definition into an existing environment at the
// YAML-node level so existing comments and unrelated sections survive.
func (cmd *configEnvInitCmd) mergeMigratedEnvironment(
	ctx context.Context,
	envBackend backend.EnvironmentsBackend,
	orgName, envProject, envName string,
	etag string,
	existingDef []byte,
	sourceDef *yaml.Node,
) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(existingDef, &doc); err != nil {
		return fmt.Errorf("unmarshaling environment definition: %w", err)
	}
	if doc.Kind != yaml.DocumentNode {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{}}}
	}

	if err := cmd.mergeValues(&doc, sourceDef); err != nil {
		return err
	}
	if err := mergeImportNodes(&doc, sourceDef); err != nil {
		return err
	}

	newYAML, err := yaml.Marshal(doc.Content[0])
	if err != nil {
		return fmt.Errorf("marshaling definition: %w", err)
	}

	diags, err := envBackend.UpdateEnvironmentWithProject(ctx, orgName, envProject, envName, newYAML, etag)
	if err != nil {
		if errors.Is(err, backend.ErrConfigConflict) {
			return fmt.Errorf("the environment was modified concurrently; please retry: %w", err)
		}
		return fmt.Errorf("updating environment %s/%s: %w", envProject, envName, err)
	}
	if diags.HasErrors() {
		return fmt.Errorf("updating environment %s/%s: %w", envProject, envName, diags)
	}
	return nil
}

// mergeValues merges sourceDef's `values` subsections into target key by key, so unrelated keys in
// the target survive.
func (cmd *configEnvInitCmd) mergeValues(target, sourceDef *yaml.Node) error {
	values, ok := (escEncoding.YAMLSyntax{Node: sourceDef}).Get(resource.PropertyPath{"values"})
	if !ok || values.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(values.Content); i += 2 {
		section := values.Content[i].Value
		node := values.Content[i+1]
		if node.Kind != yaml.MappingNode {
			if err := cmd.setMergedValue(target, resource.PropertyPath{"values", section}, node); err != nil {
				return err
			}
			continue
		}
		for j := 0; j+1 < len(node.Content); j += 2 {
			key := node.Content[j].Value
			if err := cmd.setMergedValue(target, resource.PropertyPath{"values", section, key}, node.Content[j+1]); err != nil {
				return err
			}
		}
	}
	return nil
}

// setMergedValue sets path in target, warning only when it overwrites an existing node whose value
// differs. The reconcile path (re-run after a failed link) re-writes identical values, so warning on
// an unchanged value would falsely imply a destructive overwrite.
func (cmd *configEnvInitCmd) setMergedValue(target *yaml.Node, path resource.PropertyPath, value *yaml.Node) error {
	syntax := escEncoding.YAMLSyntax{Node: target}
	if existing, ok := syntax.Get(path); ok {
		changed, err := yamlNodesDiffer(existing, value)
		if err != nil {
			return err
		}
		if changed {
			fmt.Fprintf(cmd.parent.stdout, "Warning: overwriting existing key %q\n", path[len(path)-1])
		}
	}
	if _, err := syntax.Set(nil, path, *value); err != nil {
		return fmt.Errorf("merging %v: %w", path[len(path)-1], err)
	}
	return nil
}

// mergeImportNodes appends source's imports not already present (by name) to target's imports
// sequence. Entries are cloned verbatim so a structured import (e.g. {env: {merge: false}}) keeps its
// options rather than being flattened to a bare name.
func mergeImportNodes(target, source *yaml.Node) error {
	srcImports, ok := (escEncoding.YAMLSyntax{Node: source}).Get(resource.PropertyPath{"imports"})
	if !ok || srcImports.Kind != yaml.SequenceNode || len(srcImports.Content) == 0 {
		return nil
	}

	syntax := escEncoding.YAMLSyntax{Node: target}
	var seq *yaml.Node
	if node, ok := syntax.Get(resource.PropertyPath{"imports"}); ok {
		// A non-sequence `imports` is a malformed env; refuse rather than silently discard it.
		if node.Kind != yaml.SequenceNode {
			return errors.New("environment's `imports` is not a sequence")
		}
		seq = node
	} else {
		var err error
		seq, err = syntax.Set(nil, resource.PropertyPath{"imports"}, yaml.Node{Kind: yaml.SequenceNode})
		if err != nil {
			return fmt.Errorf("internal error: %w", err)
		}
	}

	present := map[string]bool{}
	for _, n := range seq.Content {
		if name, ok := importEntryName(n); ok {
			present[name] = true
		}
	}
	for _, n := range srcImports.Content {
		name, ok := importEntryName(n)
		if !ok || present[name] {
			continue
		}
		present[name] = true
		seq.Content = append(seq.Content, cloneYAMLNode(n))
	}
	return nil
}

// cloneYAMLNode deep-copies a node so grafting it elsewhere doesn't share mutable state with the source.
func cloneYAMLNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	c := *n
	c.Content = make([]*yaml.Node, len(n.Content))
	for i, child := range n.Content {
		c.Content[i] = cloneYAMLNode(child)
	}
	return &c
}

// valueToYAMLNode marshals a native value (already rendered by renderConfigValueForESC) to a YAML node.
func valueToYAMLNode(v any) (yaml.Node, error) {
	b, err := yaml.Marshal(v)
	if err != nil {
		return yaml.Node{}, err
	}
	var node yaml.Node
	if err := yaml.Unmarshal(b, &node); err != nil {
		return yaml.Node{}, err
	}
	if len(node.Content) == 0 {
		return yaml.Node{}, errors.New("internal error: empty value node")
	}
	return *node.Content[0], nil
}

func yamlNodesDiffer(a, b *yaml.Node) (bool, error) {
	ab, err := yaml.Marshal(a)
	if err != nil {
		return false, err
	}
	bb, err := yaml.Marshal(b)
	if err != nil {
		return false, err
	}
	return !bytes.Equal(ab, bb), nil
}

// cleanupLocalConfig deletes the local Pulumi.<stack>.yaml after a migration, but only if
// --cleanup-local was passed or the user confirms; by default it is kept.
func (cmd *configEnvInitCmd) cleanupLocalConfig(stack backend.Stack, opts display.Options) error {
	remove := cmd.cleanupLocal
	if !remove && cmd.parent.interactive && !cmd.yes {
		remove = ui.ConfirmPrompt(
			"Delete the local stack configuration file now that config is stored remotely?",
			"yes", opts)
	}
	if !remove {
		fmt.Fprintf(cmd.parent.stdout,
			"Kept the local stack configuration file; it is no longer used and can be deleted\n")
		return nil
	}

	path := *cmd.parent.configFile
	if path == "" {
		_, detected, err := workspace.DetectProjectStackPath(stack.Ref().Name().Q())
		if err != nil {
			return fmt.Errorf("locating local stack configuration file: %w", err)
		}
		path = detected
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing local stack configuration file: %w", err)
	}
	fmt.Fprintf(cmd.parent.stdout, "Deleted the local stack configuration file %s\n", path)
	return nil
}
