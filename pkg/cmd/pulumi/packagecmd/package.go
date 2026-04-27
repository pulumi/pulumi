// Copyright 2016, Pulumi Corporation.
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

package packagecmd

import (
	"os"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/metadata"
	"github.com/spf13/cobra"
)

func NewPackageCmd() *cobra.Command {
	long := `Work with Pulumi packages

Install and configure Pulumi packages and their plugins and SDKs.`

	if metadata.DetectAIAgent(os.Getenv) != "" {
		long += "\n\n[Agent guidance]\n" +
			"  Use `pulumi cloud api` to query the registry. Common routes:\n" +
			"    Search packages:    /api/registry/packages?search={query}\n" +
			"    List versions:      /api/registry/packages/{source}/{publisher}/{name}/versions\n" +
			"    Readme:             /api/registry/packages/{source}/{publisher}/{name}/versions/{version}/readme\n" +
			"    Nav (tokens):       /api/registry/packages/{source}/{publisher}/{name}/versions/{version}/nav\n" +
			"    Token docs:         /api/registry/packages/{source}/{publisher}/{name}/versions/{version}/docs/{percent-encoded-token}\n" +
			"  Nav lists tokens for resources, functions, and other package members; pass one to the docs route (percent-encoded).\n" +
			"  {version} can be `latest`. Set `Accept: text/markdown` or `application/json`."
	}

	cmd := &cobra.Command{
		Use:   "package",
		Short: "Work with Pulumi packages",
		Long:  long,
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(
		newExtractSchemaCommand(),
		newExtractMappingCommand(),
		newGenSdkCommand(),
		newPackagePublishSdkCmd(),
		newPackagePackSdkCmd(),
		newPackageAddCmd(),
		newPackagePublishCmd(),
		newPackageDeleteCmd(),
		newPackageInfoCmd(),
	)
	return cmd
}
