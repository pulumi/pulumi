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

package adder

import (
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

const defaultStackUsage = "The name of the stack to operate on. Defaults to the current stack"

// StackFlag registers the --stack/-s flag on cmd and returns the handle that
// resolves it to an existing stack (it never creates one).
//
// Registration is idempotent, so a command and any value depending on the
// stack can both declare the flag: the first call registers it, and a
// non-empty usage from any call customizes it. Conflicting non-empty usages
// panic.
func StackFlag(cmd *cobra.Command, usage string) *StackRef {
	flags := cmd.PersistentFlags()
	f := flags.Lookup("stack")
	if f == nil {
		flags.StringP("stack", "s", "", defaultStackUsage)
		f = flags.Lookup("stack")
	}
	if usage != "" {
		contract.Assertf(f.Usage == defaultStackUsage || f.Usage == usage,
			"adder: conflicting usage strings for --stack: %q vs %q", f.Usage, usage)
		f.Usage = usage
	}
	return &StackRef{}
}

// StackRef is proof that --stack is registered; obtain one with [StackFlag].
type StackRef struct{}

// Resolve the --stack flag to an existing stack, prompting for a selection
// when the flag is unset and there is no current stack.
func (r *StackRef) Resolve(cmd *cobra.Command, e Environment) (backend.Stack, error) {
	return bagFrom(cmd).stack.get(func() (backend.Stack, error) {
		e := e.defaults(cmd)
		name, err := cmd.Flags().GetString("stack")
		if err != nil {
			return nil, err
		}
		return cmdStack.RequireStack(
			cmd.Context(),
			e.DiagSink,
			e.WS,
			e.LM,
			name,
			cmdStack.LoadOnly,
			display.Options{Color: e.Color},
			"",
		)
	})
}
