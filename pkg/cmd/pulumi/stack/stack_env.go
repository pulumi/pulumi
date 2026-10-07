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

package stack

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// DefaultStackEnvironmentDefinition is the inline environment definition a stack created with
// --esc-config starts with: an empty bag of stack configuration.
const DefaultStackEnvironmentDefinition = "values:\n  pulumiConfig: {}\n"

// CheckStackEnvironmentSupport reports whether a backend can host the environment a stack manages.
func CheckStackEnvironmentSupport(b backend.Backend) error {
	if _, ok := b.(backend.StackEnvironmentsBackend); !ok {
		return fmt.Errorf("backend %v does not support stack-managed environments; they require the "+
			"Pulumi Cloud backend, use `pulumi login` without arguments to log into the Pulumi Cloud backend", b.Name())
	}
	return nil
}

// InitStackEnvironment gives a newly created stack an inline environment definition, unless its
// configuration file already has one, and publishes it, which creates the environment
// `<project>/<stack>` the stack manages.
func InitStackEnvironment(
	ctx context.Context,
	stdout io.Writer,
	stack backend.Stack,
	ps *workspace.ProjectStack,
	configFile string,
) error {
	syncer, ok := stack.Backend().(backend.StackEnvironmentsBackend)
	if !ok {
		return CheckStackEnvironmentSupport(stack.Backend())
	}

	if !ps.Environment.IsDefinition() {
		if ps.Environment != nil {
			return errors.New("the stack's configuration file imports environments; replace the list with an " +
				"inline environment definition to use --esc-config")
		}
		env, err := workspace.NewEnvironmentDefinition([]byte(DefaultStackEnvironmentDefinition))
		if err != nil {
			return err
		}
		ps.Environment = env
		if err := SaveProjectStack(ctx, stack, ps, configFile); err != nil {
			return fmt.Errorf("saving stack config: %w", err)
		}
	}

	res, err := syncer.SyncStackEnvironment(ctx, stack, ps.EnvironmentBytes(), backend.StackEnvironmentSyncOptions{
		Duration: time.Hour,
	})
	if errors.Is(err, backend.ErrStackEnvironmentSyncUnsupported) {
		return errors.New("the backend does not support stack-managed environments; " +
			"upgrade the Pulumi Cloud service or drop --esc-config")
	}
	if err != nil {
		return fmt.Errorf("creating the environment for stack %s: %w", stack.Ref(), err)
	}
	if len(res.Diagnostics) != 0 {
		return fmt.Errorf("creating environment %s: %w", res.Environment, res.Diagnostics)
	}

	if stdout != nil {
		fmt.Fprintf(stdout, "Created environment %s for stack %s; edit the 'environment' block of the stack's "+
			"configuration file and run `pulumi up` to publish changes\n", res.Environment, stack.Ref())
	}
	return nil
}
