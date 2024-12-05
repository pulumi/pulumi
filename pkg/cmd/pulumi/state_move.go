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
	"io"
	"os"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

type stateMoveCmd struct {
	Stdin          io.Reader
	Stdout         io.Writer
	Colorizer      colors.Colorization
	Yes            bool
	IncludeParents bool

	ws pkgWorkspace.Context
}

func newStateMoveCommand() *cobra.Command {
	var sourceStackName string
	var destStackName string
	var yes bool
	var includeParents bool
	stateMove := &stateMoveCmd{
		Colorizer: cmdutil.GetGlobalColorization(),
	}
	cmd := &cobra.Command{
		Use:   "move [flags] <urn>...",
		Short: "Move resources from one stack to another",
		Long: `Move resources from one stack to another

This command can be used to move resources from one stack to another. This can be useful when
splitting a stack into multiple stacks or when merging multiple stacks into one.
`,
		Args: cmdutil.MinimumNArgs(1),
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance

			if sourceStackName == "" && destStackName == "" {
				return errors.New("at least one of --source or --dest must be provided")
			}
			sourceStack, err := requireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				sourceStackName,
				stackLoadOnly,
				display.Options{
					Color:         cmdutil.GetGlobalColorization(),
					IsInteractive: true,
				},
			)
			if err != nil {
				return err
			}
			destStack, err := requireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				destStackName,
				stackLoadOnly,
				display.Options{
					Color:         cmdutil.GetGlobalColorization(),
					IsInteractive: true,
				},
			)
			if err != nil {
				return err
			}

			stateMove.Yes = yes
			stateMove.IncludeParents = includeParents

			sourceSecretsProvider := stack.NamedStackSecretsProvider{
				StackName: sourceStack.Ref().FullyQualifiedName().String(),
			}
			destSecretsProvider := stack.NamedStackSecretsProvider{
				StackName: destStack.Ref().FullyQualifiedName().String(),
			}

			return stateMove.Run(ctx, sourceStack, destStack, args, sourceSecretsProvider, destSecretsProvider)
		}),
	}

	cmd.Flags().StringVarP(&sourceStackName, "source", "", "", "The name of the stack to move resources from")
	cmd.Flags().StringVarP(&destStackName, "dest", "", "", "The name of the stack to move resources to")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Automatically approve and perform the move")
	cmd.Flags().BoolVarP(&includeParents, "include-parents", "", false,
		"Include all the parents of the moved resources as well")

	return cmd
}

