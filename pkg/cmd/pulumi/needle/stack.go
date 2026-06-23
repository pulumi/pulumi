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

package needle

import (
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/spf13/cobra"
)

// Set stack to the current stack.
//
// This registers the --stack/-s flag and resolves it to an existing stack (it never creates one).
func RequireStack(stack *backend.Stack, usage string) Stitch {
	return request{
		value:       requireStack,
		fulfillInto: func(s *state) { *stack = s.stack },
		payload:     usage,
	}
}

var requireStack = &value{
	deps: []*value{optionProject},
	prepare: func(cmd *cobra.Command, state *state, payload any) {
		usage := "The name of the stack to operate on. Defaults to the current stack"
		if v, _ := payload.(string); v != "" {
			usage = v
		}
		cmd.PersistentFlags().StringVarP(&state.stackFlag, "stack", "s", "", usage)
	},
	get: func(cmd *cobra.Command, state *state, _ any) error {
		s, err := cmdStack.RequireStack(
			cmd.Context(),
			state.DiagSink,
			state.WS,
			state.LM,
			state.stackFlag,
			cmdStack.LoadOnly,
			display.Options{Color: state.Color},
			"",
		)
		if err != nil {
			return err
		}

		state.stack = s
		state.backend = s.Backend()
		return nil
	},
}
