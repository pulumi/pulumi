// Copyright 2024, Pulumi Corporation.
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

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingsFlagsAreSubsetOfEnvFlags(t *testing.T) {
	tests := []struct {
		parent []string
		subset []string
		// allowed lists flags that may appear on the subset command without yet
		// existing on the parent — divergences that are deliberate and tracked
		// as future work on the parent command.
		allowed []string
	}{
		{parent: []string{"env", "set"}, subset: []string{"env", "settings", "set"}},
		{
			parent:  []string{"env", "get"},
			subset:  []string{"env", "settings", "get"},
			allowed: []string{"output"}, // env get's JSON shape is being designed; settings get ships --output ahead.
		},
	}

	esc := New(&Options{})

	for _, tt := range tests {
		t.Run(tt.subset[len(tt.subset)-1], func(t *testing.T) {
			parentCmd := findCommand(esc, tt.parent)
			subsetCmd := findCommand(esc, tt.subset)

			require.NotNil(t, parentCmd)
			require.NotNil(t, subsetCmd)

			parentFlags := getFlagNames(parentCmd)
			subsetFlags := getFlagNames(subsetCmd)

			allowed := map[string]bool{}
			for _, f := range tt.allowed {
				allowed[f] = true
			}

			for _, flag := range subsetFlags {
				if allowed[flag] {
					continue
				}
				assert.Contains(t, parentFlags, flag,
					"%s has flag --%s which doesn't exist in %s. "+
						"If this is a deliberate product decision, update or remove this test.",
					strings.Join(tt.subset, " "), flag, strings.Join(tt.parent, " "))
			}
		})
	}
}

func getFlagNames(cmd *cobra.Command) []string {
	var names []string
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		names = append(names, f.Name)
	})
	return names
}

func findCommand(root *cobra.Command, path []string) *cobra.Command {
	cmd := root
	for _, part := range path {
		found := false
		for _, subCmd := range cmd.Commands() {
			if subCmd.Name() == part {
				cmd = subCmd
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	return cmd
}
