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
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	client "github.com/pulumi/pulumi/sdk/v3/go/esc/cloud"
)

func newEnvRotateCmd(envcmd *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rotate [<org-name>/][<project-name>/]<environment-name> [path(s) to rotate]",
		Short: "Rotate secrets in an environment",
		Long: "Rotate secrets in an environment\n" +
			"\n" +
			"Optionally accepts any number of Property Paths as additional arguments. If given any paths, will only rotate secrets at those paths.\n",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := envcmd.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, _, err := envcmd.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}

			if ref.version != "" {
				return errors.New("the rotate command does not accept environments at specific versions")
			}

			rotationPaths := []string{}
			for _, arg := range args[1:] {
				_, err := resource.ParsePropertyPath(arg)
				if err != nil {
					return fmt.Errorf("'%s' is an invalid property path: %w", arg, err)
				}
				rotationPaths = append(rotationPaths, arg)
			}

			resp, diags, err := envcmd.esc.client.RotateEnvironment(ctx, ref.orgName, ref.projectName, ref.envName, rotationPaths)
			if err != nil {
				return err
			}
			if len(diags) != 0 {
				return envcmd.writePropertyEnvironmentDiagnostics(envcmd.esc.stderr, diags)
			}
			if resp == nil {
				return nil
			}

			event := resp.SecretRotationEvent

			// Print result of rotation
			var b strings.Builder
			switch event.Status {
			case client.RotationEventSucceeded:
				fmt.Fprintf(&b, "Environment '%s' rotated.\n", args[0])
			case client.RotationEventFailed:
				if event.ErrorMessage != nil {
					fmt.Fprintf(&b, "%vError rotating: %s.%v\n", colors.SpecError, *event.ErrorMessage, colors.Reset)
				} else {
					fmt.Fprintf(&b, "%vEnvironment '%s' rotated with errors.%v\n", colors.SpecWarning, args[0], colors.Reset)
				}
			}
			if event.PostRotationRevision != nil {
				fmt.Fprintf(&b, "New revision '%d' was created.\n", *event.PostRotationRevision)
			}

			var failedRotations []client.SecretRotation
			for _, rotation := range event.Rotations {
				if rotation.Status == client.RotationFailed {
					failedRotations = append(failedRotations, rotation)
				}
			}
			if len(failedRotations) > 0 {
				fmt.Fprintf(&b, "\n%vFailed secrets:%v\n", colors.SpecError, colors.Reset)
				for _, rotation := range failedRotations {
					fmt.Fprintf(&b, "Path: %s, error: %s\n", rotation.EnvironmentPath, *rotation.ErrorMessage)
				}
			}

			fmt.Fprint(envcmd.esc.stdout, envcmd.esc.colors.Colorize(b.String()))

			return nil
		},
	}

	return cmd
}
