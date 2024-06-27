// Copyright 2024-2024, Pulumi Corporation.
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

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

type stateMoveCmd struct{}

func newStateMoveCommand() *cobra.Command {
	var sourceStackName string
	var destStackName string
	stateMove := &stateMoveCmd{}
	cmd := &cobra.Command{
		Use:   "move",
		Short: "Move resources from one stack to another",
		Long: `Move resources from one stack to another

This command can be used to move resources from one stack to another. This can be useful when
splitting a stack into multiple stacks or when merging multiple stacks into one.

EXPERIMENTAL: this feature is currently in development.
`,
		Args: cmdutil.MinimumNArgs(1),
		// TODO: this command should be hidden until it is fully implemented
		Hidden: true,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if sourceStackName == "" && destStackName == "" {
				return errors.New("at least one of --source or --dest must be provided")
			}
			// TODO: make sure to load the source stack even if it is from a different project
			sourceStack, err := requireStack(ctx, sourceStackName, stackLoadOnly, display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			})
			if err != nil {
				return err
			}
			// TODO: make sure to load the dest stack even if it is from a different project.
			destStack, err := requireStack(ctx, destStackName, stackLoadOnly, display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			})
			if err != nil {
				return err
			}

			return stateMove.Run(ctx, sourceStack, destStack, args, stack.DefaultSecretsProvider)
		}),
	}

	cmd.Flags().StringVarP(&sourceStackName, "source", "", "", "The name of the stack to move resources from")
	cmd.Flags().StringVarP(&destStackName, "dest", "", "", "The name of the stack to move resources to")

	return cmd
}

func (cmd *stateMoveCmd) Run(
	ctx context.Context, source backend.Stack, dest backend.Stack, args []string,
	secretsProvider secrets.Provider,
) error {
	sourceSnapshot, err := source.Snapshot(ctx, secretsProvider)
	if err != nil {
		return err
	}
	destSnapshot, err := dest.Snapshot(ctx, secretsProvider)
	if err != nil {
		return err
	}
	err = destSnapshot.VerifyIntegrity()
	if err != nil {
		return fmt.Errorf("failed to verify integrity of destination snapshot: %w", err)
	}

	err = sourceSnapshot.VerifyIntegrity()
	if err != nil {
		return fmt.Errorf("failed to verify integrity of source snapshot: %w", err)
	}

	resourcesToMove := make(map[string]*resource.State)
	providersToCopy := make(map[string]bool)
	remainingResources := make(map[string]*resource.State)
	for _, res := range sourceSnapshot.Resources {
		if resourceMatches(res, args) {
			resourcesToMove[string(res.URN)] = res
			providersToCopy[res.Provider] = true
		} else {
			remainingResources[string(res.URN)] = res
		}
	}

	sourceDepGraph := graph.NewDependencyGraph(sourceSnapshot.Resources)

	// include all children in the list of resources to move
	for _, res := range resourcesToMove {
		for _, dep := range sourceDepGraph.ChildrenOf(res) {
			resourcesToMove[string(dep.URN)] = dep
			providersToCopy[dep.Provider] = true
		}
	}

	// run through the source snapshot and find all the providers
	// that need to be copied, remove all the resources that need
	// to be removed, and break the dependencies that are no
	// longer valid.
	var providers []*resource.State
	i := 0
	for _, res := range sourceSnapshot.Resources {
		// Find providers that need to be copied
		if _, ok := resourcesToMove[string(res.URN)]; ok {
			continue
		}
		if providersToCopy[string(res.URN)+"::"+string(res.ID)] {
			providers = append(providers, res)
		}

		sourceSnapshot.Resources[i] = res
		i++
		breakDependencies(res, resourcesToMove)
	}
	sourceSnapshot.Resources = sourceSnapshot.Resources[:i]

	for _, res := range providers {
		// Providers stay in the source stack, so we need a copy of the provider to be able to
		// rewrite the URNs of the resource.
		copy := res.Copy()
		rewriteURNs(copy, dest)
		destSnapshot.Resources = append(destSnapshot.Resources, copy)
	}

	for _, res := range resourcesToMove {
		breakDependencies(res, remainingResources)
		rewriteURNs(res, dest)
		if _, ok := resourcesToMove[string(res.Parent)]; !ok {
			rootStack, err := stack.GetRootStackResource(destSnapshot)
			if err != nil {
				return err
			}
			if rootStack == nil {
				projectName, ok := source.Ref().Project()
				if !ok {
					return errors.New("failed to get project name of source stack")
				}
				rootStack = stack.CreateRootStackResource(
					source.Ref().Name().Q(), tokens.PackageName(projectName))
				destSnapshot.Resources = append(destSnapshot.Resources, rootStack)
			}
			res.Parent = rootStack.URN
		}

		destSnapshot.Resources = append(destSnapshot.Resources, res)
	}

	err = destSnapshot.VerifyIntegrity()
	if err != nil {
		return fmt.Errorf("failed to verify integrity of destination snapshot: %w", err)
	}

	err = sourceSnapshot.VerifyIntegrity()
	if err != nil {
		return fmt.Errorf("failed to verify integrity of source snapshot: %w", err)
	}

	return nil
}

func resourceMatches(res *resource.State, args []string) bool {
	for _, arg := range args {
		if string(res.URN) == arg {
			return true
		}
	}
	return false
}

func breakDependencies(res *resource.State, resourcesToMove map[string]*resource.State) {
	j := 0
	for _, dep := range res.Dependencies {
		if _, ok := resourcesToMove[string(dep)]; !ok {
			res.Dependencies[j] = dep
			j++
		}
	}
	res.Dependencies = res.Dependencies[:j]
	for k, propDeps := range res.PropertyDependencies {
		j = 0
		for _, propDep := range propDeps {
			if _, ok := resourcesToMove[string(propDep)]; !ok {
				propDeps[j] = propDep
				j++
			}
		}
		res.PropertyDependencies[k] = propDeps[:j]
	}
	if _, ok := resourcesToMove[string(res.DeletedWith)]; !ok {
		res.DeletedWith = ""
	}
}

func rewriteURNs(res *resource.State, dest backend.Stack) {
	// TODO: rewrite project name
	res.URN = res.URN.RenameStack(dest.Ref().Name())
	if res.Provider != "" {
		res.Provider = string(urn.URN(res.Provider).RenameStack(dest.Ref().Name()))
	}
	for k, dep := range res.Dependencies {
		res.Dependencies[k] = dep.RenameStack(dest.Ref().Name())
	}
	for k, propDeps := range res.PropertyDependencies {
		for j, propDep := range propDeps {
			res.PropertyDependencies[k][j] = propDep.RenameStack(dest.Ref().Name())
		}
	}
	if res.DeletedWith != "" {
		res.DeletedWith = res.DeletedWith.RenameStack(dest.Ref().Name())
	}
}