func (cmd *stateMoveCmd) Run(
	ctx context.Context, source backend.Stack, dest backend.Stack, args []string,
	sourceSecretsProvider secrets.Provider, destSecretsProvider secrets.Provider,
) error {
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	if cmd.ws == nil {
		cmd.ws = pkgWorkspace.Instance
	}

	sourceSnapshot, err := source.Snapshot(ctx, sourceSecretsProvider)
	if err != nil {
		return err
	}
	destSnapshot, err := dest.Snapshot(ctx, destSecretsProvider)
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

	if sourceSnapshot == nil {
		return errors.New("source stack has no resources")
	}
	if destSnapshot == nil {
		destSnapshot = &deploy.Snapshot{}
	}
	if destSnapshot.SecretsManager == nil {
		// If the destination stack has no secret manager, we
		// need to create one.  This only works if the user is
		// currently in the destination project directory.  If
		// we fail here this indicates that they are not, and
		// we return an projectError explaining that.
		//nolint:lll
		projectError := errors.New("destination stack has no secret manager. To move resources either initialize the stack with a secret manager, or run the pulumi state move command from the destination project directory")
		path, err := workspace.DetectProjectPath()
		if err != nil {
			return projectError
		}
		if path == "" {
			return projectError
		}
		project, err := workspace.LoadProject(path)
		if err != nil {
			return projectError
		}
		if string(project.Name) != string(dest.Ref().FullyQualifiedName().Namespace().Name()) {
			return projectError
		}

		// The user is in the right directory.  If we fail below we will return the error of that failure.
		err = createSecretsManagerForExistingStack(ctx, cmd.ws, dest, "", false, true)
		if err != nil {
			return err
		}
		ps, err := loadProjectStack(project, dest)
		if err != nil {
			return err
		}

		destSecretManager, err := dest.DefaultSecretManager(ps)
		if err != nil {
			return err
		}
		destSnapshot.SecretsManager = destSecretManager
	}

	resourcesToMove := make(map[string]*resource.State)
	providersToCopy := make(map[string]bool)
	unmatchedArgs := mapset.NewSet(args...)
	rootStackURN := ""
	for _, res := range sourceSnapshot.Resources {
		matchedArg := resourceMatches(res, args)
		if matchedArg != "" {
			if strings.HasPrefix(string(res.Type), "pulumi:providers:") {
				//nolint:lll
				return errors.New("cannot move providers. Only resources can be moved, and providers will be included automatically")
			}
			if res.Type == resource.RootStackType && res.Parent == "" {
				rootStackURN = string(res.URN)
			}
			resourcesToMove[string(res.URN)] = res
			providersToCopy[res.Provider] = true
			unmatchedArgs.Remove(matchedArg)
		}
	}

	for _, arg := range unmatchedArgs.ToSlice() {
		fmt.Fprintf(cmd.Stdout, cmd.Colorizer.Colorize(colors.SpecWarning+"warning:"+
			colors.Reset+" Resource %s not found in source stack\n"), arg)
	}

	if len(resourcesToMove) == 0 {
		return errors.New("no resources found to move")
	}

	sourceDepGraph := graph.NewDependencyGraph(sourceSnapshot.Resources)

	if cmd.IncludeParents {
		for _, res := range resourcesToMove {
			for _, parent := range sourceDepGraph.ParentsOf(res) {
				if res.Type == resource.RootStackType && res.Parent == "" {
					// We don't move the root stack explicitly, the code below will take care of dealing with that correctly.
					continue
				}
				resourcesToMove[string(parent.URN)] = parent
				providersToCopy[parent.Provider] = true
			}
		}
	}

	// include all children in the list of resources to move
	for _, res := range resourcesToMove {
		for _, dep := range sourceDepGraph.ChildrenOf(res) {
			resourcesToMove[string(dep.URN)] = dep
			providersToCopy[dep.Provider] = true
		}
	}

	// We don't want to include the root stack in the list of resources to move.  The root stack
	// either already exists in the destination stack or will be created when we move the resources.
	delete(resourcesToMove, rootStackURN)

	// We want to move the resources in the order they appear in the source snapshot,
	// so that resources with relationships are in the right order.  Also check which
	// resources are remaining in the source stack, now that we know all resources
	// that are going to be moved.
	remainingResources := make(map[string]*resource.State)
	var resourcesToMoveOrdered []*resource.State
	for _, res := range sourceSnapshot.Resources {
		if _, ok := resourcesToMove[string(res.URN)]; ok {
			resourcesToMoveOrdered = append(resourcesToMoveOrdered, res)
		} else {
			remainingResources[string(res.URN)] = res
		}
	}

	// run through the source snapshot and find all the providers
	// that need to be copied, remove all the resources that need
	// to be removed, and break the dependencies that are no
	// longer valid.
	var providers []*resource.State
	var brokenSourceDependencies []brokenDependency
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
		brokenSourceDependencies = append(brokenSourceDependencies, breakDependencies(res, resourcesToMove)...)
	}
	sourceSnapshot.Resources = sourceSnapshot.Resources[:i]

	// Save a copy of the destination snapshot so we can restore it if saving the source snapshot with the
	// deleted resources fails.
	originalDestResources := destSnapshot.Resources

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
		destSnapshot.Resources = append([]*resource.State{rootStack}, destSnapshot.Resources...)
	}

	destResMap := make(map[urn.URN]*resource.State)
	for _, res := range destSnapshot.Resources {
		destResMap[res.URN] = res
	}

	rewriteMap := make(map[string]string)
	for _, res := range providers {
		// Providers stay in the source stack, so we need a copy of the provider to be able to
		// rewrite the URNs of the resource.
		r := res.Copy()
		if _, ok := resourcesToMove[string(r.Parent)]; !ok {
			rootStack, err := stack.GetRootStackResource(destSnapshot)
			if err != nil {
				return err
			}
			r.Parent = rootStack.URN
		}
		err = rewriteURNs(r, dest, nil)
		if err != nil {
			return err
		}

		if destRes, ok := destResMap[r.URN]; ok {
			// If the provider ID matches, we can assume that the provider has previously been copied and we can just copy it.
			if destRes.ID == r.ID {
				continue
			}
			// If all the inputs of the provider in the destination stack are the same as the provider in the source stack,
			// we can assume that the provider is equal for the purpose of resources depending on it.  We don't need to copy
			// it, but we need to set the provider for all resources to the provider in the destination stack.
			if destRes.Inputs.DeepEquals(r.Inputs) {
				rewriteMap[fmt.Sprintf("%s::%s", res.URN, res.ID)] = fmt.Sprintf("%s::%s", destRes.URN, destRes.ID)
				continue
			}
			return fmt.Errorf("provider %s already exists in destination stack", r.URN)
		}

		destSnapshot.Resources = append(destSnapshot.Resources, r)
	}

	fmt.Fprintf(cmd.Stdout, cmd.Colorizer.Colorize(
		colors.SpecHeadline+"Planning to move the following resources from %s to %s:\n"+colors.Reset),
		source.Ref().FullyQualifiedName(), dest.Ref().FullyQualifiedName())

	fmt.Fprintf(cmd.Stdout, "\n")
	for _, res := range resourcesToMoveOrdered {
		fmt.Fprintf(cmd.Stdout, "  - %s\n", res.URN)
	}
	fmt.Fprintf(cmd.Stdout, "\n")

	var brokenDestDependencies []brokenDependency
	for _, res := range resourcesToMoveOrdered {
		// We need the original resources URNs later in case of errors, so make a copy here before modifying them.
		r := res.Copy()
		if _, ok := resourcesToMove[string(r.Parent)]; !ok {
			rootStack, err := stack.GetRootStackResource(destSnapshot)
			if err != nil {
				return err
			}
			r.Parent = rootStack.URN
		}

		brokenDestDependencies = append(brokenDestDependencies, breakDependencies(r, remainingResources)...)
		err = rewriteURNs(r, dest, rewriteMap)
		if err != nil {
			return err
		}

		if _, ok := destResMap[r.URN]; ok {
			return fmt.Errorf("resource %s already exists in destination stack", r.URN)
		}

		destSnapshot.Resources = append(destSnapshot.Resources, r)
	}

	if len(brokenSourceDependencies) > 0 {
		fmt.Fprintf(cmd.Stdout, cmd.Colorizer.Colorize(
			colors.SpecWarning+"The following resources remaining in %s have dependencies on resources moved to %s:\n\n"+
				colors.Reset), source.Ref().FullyQualifiedName(), dest.Ref().FullyQualifiedName())
	}

	cmd.printBrokenDependencyRelationships(brokenSourceDependencies)

	if len(brokenDestDependencies) > 0 {
		if len(brokenSourceDependencies) > 0 {
			fmt.Fprintln(cmd.Stdout)
		}
		fmt.Fprintf(cmd.Stdout, cmd.Colorizer.Colorize(
			colors.SpecWarning+"The following resources being moved to %s have dependencies on resources in %s:\n\n"+
				colors.Reset), dest.Ref().FullyQualifiedName(), source.Ref().FullyQualifiedName())
	}

	cmd.printBrokenDependencyRelationships(brokenDestDependencies)

	if len(brokenSourceDependencies) > 0 || len(brokenDestDependencies) > 0 {
		fmt.Fprint(cmd.Stdout, cmd.Colorizer.Colorize(
			colors.SpecInfo+"\nIf you go ahead with moving these dependencies, it will be necessary to create the "+
				"appropriate inputs and outputs in the program for the stack the resources are moved to.\n\n"+
				colors.Reset))
	}

	if !cmd.Yes {
		yes := "yes"
		no := "no"
		msg := "Do you want to perform this move?"
		options := []string{
			yes,
			no,
		}

		switch response := promptUser(msg, options, no, cmdutil.GetGlobalColorization()); response {
		case yes:
		// continue
		case no:
			fmt.Println("Confirmation denied, not proceeding with the state move")
			return nil
		}
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
	err = saveSnapshot(ctx, dest, destSnapshot, false)
	if err != nil {
		return fmt.Errorf(`failed to save destination snapshot: %w

None of the resources have been moved, it is safe to try again`, err)
	}

	err = saveSnapshot(ctx, source, sourceSnapshot, false)
	if err != nil {
		// Try to restore the destination snapshot to its original state
		destSnapshot.Resources = originalDestResources
		errDest := saveSnapshot(ctx, dest, destSnapshot, false)
		if errDest != nil {
			var deleteCommands string
			// Iterate over the resources in reverse order, so resources with no dependencies will be deleted first.
			for i := len(resourcesToMoveOrdered) - 1; i >= 0; i-- {
				deleteCommands += fmt.Sprintf(
					"\n    pulumi state delete --stack %s '%s'",
					source.Ref().FullyQualifiedName(),
					resourcesToMoveOrdered[i].URN)
			}
			return fmt.Errorf(`failed to save source snapshot: %w

The resources being moved have already been appended to the destination stack, but will still also be in the
source stack.  Please remove the resources from the source stack manually using the following commands:%v
'`, err, deleteCommands)
		}
		return fmt.Errorf(`failed to save source snapshot: %w

None of the resources have been moved.  Please fix the error and try again`, err)
	}

	fmt.Fprintf(cmd.Stdout, cmd.Colorizer.Colorize(
		colors.SpecHeadline+"Successfully moved resources from %s to %s\n"+colors.Reset),
		source.Ref().FullyQualifiedName(), dest.Ref().FullyQualifiedName())

	return nil
}

