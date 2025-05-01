// Copyright 2016-2018, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// CurrentStack reads the current stack and returns an instance connected to its backend provider.
func CurrentStack(ctx context.Context, b backend.Backend) (backend.Stack, error) {
	stackName, err := getCurrentStackName()
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
		return nil, err
	}

	return b.GetStack(ctx, ref)
}

func getCurrentStackName() (string, error) {
	// PULUMI_STACK environment variable overrides any stack name in the workspace settings
	if stackName, ok := os.LookupEnv("PULUMI_STACK"); ok {
		return stackName, nil
	}

	w, err := workspace.New()
	if err != nil {
		return "", err
	}

	return w.Settings().Stack, nil
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

// SetCurrentStack changes the current stack to the given stack name.
func SetCurrentStack(name string) error {
	// Switch the current workspace to that stack.
	w, err := workspace.New()
	if err != nil {
		return err
	}

	w.Settings().Stack = name
	return w.Save()
}
