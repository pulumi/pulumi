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
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
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
	setTargets     map[setKey]setTarget
}

type setKey struct {
	key  config.Key
	path bool
}

type setTarget struct {
	names   []string
	subPath resource.PropertyPath
}

type importCheck struct {
	values       map[string]esc.Value
	checkWarning error
}

func newManagedStore(
	s backend.Stack,
	projectName tokens.PackageName,
	ps *workspace.ProjectStack,
	configFile string,
	overrideImport bool,
) (*managedStore, error) {
	loc := s.ConfigLocation()
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
		stack:          s,
		ps:             ps,
		definition:     definition,
		projectName:    projectName,
		configFile:     configFile,
		org:            org,
		sink:           cmdutil.Diag(),
		overrideImport: overrideImport,
	}, nil
}

func (m *managedStore) Set(ctx context.Context, key config.Key, v config.Value, path bool) error {
	target, err := m.setTarget(ctx, key, path)
	if err != nil {
		return err
	}
	value, err := pulumiconfig.ValueNode(v, len(target.subPath) > 0)
	if err != nil {
		return err
	}
	return pulumiconfig.Set(m.definition, target.names, target.subPath, value)
}

func (m *managedStore) CheckSet(ctx context.Context, key config.Key, path bool) error {
	_, err := m.setTarget(ctx, key, path)
	return err
}

// setTarget checks each key once, so CheckSet followed by Set prints the import warning or override note once.
func (m *managedStore) setTarget(ctx context.Context, key config.Key, path bool) (setTarget, error) {
	if target, ok := m.setTargets[setKey{key, path}]; ok {
		return target, nil
	}
	root, subPath, err := m.entryPath(key, path)
	if err != nil {
		return setTarget{}, err
	}
	names, err := m.checkImported(ctx, key, root, subPath, false)
	if err != nil {
		return setTarget{}, err
	}
	target := setTarget{names: names, subPath: subPath}
	if m.setTargets == nil {
		m.setTargets = map[setKey]setTarget{}
	}
	m.setTargets[setKey{key, path}] = target
	return target, nil
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

func rejectSecretObjects(jsonConfig map[string]configValueJSON) error {
	for jsonKey, jsonValue := range jsonConfig {
		if jsonValue.Secret && jsonValue.ObjectValue != nil {
			return jsonConfigSetError(jsonKey, pulumiconfig.ErrSecretObject)
		}
	}
	return nil
}

func (m *managedStore) Encrypter(context.Context) (config.Encrypter, error) {
	be, err := backend.AsManagedEnvironmentsBackend(m.stack.Backend())
	if err != nil {
		return nil, err
	}
	project, name := m.stack.ConfigLocation().ManagedEnvironmentRef()
	return &managedEnvEncrypter{backend: be, org: m.org, project: project, name: name}, nil
}

type managedEnvEncrypter struct {
	backend backend.ManagedEnvironmentsBackend
	org     string
	project string
	name    string
}

func (e *managedEnvEncrypter) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	ciphertexts, err := e.BatchEncrypt(ctx, []string{plaintext})
	if err != nil {
		return "", err
	}
	return ciphertexts[0], nil
}

func (e *managedEnvEncrypter) BatchEncrypt(ctx context.Context, secrets []string) ([]string, error) {
	plaintexts := make([][]byte, len(secrets))
	for i, secret := range secrets {
		plaintexts[i] = []byte(secret)
	}
	ciphertexts, err := e.backend.EncryptEnvironmentSecrets(ctx, e.org, e.project, e.name, plaintexts)
	if err != nil {
		return nil, fmt.Errorf("encrypting the value for environment %s/%s/%s: %w", e.org, e.project, e.name, err)
	}
	return ciphertexts, nil
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
				return nil, errImportedKey(key, value.Trace.Def.Environment, local, subPath, true)
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
			return nil, errImportedKey(root, env, name, subPath, true)
		}
		if m.overrideImport {
			if m.entersImportedList(name, value, subPath) {
				return nil, errEntersImportedList(root, env)
			}
			m.sink.Infof(diag.Message("", "%s overrides the value from imported environment %s"), root, env)
			// The import's spelling, so the two don't become two spellings of one key with an undefined winner.
			return []string{name}, nil
		}
		return nil, errImportedKey(root, env, name, subPath, false)
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

func errImportedKey(key config.Key, env, entry string, subPath resource.PropertyPath, removing bool) error {
	envPath := "pulumiConfig." + entry
	if len(subPath) > 0 {
		rendered := subPath.String()
		if !strings.HasPrefix(rendered, "[") {
			rendered = "." + rendered
		}
		envPath += rendered
	}
	fix := fmt.Sprintf("Set it in that environment with `pulumi env set %s %s <value>`, remove the import, or pass "+
		"`--override-import` to override it in this stack's environment", env, envPath)
	if removing {
		fix = fmt.Sprintf("Remove it from that environment with `pulumi env rm %s %s`, or remove the import", env, envPath)
	}
	return fmt.Errorf("config key %s is defined by imported environment %s; the CLI does not edit imported "+
		"environments. %s", key, env, fix)
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
	if errResp, ok := errors.AsType[*client.EnvironmentErrorResponse](err); ok {
		for _, d := range errResp.Diagnostics {
			diags = append(diags, apitype.EnvironmentDiagnostic{Range: d.Range, Summary: d.Summary, Detail: d.Detail})
		}
	}
	if len(diags) != 0 {
		return nil, errors.New(oneLineESCDiagnostics(diags))
	}
	if err != nil {
		return nil, err
	}
	if env != nil {
		values, _ = env.Properties["pulumiConfig"].Value.(map[string]esc.Value)
	}
	return values, nil
}