func resourceMatches(res *resource.State, args []string) string {
	for _, arg := range args {
		if string(res.URN) == arg {
			return arg
		}
	}
	return ""
}

type dependencyType int

const (
	dependency dependencyType = iota
	propertyDependency
	deletedWith
)

type brokenDependency struct {
	dependencyURN  urn.URN
	dependencyType dependencyType
	propdepKey     resource.PropertyKey
	resourceURN    urn.URN
}

func breakDependencies(res *resource.State, resourcesToMove map[string]*resource.State) []brokenDependency {
	var brokenDeps []brokenDependency

	var preservedDeps []urn.URN
	preservedPropDeps := map[resource.PropertyKey][]urn.URN{}
	preservedDeletedWith := urn.URN("")

	// Providers are always moved, so we don't need to break the dependency and can ignore them here.
	_, allDeps := res.GetAllDependencies()
	for _, dep := range allDeps {
		switch dep.Type {
		case resource.ResourceParent:
			// Resources are reparented appropriately later on, so we ignore parent dependencies here.
			continue
		case resource.ResourceDependency:
			if _, ok := resourcesToMove[string(dep.URN)]; ok {
				brokenDeps = append(brokenDeps, brokenDependency{
					dependencyURN:  dep.URN,
					dependencyType: dependency,
					resourceURN:    res.URN,
				})
			} else {
				preservedDeps = append(preservedDeps, dep.URN)
			}
		case resource.ResourcePropertyDependency:
			if _, ok := resourcesToMove[string(dep.URN)]; ok {
				brokenDeps = append(brokenDeps, brokenDependency{
					dependencyURN:  dep.URN,
					dependencyType: propertyDependency,
					propdepKey:     dep.Key,
					resourceURN:    res.URN,
				})
			} else {
				preservedPropDeps[dep.Key] = append(preservedPropDeps[dep.Key], dep.URN)
			}
		case resource.ResourceDeletedWith:
			if _, ok := resourcesToMove[string(dep.URN)]; ok {
				brokenDeps = append(brokenDeps, brokenDependency{
					dependencyURN:  dep.URN,
					dependencyType: deletedWith,
					resourceURN:    res.URN,
				})
			} else {
				preservedDeletedWith = dep.URN
			}
		}
	}

	res.Dependencies = preservedDeps
	res.PropertyDependencies = preservedPropDeps
	res.DeletedWith = preservedDeletedWith

	return brokenDeps
}

