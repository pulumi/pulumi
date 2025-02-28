// Copyright 2016-2024, Pulumi Corporation.
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
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newStackTagCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage stack tags",
		Long: "Manage stack tags\n" +
			"\n" +
			"Stacks have associated metadata in the form of tags. Each tag consists of a name\n" +
			"and value. The `get`, `ls`, `rm`, and `set` commands can be used to manage tags.\n" +
			"Some tags are automatically assigned based on the environment each time a stack\n" +
			"is updated.\n",
		Args: cmdutil.NoArgs,
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "", "The name of the stack to operate on. Defaults to the current stack")

	cmd.AddCommand(newStackTagGetCmd(&stack))
	cmd.AddCommand(newStackTagLsCmd(&stack))
	cmd.AddCommand(newStackTagRmCmd(&stack))
	cmd.AddCommand(newStackTagSetCmd(&stack))

	return cmd
}

func newStackTagGetCmd(stack *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Get a single stack tag value",
		Args:  cmdutil.SpecificArgs([]string{"name"}),
		RunE: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			name := args[0]

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}
			s, err := RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				LoadOnly,
				opts,
			)
			if err != nil {
				return err
			}

			b := s.Backend()
			if !b.SupportsTags() {
				return fmt.Errorf("the current backend (%s) does not support stack tags", b.Name())
			}

			tags := s.Tags()
			if value, ok := tags[name]; ok {
				fmt.Printf("%v\n", value)
				return nil
			}

			return fmt.Errorf("stack tag '%s' not found for stack '%s'", name, s.Ref())
		}),
	}
}

func newStackTagLsCmd(stack *string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all stack tags",
		Args:  cmdutil.NoArgs,
		RunE: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				SetCurrent,
				opts,
			)
			if err != nil {
				return err
			}

			b := s.Backend()
			if !b.SupportsTags() {
				return fmt.Errorf("the current backend (%s) does not support stack tags", b.Name())
			}

			tags := s.Tags()

			if jsonOut {
				return ui.PrintJSON(tags)
			}

			printStackTags(tags)
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&jsonOut, "json", "j", false, "Emit output as JSON")

	return cmd
}

func printStackTags(tags map[apitype.StackTagName]string) {
	names := slice.Prealloc[string](len(tags))
	for n := range tags {
		names = append(names, n)
	}
	sort.Strings(names)

	rows := slice.Prealloc[cmdutil.TableRow](len(names))
	for _, name := range names {
		rows = append(rows, cmdutil.TableRow{Columns: []string{name, tags[name]}})
	}

	ui.PrintTable(cmdutil.Table{
		Headers: []string{"NAME", "VALUE"},
		Rows:    rows,
	}, nil)
}

func newStackTagRmCmd(stack *string) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a stack tag",
		Args:  cmdutil.SpecificArgs([]string{"name"}),
		RunE: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			name := args[0]

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}
			s, err := RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				SetCurrent,
				opts,
			)
			if err != nil {
				return err
			}

			b := s.Backend()
			if !b.SupportsTags() {
				return fmt.Errorf("the current backend (%s) does not support stack tags", b.Name())
			}

			tags := s.Tags()
			delete(tags, name)

			return backend.UpdateStackTags(ctx, s, tags)
		}),
	}
}

func newStackTagSetCmd(stack *string) *cobra.Command {
	return &cobra.Command{
		Use:   "set <name> <value>",
		Short: "Set a stack tag",
		Args:  cmdutil.SpecificArgs([]string{"name", "value"}),
		RunE: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			name := args[0]
			value := args[1]

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}
			s, err := RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				SetCurrent,
				opts,
			)
			if err != nil {
				return err
			}

			b := s.Backend()
			if !b.SupportsTags() {
				return fmt.Errorf("the current backend (%s) does not support stack tags", b.Name())
			}

			tags := s.Tags()
			if tags == nil {
				tags = make(map[apitype.StackTagName]string)
			}
			tags[name] = value

			return backend.UpdateStackTags(ctx, s, tags)
		}),
	}
}
