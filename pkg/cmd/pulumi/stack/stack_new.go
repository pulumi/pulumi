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

package stack

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// TODO[https://github.com/pulumi/pulumi/issues/23066]: Not yet implemented.
func newStackNewCmd() *cobra.Command {
	var (
		org             string
		environment     string
		secretsProvider string
		encryptedKey    string
		encryptionSalt  string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "new",
		Short:  "Create a new stack",
		Long: "[EXPERIMENTAL] Create a new stack.\n" +
			"\n" +
			"A stack is an isolated, independently configurable instance of a Pulumi\n" +
			"program, typically representing a deployment environment.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "project"},
			{Name: "name"},
		},
		Required: 2,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization to create the stack in")
	cmd.Flags().StringVar(&environment, "environment", "",
		"Reference to an ESC environment for storing stack configuration")
	cmd.Flags().StringVar(&secretsProvider, "secrets-provider", "",
		"The secrets provider for the stack")
	cmd.Flags().StringVar(&encryptedKey, "encrypted-key", "",
		"KMS-encrypted ciphertext for the data key (cloud-based secrets providers)")
	cmd.Flags().StringVar(&encryptionSalt, "encryption-salt", "",
		"Base64-encoded encryption salt (passphrase-based secrets providers)")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23065]: Not yet implemented.
func newStackGetCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "get",
		Short:  "Retrieve detailed information about a stack",
		Long:   "[EXPERIMENTAL] Retrieve detailed information about a stack.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}
