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

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// CompleteStackName is a cobra flag-completion function that lists stack names
// from the current backend, filtered to the current project when one is
// available. It is intended for use with RegisterFlagCompletionFunc on the
// `--stack` flag.
//
// Failures (unreadable project, unreachable backend, list errors) are surfaced
// to the user via cobra ActiveHelp lines so the completion menu shows why no
// candidates are offered, rather than silently dropping back to file
// completion. ActiveHelp is the only channel actually displayed by the
// generated shell completion scripts — stderr from this function is discarded.
func CompleteStackName(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return cobra.AppendActiveHelp(nil, fmt.Sprintf("cannot read project: %v", err)), cobra.ShellCompDirectiveNoFileComp
	}

	b, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, opts)
	if err != nil {
		return cobra.AppendActiveHelp(nil, fmt.Sprintf("cannot reach backend: %v", err)), cobra.ShellCompDirectiveNoFileComp
	}

	var filter backend.ListStacksFilter
	if project != nil {
		projName := string(project.Name)
		filter.Project = &projName
	}

	var names []string
	var inContToken backend.ContinuationToken
	for {
		summaries, outContToken, err := b.ListStacks(ctx, filter, inContToken)
		if err != nil {
			return cobra.AppendActiveHelp(nil, fmt.Sprintf("cannot list stacks: %v", err)), cobra.ShellCompDirectiveNoFileComp
		}
		for _, s := range summaries {
			names = append(names, s.Name().String())
		}
		if outContToken == nil {
			break
		}
		inContToken = outContToken
	}

	if len(names) == 0 {
		return cobra.AppendActiveHelp(nil, "no stacks found for this project"), cobra.ShellCompDirectiveNoFileComp
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// RegisterCompleteStack registers CompleteStackName as the completion function
// for the `--stack` flag on the given command. Callers should prefer this
// helper over calling RegisterFlagCompletionFunc directly so behavior stays
// consistent across the CLI.
func RegisterCompleteStack(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("stack", CompleteStackName)
}
