// Copyright 2016, Pulumi Corporation.
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

package state

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

// CurrentStack reads the current stack and returns an instance connected to its backend provider.
func CurrentStack(ctx context.Context, ws pkgWorkspace.Context, b backend.Backend) (backend.Stack, error) {
	return CurrentStackAt(ctx, ws, b, "")
}

// CurrentStackAt is like CurrentStack, but resolves the workspace (and thus the selected stack)
// from dir instead of the process working directory. An empty dir means the process working
// directory.
func CurrentStackAt(
	ctx context.Context, ws pkgWorkspace.Context, b backend.Backend, dir string,
) (backend.Stack, error) {
	stackName, fromLegacy, err := getCurrentStackName(ws, dir, b.URL())
	if err != nil {
		return nil, err
	} else if stackName == "" {
		return nil, nil
	}

	qualifiedStackName, err := getStackNameWithLegacyOrgNameIfNeeded(b, stackName)
	if err != nil {
		return nil, err
	}

	ref, err := b.ParseStackReference(qualifiedStackName)
	if err != nil {
		// A legacy unscoped workspace selection may belong to a different backend
		// (for example a cloud FQN after `pulumi login --local`). Treat that as
		// unset for this backend. Explicit per-backend selections and PULUMI_STACK
		// still surface parse errors.
		if fromLegacy {
			return nil, nil
		}
		return nil, err
	}

	return b.GetStack(ctx, ref)
}

func getCurrentStackName(ws pkgWorkspace.Context, dir, backendURL string) (name string, fromLegacy bool, err error) {
	// PULUMI_STACK environment variable overrides any stack name in the workspace settings
	if stackName, ok := os.LookupEnv("PULUMI_STACK"); ok {
		return stackName, false, nil
	}

	// An empty dir resolves to the process working directory inside ws.New.
	w, err := ws.New(dir)
	if err != nil {
		return "", false, err
	}

	name, fromLegacy = w.Settings().StackForBackend(backendURL)
	return name, fromLegacy, nil
}

// Potentially qualifies a stack name with the username as the org, if orgs are supported by the backend.
// Earlier versions of the Pulumi CLI did not always store the current selected stack with the fully qualified
// stack name. Ensure backwards compatibility for these users when they upgrade that we qualify with the
// correct org name.
func getStackNameWithLegacyOrgNameIfNeeded(b backend.Backend, stackName string) (string, error) {
	// Check if only stack name is configured.
	split := strings.Split(stackName, "/")
	if len(split) == 1 {
		// If so, see if we should qualify the stack with legacy default org behavior:
		fallbackOrg, err := backend.GetLegacyDefaultOrgFallback(b, nil)
		if err != nil {
			return "", err
		}
		if fallbackOrg != "" {
			return fmt.Sprintf("%s/%s", fallbackOrg, stackName), nil
		}
	}

	return stackName, nil
}

// SetCurrentStack changes the current stack for the given backend URL.
func SetCurrentStack(ws pkgWorkspace.Context, backendURL, name string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current working directory: %w", err)
	}

	// Switch the current workspace to that stack.
	w, err := ws.New(cwd)
	if err != nil {
		return err
	}

	w.Settings().SetStackForBackend(backendURL, name)
	return w.Save()
}
