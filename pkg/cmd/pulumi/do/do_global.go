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

package do

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// globalProjectName is the hardcoded name of the fallback project stored under PULUMI_HOME. It
// doubles as the directory name so that `pulumi do` can operate with zero setup when the user
// hasn't cd'd into a real Pulumi project.
const globalProjectName = "default-global-project"

// globalStackName is the hardcoded stack name auto-created inside the global project.
const globalStackName = "default"

// ensureGlobalProject materializes the global fallback project under $PULUMI_HOME, creating the
// directory and a minimal Pulumi.yaml on first use. Returns the loaded project and its root
// directory in the same shape as pkgWorkspace.Context.ReadProject.
func ensureGlobalProject() (*workspace.Project, string, error) {
	home, err := workspace.GetPulumiHomeDir()
	if err != nil {
		return nil, "", fmt.Errorf("resolve pulumi home: %w", err)
	}
	root := filepath.Join(home, globalProjectName)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, "", fmt.Errorf("create global project dir: %w", err)
	}
	projPath := filepath.Join(root, "Pulumi.yaml")
	if _, err := os.Stat(projPath); os.IsNotExist(err) {
		proj := &workspace.Project{Name: tokens.PackageName(globalProjectName)}
		if err := proj.Save(projPath); err != nil {
			return nil, "", fmt.Errorf("write global Pulumi.yaml: %w", err)
		}
	} else if err != nil {
		return nil, "", fmt.Errorf("stat global Pulumi.yaml: %w", err)
	}
	proj, err := workspace.LoadProject(projPath)
	if err != nil {
		return nil, "", fmt.Errorf("load global Pulumi.yaml: %w", err)
	}
	return proj, root, nil
}

// ensureGlobalStack loads (or creates on first use) the hardcoded `default` stack inside the
// global project and marks it selected in the global project's workspace settings so subsequent
// invocations resolve to the same stack via the usual CurrentStack lookup.
func ensureGlobalStack(
	ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, lm cmdBackend.LoginManager,
	proj *workspace.Project, root string, opts display.Options,
) (backend.Stack, error) {
	b, err := cmdBackend.CurrentBackend(ctx, ws, lm, proj, opts)
	if err != nil {
		return nil, fmt.Errorf("open backend for global project: %w", err)
	}
	// Look up (or fall back to) the currently selected stack for the global project. If nothing
	// has been selected yet we create the hardcoded `default` stack and remember the selection.
	if stack, err := state.CurrentStackAt(ctx, ws, b, root); err != nil {
		return nil, fmt.Errorf("read current stack for global project: %w", err)
	} else if stack != nil {
		return stack, nil
	}

	ref, err := b.ParseStackReference(globalStackName)
	if err != nil {
		return nil, fmt.Errorf("parse global stack reference: %w", err)
	}
	stack, err := b.GetStack(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("look up global stack: %w", err)
	}
	if stack == nil {
		// SetCurrent stays off: we handle selection ourselves below to target the global project.
		stack, err = cmdStack.CreateStack(ctx, sink, ws, b, ref, root, cmdStack.CreateStackOptions{})
		if err != nil {
			return nil, fmt.Errorf("create global stack: %w", err)
		}
	}
	w, err := ws.New(root)
	if err != nil {
		return nil, fmt.Errorf("record global stack selection: %w", err)
	}
	w.Settings().SetStackForBackend(state.BackendURLKey(b), stack.Ref().FullyQualifiedName().String())
	if err := w.Save(); err != nil {
		return nil, fmt.Errorf("record global stack selection: %w", err)
	}
	return stack, nil
}

// loadStackForStateful resolves the stack a stateful subcommand should operate on. For real
// projects this is the workspace's currently selected stack (via RequireStack). For the
// synthesized global project we bypass RequireStack — which reads selection state from cwd and
// won't prompt in non-interactive mode — and instead lazily create-and-select the hardcoded
// `default` stack so first-time users don't need any manual setup.
func (pc *packageCommand) loadStackForStateful(
	ctx context.Context, opts display.Options,
) (backend.Stack, error) {
	if pc.globalFallback {
		return ensureGlobalStack(ctx, pc.diagFwd, pc.ws, pc.lm, pc.proj, pc.root, opts)
	}
	stack, err := cmdStack.RequireStack(
		ctx, pc.diagFwd, pc.ws, pc.lm,
		"",                          /*stackName — use currently selected*/
		cmdStack.LoadOnly, opts, "", /*configFile*/
	)
	if err != nil {
		return nil, fmt.Errorf("load stack: %w", err)
	}
	return stack, nil
}
