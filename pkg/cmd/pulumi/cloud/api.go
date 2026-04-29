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

package cloud

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// apiCommand carries the state and persistent flags shared by every
// `pulumi cloud api` subcommand.
type apiCommand struct {
	// refreshSpec is the value of the persistent --refresh-spec flag.
	// True forces a re-fetch of the OpenAPI spec, bypassing the local cache.
	refreshSpec bool
}

// newAPICmd builds `pulumi cloud api` — the parent command that hosts `list`
// for browsing the API surface. The dispatcher (calling arbitrary endpoints)
// and `describe` subcommand land in follow-up PRs.
func newAPICmd() *cobra.Command {
	api := &apiCommand{}

	cmd := &cobra.Command{
		Use:   "api",
		Short: "Call any Pulumi Cloud API endpoint",
		Long: "Call any Pulumi Cloud API endpoint.\n\n" +
			"The OpenAPI spec is cached locally for 24 hours; pass --refresh-spec " +
			"on any subcommand to force a re-fetch.",
	}
	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.PersistentFlags().BoolVar(&api.refreshSpec, "refresh-spec", false,
		"Re-fetch the OpenAPI spec from Pulumi Cloud and overwrite the local cache")

	cmd.AddCommand(newLsCmd(api))

	return cmd
}
