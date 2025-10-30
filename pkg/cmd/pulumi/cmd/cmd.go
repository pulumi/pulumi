package cmd

import cmd "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/cmd"

// Display an error to the user.
// 
// DisplayErrorMessage respects [result.IsBail].
// 
// DisplayErrorMessage adds additional error handling specific to the Pulumi CLI. This
// includes e.g. specific and more helpful messages in the case of decryption or snapshot
// integrity errors.
func DisplayErrorMessage(err error) {
	cmd.DisplayErrorMessage(err)
}

