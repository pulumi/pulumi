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
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/pulumiconfig"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type managedStore struct {
	stack       backend.Stack
	ps          *workspace.ProjectStack
	definition  *yaml.Node // backs ps.Environment, so saving ps writes the edits
	projectName tokens.PackageName
	configFile  string
	org         string
	sink        diag.Sink
	imported    *importCheck

	overrideImport bool
}

type importCheck struct {
	values       map[string]esc.Value
	checkWarning error
}

func managedEnvForWrite(
	s backend.Stack,
	projectName tokens.PackageName,
	ps *workspace.ProjectStack,
	configFile string,
) (*managedStore, error) {
	loc := s.ConfigLocation()
	if loc.ManagedEnvironment == "" {
		return nil, nil
	}
	orgNamer, ok := s.(interface{ OrgName() string })
	if !ok {
		return nil, fmt.Errorf("cannot determine organization for stack %v", s.Ref())
	}
	org := orgNamer.OrgName()
	definition, err := cmdStack.ManagedEnvironmentDefinition(loc, ps, org, s.Ref().Name().String(), configFile)
	if err != nil {
		return nil, err
	}
	return &managedStore{
		stack:       s,
		ps:          ps,
		definition:  definition,
		projectName: projectName,
		configFile:  configFile,
		org:         org,
		sink:        cmdutil.Diag(),
	}, nil
}

func removeConfigKeys(
	ctx context.Context,
	s backend.Stack,
	project *workspace.Project,
	ps *workspace.ProjectStack,
	configFile string,
	keys []config.Key,
	path bool,
) error {
	managed, err := managedEnvForWrite(s, project.Name, ps, configFile)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if managed != nil {
			err = managed.Remove(ctx, key, path)
		} else {
			err = ps.Config.Remove(key, path)
		}
		if err != nil {
			return err
		}
	}
	if managed != nil {
		return managed.Save(ctx)
	}
	return cmdStack.SaveProjectStack(ctx, s, ps, configFile)
}

func (m *managedStore) Set(ctx context.Context, key config.Key, v config.Value, path bool) error {
	root, subPath, err := m.entryPath(key, path)
	if err != nil {
		return err
	}
	names, err := m.checkImported(ctx, key, root, subPath, false)
	if err != nil {
		return err
	}
	value, err := pulumiconfig.ValueNode(v, len(subPath) > 0)
	if err != nil {
		return err
	}
	return pulumiconfig.Set(m.definition, names, subPath, value)
}

func (m *managedStore) Remove(ctx context.Context, key config.Key, path bool) error {
	root, subPath, err := m.entryPath(key, path)
	if err != nil {
		return err
	}
	if _, err := m.checkImported(ctx, key, root, subPath, true); err != nil {
		return err
	}
	return pulumiconfig.Remove(m.definition, m.entryNames(root), subPath)
}

func (m *managedStore) Save(ctx context.Context) error {
	return cmdStack.SaveProjectStack(ctx, m.stack, m.ps, m.configFile)
}

func (m *managedStore) entryPath(key config.Key, path bool) (config.Key, resource.PropertyPath, error) {
	if !path {
		return key, nil, nil
	}
	segments, err := resource.ParsePropertyPathStrict(key.Name())
	if err != nil {
		return config.Key{}, nil, fmt.Errorf("invalid config key path: %w", err)
	}
	if len(segments) == 0 {
		return config.Key{}, nil, errors.New("empty config key path")
	}
	first, ok := segments[0].(string)
	if !ok || first == "" {
		return config.Key{}, nil, errors.New("first path segment of config key must be a non-empty string")
	}
	root := config.MustMakeKey(key.Namespace(), first)
	if len(segments) == 1 {
		return root, nil, nil
	}
	return root, segments[1:], nil
}

func (m *managedStore) entryNames(root config.Key) []string {
	if root.Namespace() == string(m.projectName) {
		return []string{root.String(), root.Name()}
	}
	return []string{root.String()}
}

