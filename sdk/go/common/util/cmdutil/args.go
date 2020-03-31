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

package cmdutil

import (
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/spf13/cobra"
)

// ArgsFunc wraps a standard cobra argument validator with standard Pulumi error handling.
func ArgsFunc(argsValidator cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		err := argsValidator(cmd, args)
		if err != nil {
			contract.IgnoreError(cmd.Help())
			Exit(err)
		}

		return nil
	}
}

// NoArgs is the same as cobra.NoArgs, except it is wrapped with ArgsFunc to provide standard
// Pulumi error handling.
var NoArgs = ArgsFunc(cobra.NoArgs)

// MaximumNArgs is the same as cobra.MaximumNArgs, except it is wrapped with ArgsFunc to provide standard
// Pulumi error handling.
func MaximumNArgs(n int) cobra.PositionalArgs {
	return ArgsFunc(cobra.MaximumNArgs(n))
}

// ExactArgs is the same as cobra.ExactArgs, except it is wrapped with ArgsFunc to provide standard
// Pulumi error handling.
func ExactArgs(n int) cobra.PositionalArgs {
	return ArgsFunc(cobra.ExactArgs(n))
}

// SpecificArgs requires a set of specific arguments.  We use the names to improve diagnostics.
func SpecificArgs(argNames []string) cobra.PositionalArgs {
	return ArgsFunc(func(cmd *cobra.Command, args []string) error {
		if len(args) > len(argNames) {
			return errors.Errorf("too many arguments: got %d, expected %d", len(args), len(argNames))
		} else if len(args) < len(argNames) {
			var result error
			for i := len(args); i < len(argNames); i++ {
				result = multierror.Append(result, errors.Errorf("missing required argument: %s", argNames[i]))
			}
			return result
		} else {
			return nil
		}
	})
}

// RangeArgs is the same as cobra.RangeArgs, except it is wrapped with ArgsFunc to provide standard
// Pulumi error handling.
func RangeArgs(min int, max int) cobra.PositionalArgs {
	return ArgsFunc(cobra.RangeArgs(min, max))
}
