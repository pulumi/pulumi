// Copyright 2024, Pulumi Corporation.
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

package newcmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

// GetStack gets a stack and the project name & description, or returns nil if the stack doesn't exist.
func GetStack(ctx context.Context, b backend.Backend,
	stack string, opts display.Options,
) (backend.Stack, string, string, error) {
	contract.Requiref(b != nil, "b", "must not be nil")

	stackRef, err := b.ParseStackReference(stack)
	if err != nil {
		return nil, "", "", err
	}

	s, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, "", "", err
	}

	name := ""
	description := ""
	if s != nil {
		tags := s.Tags()
		// Tags might be nil/empty, but if it has name and description use them
		name = tags[apitype.ProjectNameTag]
		description = tags[apitype.ProjectDescriptionTag]
	}

	return s, name, description, nil
}

// PromptAndCreateStack creates and returns a new stack (prompting for the name as needed).
func PromptAndCreateStack(ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context,
	b backend.Backend, prompt promptForValueFunc, stack string, root string, setCurrent bool,
	yes bool, opts display.Options, secretsProvider string, useRemoteConfig bool, configFile string,
) (backend.Stack, error) {
	contract.Requiref(b != nil, "b", "must not be nil")
	contract.Requiref(root != "", "root", "must not be empty")

	createOpts := cmdStack.CreateStackOptions{
		SetCurrent:      setCurrent,
		SecretsProvider: secretsProvider,
		UseRemoteConfig: useRemoteConfig,
		ConfigFile:      configFile,
	}
	if stack != "" {
		return createStack(ctx, sink, ws, b, stack, root, createOpts)
	}

	// Helper used by multiple commands, with no writer of its own; the blurb goes to process stdout.
	stackName, err := promptStackName(os.Stdout, b, prompt, defaultStackName, yes, opts) //nolint:forbidigo
	if err != nil {
		return nil, err
	}
	s, _, err := createStackWithRetry(ctx, sink, ws, b, prompt, stackName, root, yes, opts, createOpts)
	return s, err
}

const defaultStackName = "dev"

func promptStackName(
	w io.Writer, b backend.Backend, prompt promptForValueFunc, defaultName string,
	yes bool, opts display.Options,
) (string, error) {
	if b.SupportsOrganizations() {
		fmt.Fprint(w, "Please enter your desired stack name.\n"+
			"To create a stack in an organization, "+
			"use the format <org-name>/<stack-name> (e.g. `acmecorp/dev`).\n")
	}
	return prompt(yes, "Stack name", defaultName, false, b.ValidateStackName, opts)
}

func createStack(ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, b backend.Backend,
	stackName, root string, opts cmdStack.CreateStackOptions,
) (backend.Stack, error) {
	formatted, err := buildStackName(ctx, b, stackName)
	if err != nil {
		return nil, err
	}
	return cmdStack.InitStack(ctx, sink, ws, b, formatted, root, opts)
}

// The returned name is org-resolved; [backend.StackReference.String] elides the org when it is
// the default.
func createStackWithRetry(ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context,
	b backend.Backend, prompt promptForValueFunc, stackName, root string, yes bool,
	opts display.Options, createOpts cmdStack.CreateStackOptions,
) (backend.Stack, string, error) {
	for {
		formatted, err := buildStackName(ctx, b, stackName)
		if err != nil {
			return nil, "", err
		}
		s, err := cmdStack.InitStack(ctx, sink, ws, b, formatted, root, createOpts)
		if err == nil {
			return s, formatted, nil
		}
		if yes {
			return nil, "", err
		}
		// Let the user know about the error and loop around to try again.
		fmt.Printf("Sorry, could not create stack '%s': %v\n", stackName, err) //nolint:forbidigo
		if stackName, err = prompt(yes, "Stack name", defaultStackName, false, b.ValidateStackName, opts); err != nil {
			return nil, "", err
		}
	}
}

func buildStackName(ctx context.Context, b backend.Backend, stackName string) (string, error) {
	// If we already have a slash (e.g. org/stack, or org/proj/stack) don't add the default org.
	if strings.Contains(stackName, "/") {
		return stackName, nil
	}

	// We never have a project at the point of calling buildStackName (only called from new), so we just pass
	// nil for the project and only check the global settings.
	defaultOrg, err := b.GetDefaultOrg(ctx)
	if err != nil {
		return "", err
	}

	if defaultOrg != "" {
		return fmt.Sprintf("%s/%s", defaultOrg, stackName), nil
	}

	return stackName, nil
}
