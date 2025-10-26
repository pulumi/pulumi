package show

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func ShowCmd() *cobra.Command {
	var stackName string
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
			resources := ss.Resources[2:]
			for _, resState := range resources {
				rn := string(resState.Type)
				fmt.Println(rn)
			}
			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&stackName, "stack", "", "the stack for which resources will be shown")
	cmd.MarkFlagRequired("stack")

	return cmd
}
