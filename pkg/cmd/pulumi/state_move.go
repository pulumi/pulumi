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
			sourceStack, err := requireStack(ctx, sourceStackName, stackLoadOnly, display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			})
			if err != nil {
				return err
			}
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

	// We want to move there resources in the order they appear in the source snapshot,
	// resources with relationships are in the right order.
	var resourcesToMoveOrdered []*resource.State
	for _, res := range sourceSnapshot.Resources {
		if _, ok := resourcesToMove[string(res.URN)]; ok {
			resourcesToMoveOrdered = append(resourcesToMoveOrdered, res)
		}
	}

	// run through the source snapshot and find all the providers
	// that need to be copied, remove all the resources that need
	// to be removed, and break the dependencies that are no
	// longer valid.
	//
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

	// Create a root stack if there is none
	rootStack, err := stack.GetRootStackResource(destSnapshot)
	if err != nil {
		return err
	}
	if rootStack == nil {
		projectName, ok := dest.Ref().Project()
		if !ok {
			return errors.New("failed to get project name of source stack")
		}
		rootStack = stack.CreateRootStackResource(
			dest.Ref().Name().Q(), tokens.PackageName(projectName))
		destSnapshot.Resources = append(destSnapshot.Resources, rootStack)
	}

	for _, res := range providers {
		// Providers stay in the source stack, so we need a copy of the provider to be able to
		// rewrite the URNs of the resource.
		r := res.Copy()
		err = rewriteURNs(r, dest)
		if err != nil {
			return err
		}
		destSnapshot.Resources = append(destSnapshot.Resources, r)
	}

	for _, res := range resourcesToMoveOrdered {
		if _, ok := resourcesToMove[string(res.Parent)]; !ok {
			rootStack, err := stack.GetRootStackResource(destSnapshot)
			if err != nil {
				return err
			}
			res.Parent = rootStack.URN
		}

		breakDependencies(res, remainingResources)
		err = rewriteURNs(res, dest)
		if err != nil {
			return err
		}

		destSnapshot.Resources = append(destSnapshot.Resources, res)
	}

	err = destSnapshot.VerifyIntegrity()
	if err != nil {
		return fmt.Errorf(`failed to verify integrity of destination snapshot: %w

This is a bug! We would appreciate a report: https://github.com/pulumi/pulumi/issues/`, err)
	}

	err = sourceSnapshot.VerifyIntegrity()
	if err != nil {
		return fmt.Errorf(`failed to verify integrity of source snapshot: %w

This is a bug! We would appreciate a report: https://github.com/pulumi/pulumi/issues/`, err)
	}

	// We're saving the destination snapshot first, so that if saving a snapshot fails
	// the resources will always still be tracked.  If the source snapshot fails the user
	// will have to manually remove the resources from the source stack.
	//
	// TODO: give a better error message and instructions for how the user can fix this
	// situation.
	err = saveSnapshot(ctx, dest, destSnapshot, false)
	if err != nil {
		return fmt.Errorf("failed to save destination snapshot: %w", err)
	}

	err = saveSnapshot(ctx, source, sourceSnapshot, false)
	if err != nil {
		return fmt.Errorf("failed to save source snapshot: %w", err)
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
	if _, ok := resourcesToMove[string(res.DeletedWith)]; ok {
		res.DeletedWith = ""
	}
}

func renameStackAndProject(urn urn.URN, stack backend.Stack) (urn.URN, error) {
	newURN := urn.RenameStack(stack.Ref().Name())
	if project, ok := stack.Ref().Project(); ok {
		newURN = newURN.RenameProject(tokens.PackageName(project))
	} else {
		return "", errors.New("cannot get project name")
	}
	return newURN, nil
}

func rewriteURNs(res *resource.State, dest backend.Stack) error {
	var err error
	res.URN, err = renameStackAndProject(res.URN, dest)
	if err != nil {
		return err
	}
	if res.Provider != "" {
		providerURN, err := renameStackAndProject(urn.URN(res.Provider), dest)
		if err != nil {
			return err
		}
		res.Provider = string(providerURN)
	}
	if res.Parent != "" {
		parentURN, err := renameStackAndProject(res.Parent, dest)
		if err != nil {
			return err
		}
		res.Parent = parentURN
	}
	for k, dep := range res.Dependencies {
		depURN, err := renameStackAndProject(dep, dest)
		if err != nil {
			return err
		}
		res.Dependencies[k] = depURN
	}
	for k, propDeps := range res.PropertyDependencies {
		for j, propDep := range propDeps {
			depURN, err := renameStackAndProject(propDep, dest)
			if err != nil {
				return err
			}
			res.PropertyDependencies[k][j] = depURN
		}
	}
	if res.DeletedWith != "" {
		urn, err := renameStackAndProject(res.DeletedWith, dest)
		if err != nil {
			return err
		}
		res.DeletedWith = urn
	}
	return nil
}
