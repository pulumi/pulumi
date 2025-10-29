package show

import (
	"fmt"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func ShowCmd() *cobra.Command {
	var stackName string
	var rf resourceFilters

	cmd := &cobra.Command{
		Use:   "show",
		Short: "show resources in the stack",
		Long:  "show resources in the  stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := workspace.Instance
			ctx := cmd.Context()
			snk := cmdutil.Diag()

			s, err := stack.RequireStack(ctx, snk, ws, backend.DefaultLoginManager, stackName, stack.OfferNew, display.Options{})
			if err != nil {
				return err
			}

			// Read resources from snapshot
			ss, err := s.Snapshot(ctx, secrets.DefaultProvider)
			if err != nil {
				return err
			}
			resources := ss.Resources
			for _, r := range resources {
				passes, err := resourcePassesFilters(r, &rf)
				if err != nil {
					return err
				}
				if passes {
					printResourceState(r)
				}

			}

			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&stackName, "stack", "", "the stack for which resources will be shown")
	cmd.MarkFlagRequired("stack")
	cmd.PersistentFlags().StringVar(&rf.name, "name", "", "filter resources by name")

	return cmd
}

func printResourceState(rs *resource.State) {
	// print resource name
	fmt.Printf("ResourceName: %s\n", rs.URN.Name())
	// print URN
	fmt.Println(rs.URN)
	// print resource properties
	fmt.Println("Properties:")
	for k, v := range rs.Outputs {
		fmt.Println("	", k, ": ", v)
	}
}

func resourcePassesFilters(rs *resource.State, rf *resourceFilters) (bool, error) {
	resMatched, err := filepath.Match(rf.name, rs.URN.Name())
	if err != nil {
		return false, err
	}
	return resMatched, nil
}

type resourceFilters struct {
	name string
}