func renameStackAndProject(urn urn.URN, stack backend.Stack) (urn.URN, error) {
	newURN := urn.RenameStack(stack.Ref().Name())
	if project, ok := stack.Ref().Project(); ok {
		newURN = newURN.RenameProject(tokens.PackageName(project))
	} else {
		return "", errors.New("cannot get project name. " +
			"Please upgrade your project with `pulumi state upgrade` to solve this.")
	}
	return newURN, nil
}

func rewriteURNs(res *resource.State, dest backend.Stack, rewriteMap map[string]string) error {
	var err error
	res.URN, err = renameStackAndProject(res.URN, dest)
	if err != nil {
		return err
	}

	provider, allDeps := res.GetAllDependencies()
	if provider != "" {
		if newProviderURN, ok := rewriteMap[provider]; ok {
			res.Provider = newProviderURN
		} else {
			providerURN, err := renameStackAndProject(urn.URN(provider), dest)
			if err != nil {
				return err
			}
			res.Provider = string(providerURN)
		}
	}

	var rewrittenDeps []urn.URN
	rewrittenPropDeps := map[resource.PropertyKey][]urn.URN{}

	for _, dep := range allDeps {
		rewrittenURN, err := renameStackAndProject(dep.URN, dest)
		if err != nil {
			return err
		}

		switch dep.Type {
		case resource.ResourceParent:
			res.Parent = rewrittenURN
		case resource.ResourceDependency:
			rewrittenDeps = append(rewrittenDeps, rewrittenURN)
		case resource.ResourcePropertyDependency:
			rewrittenPropDeps[dep.Key] = append(rewrittenPropDeps[dep.Key], rewrittenURN)
		case resource.ResourceDeletedWith:
			res.DeletedWith = rewrittenURN
		}
	}

	res.Dependencies = rewrittenDeps
	res.PropertyDependencies = rewrittenPropDeps

	return nil
}

func (cmd *stateMoveCmd) printBrokenDependencyRelationships(brokenDeps []brokenDependency) {
	for _, res := range brokenDeps {
		switch res.dependencyType {
		case dependency:
			fmt.Fprintf(cmd.Stdout, "  - %s has a dependency on %s\n", res.resourceURN, res.dependencyURN)
		case propertyDependency:
			fmt.Fprintf(cmd.Stdout, "  - %s (%s) has a property dependency on %s\n",
				res.resourceURN, res.propdepKey, res.dependencyURN)
		case deletedWith:
			fmt.Fprintf(cmd.Stdout, "  - %s is marked as deleted with %s\n", res.resourceURN, res.dependencyURN)
		}
	}
}