func (m *managedStore) checkImported(
	ctx context.Context, key, root config.Key, subPath resource.PropertyPath, removing bool,
) ([]string, error) {
	imported, checkWarning := m.importedConfig(ctx)
	names := m.entryNames(root)
	local, isLocal := pulumiconfig.Find(m.definition, names)
	if removing && isLocal {
		// The local entry may be an object that ESC merges with the import, so the path can name only an imported value.
		if value, ok := imported[local]; ok && len(subPath) > 0 && hasPath(value, subPath) {
			if _, ok := pulumiconfig.Get(m.definition, local, subPath); !ok {
				return nil, errRemoveImported(key, value.Trace.Def.Environment)
			}
		}
		return names, nil
	}
	if checkWarning != nil {
		outcome := "writing it to the local definition"
		if removing {
			outcome = "it is not set in the local definition"
		}
		m.sink.Warningf(diag.Message("", "could not check imported environments for %s: %v; %s"),
			root, checkWarning, outcome)
		return names, nil
	}
	for _, name := range names {
		value, ok := imported[name]
		if !ok {
			continue
		}
		env := value.Trace.Def.Environment
		if name == local {
			if m.entersImportedList(name, value, subPath) {
				return nil, errEntersImportedList(root, env)
			}
			continue
		}
		if isLocal {
			return nil, fmt.Errorf("pulumiConfig sets %s locally and imported environment %s sets %s; use one spelling",
				local, env, name)
		}
		if removing {
			return nil, errRemoveImported(root, env)
		}
		if m.overrideImport {
			if m.entersImportedList(name, value, subPath) {
				return nil, errEntersImportedList(root, env)
			}
			m.sink.Infof(diag.Message("", "%s overrides the value from imported environment %s"), root, env)
			// The import's spelling, so the two don't become two spellings of one key with an undefined winner.
			return []string{name}, nil
		}
		return nil, fmt.Errorf("config key %s is defined by imported environment %s; the CLI does not edit "+
			"imported environments. Set it in that environment with `pulumi env set`, remove the import, or pass "+
			"`--override-import` to override it in this stack's environment", root, env)
	}
	return names, nil
}

// entersImportedList reports whether a path write into entry name would replace part of a list that the import
// defines. ESC merges objects key by key but any other local value replaces the imported one whole, so the write is
// safe only where the local definition already has a value in the list's place.
func (m *managedStore) entersImportedList(name string, imported esc.Value, path resource.PropertyPath) bool {
	prefix, ok := importedListPrefix(imported, path)
	if !ok {
		return false
	}
	_, ok = pulumiconfig.Get(m.definition, name, prefix)
	return !ok
}

// importedListPrefix returns the part of path that names a list in v, when path goes on into that list.
func importedListPrefix(v esc.Value, path resource.PropertyPath) (resource.PropertyPath, bool) {
	for i, segment := range path {
		switch repr := v.Value.(type) {
		case []esc.Value:
			return path[:i], true
		case map[string]esc.Value:
			key, ok := segment.(string)
			if !ok {
				return nil, false
			}
			if v, ok = repr[key]; !ok {
				return nil, false
			}
		default:
			return nil, false
		}
	}
	return nil, false
}

func errRemoveImported(key config.Key, env string) error {
	return fmt.Errorf("config key %s is defined by imported environment %s; the CLI does not edit "+
		"imported environments. Set it in that environment with `pulumi env set`, or remove the import", key, env)
}

func hasPath(v esc.Value, path resource.PropertyPath) bool {
	for _, segment := range path {
		switch repr := v.Value.(type) {
		case map[string]esc.Value:
			key, ok := segment.(string)
			if !ok {
				return false
			}
			if v, ok = repr[key]; !ok {
				return false
			}
		case []esc.Value:
			index, ok := segment.(int)
			if !ok || index < 0 || index >= len(repr) {
				return false
			}
			v = repr[index]
		default:
			return false
		}
	}
	return true
}

func errEntersImportedList(root config.Key, env string) error {
	return fmt.Errorf("config key %s reaches into a list that imported environment %s defines, and a local list "+
		"replaces the whole imported list; override the whole list instead of part of it", root, env)
}

func (m *managedStore) importedConfig(ctx context.Context) (values map[string]esc.Value, checkWarning error) {
	if m.imported != nil {
		return m.imported.values, m.imported.checkWarning
	}
	defer func() { m.imported = &importCheck{values: values, checkWarning: checkWarning} }()

	// Imports only: diagnostics from a broken local values block would otherwise hide every imported key.
	imports, ok := pulumiconfig.ImportsDefinition(m.definition)
	if !ok {
		return nil, nil
	}
	envs, ok := m.stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return nil, errBackendNoEnvironments(m.stack.Backend())
	}
	env, diags, err := envs.CheckYAMLEnvironment(ctx, m.org, imports)
	summaries := make([]string, 0, len(diags))
	for _, d := range diags {
		summaries = append(summaries, d.Summary)
	}
	if errResp, ok := errors.AsType[*client.EnvironmentErrorResponse](err); ok {
		for _, d := range errResp.Diagnostics {
			summaries = append(summaries, d.Summary)
		}
	}
	if len(summaries) != 0 {
		return nil, errors.New(strings.Join(summaries, "; "))
	}
	if err != nil {
		return nil, err
	}
	if env != nil {
		values, _ = env.Properties["pulumiConfig"].Value.(map[string]esc.Value)
	}
	return values, nil
}
