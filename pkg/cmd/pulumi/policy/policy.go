package policy

import policy "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/policy"

func NewPolicyCmd() *cobra.Command {
	return policy.NewPolicyCmd()
}

