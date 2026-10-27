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

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/pulumiconfig"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type managedStore struct {
	stack       backend.Stack
	ps          *workspace.ProjectStack
	definition  *yaml.Node // backs ps.Environment, so saving ps writes the edits
	projectName tokens.PackageName
	configFile  string
}

func rejectConfigFileOnManagedStack(s backend.Stack, configFile string) error {
	loc := s.ConfigLocation()
	if configFile != "" && loc.ManagedEnvironment != "" {
		return cmdStack.ManagedEnvironmentNotSupportedError("--config-file")
	}
	return nil
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
	definition, err := cmdStack.ManagedEnvironmentDefinition(
		loc, ps, orgNamer.OrgName(), s.Ref().Name().String(), configFile)
	if err != nil {
		return nil, err
	}
	return &managedStore{
		stack:       s,
		ps:          ps,
		definition:  definition,
		projectName: projectName,
		configFile:  configFile,
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

func (m *managedStore) Set(_ context.Context, key config.Key, v config.Value, path bool) error {
	root, subPath, err := m.entryPath(key, path)
	if err != nil {
		return err
	}
	value, err := pulumiconfig.ValueNode(v, len(subPath) > 0)
	if err != nil {
		return err
	}
	return pulumiconfig.Set(m.definition, m.entryNames(root), subPath, value)
}

func (m *managedStore) Remove(_ context.Context, key config.Key, path bool) error {
	root, subPath, err := m.entryPath(key, path)
	if err != nil {
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
