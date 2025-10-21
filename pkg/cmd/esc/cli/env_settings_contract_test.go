// Copyright 2025, Pulumi Corporation.

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
	}{
		{parent: []string{"env", "set"}, subset: []string{"env", "settings", "set"}},
		{parent: []string{"env", "get"}, subset: []string{"env", "settings", "get"}},
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

			for _, flag := range subsetFlags {
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
