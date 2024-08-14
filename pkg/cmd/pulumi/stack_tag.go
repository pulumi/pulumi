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

package main

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newStackTagCmd(v *viper.Viper) *cobra.Command {
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

	cmd.AddCommand(newStackTagGetCmd(v))
	cmd.AddCommand(newStackTagLsCmd(v))
	cmd.AddCommand(newStackTagRmCmd(v))
	cmd.AddCommand(newStackTagSetCmd(v))

	return cmd
}

type StackTagGetArgs struct {
	Stack string `argsShort:"s" argsUsage:"The name of the stack to operate on. Defaults to the current stack"`
}

func newStackTagGetCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Get a single stack tag value",
		Args:  cmdutil.SpecificArgs([]string{"name"}),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cmdArgs []string) error {
			args := UnmarshalArgs[StackTagGetArgs](v, cmd)

			ctx := cmd.Context()
			name := cmdArgs[0]

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}
			s, err := requireStack(ctx, args.Stack, stackLoadOnly, opts)
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

	BindFlags[StackTagGetArgs](v, cmd)

	return cmd
}

type StackTagLsArgs struct {
	Stack string `argsShort:"s" argsUsage:"The name of the stack to operate on. Defaults to the current stack"`
	JSON  bool   `argsShort:"j" argsUsage:"Emit output as JSON"`
}

func newStackTagLsCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all stack tags",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cmdArgs []string) error {
			args := UnmarshalArgs[StackTagLsArgs](v, cmd)

			ctx := cmd.Context()
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := requireStack(ctx, args.Stack, stackSetCurrent, opts)
			if err != nil {
				return err
			}

			b := s.Backend()
			if !b.SupportsTags() {
				return fmt.Errorf("the current backend (%s) does not support stack tags", b.Name())
			}

			tags := s.Tags()

			if args.JSON {
				return printJSON(tags)
			}

			printStackTags(tags)
			return nil
		}),
	}

	BindFlags[StackTagLsArgs](v, cmd)

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

	printTable(cmdutil.Table{
		Headers: []string{"NAME", "VALUE"},
		Rows:    rows,
	}, nil)
}

type StackTagRmArgs struct {
	Stack string `argsShort:"s" argsUsage:"The name of the stack to operate on. Defaults to the current stack"`
}

func newStackTagRmCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a stack tag",
		Args:  cmdutil.SpecificArgs([]string{"name"}),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cmdArgs []string) error {
			args := UnmarshalArgs[StackTagRmArgs](v, cmd)

			ctx := cmd.Context()
			name := cmdArgs[0]

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}
			s, err := requireStack(ctx, args.Stack, stackSetCurrent, opts)
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

	BindFlags[StackTagRmArgs](v, cmd)

	return cmd
}

type StackTagSetArgs struct {
	Stack string `argsShort:"s" argsUsage:"The name of the stack to operate on. Defaults to the current stack"`
}

func newStackTagSetCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <name> <value>",
		Short: "Set a stack tag",
		Args:  cmdutil.SpecificArgs([]string{"name", "value"}),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cmdArgs []string) error {
			args := UnmarshalArgs[StackTagSetArgs](v, cmd)

			ctx := cmd.Context()
			name := cmdArgs[0]
			value := cmdArgs[1]

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}
			s, err := requireStack(ctx, args.Stack, stackSetCurrent, opts)
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

	BindFlags[StackTagSetArgs](v, cmd)

	return cmd
}
