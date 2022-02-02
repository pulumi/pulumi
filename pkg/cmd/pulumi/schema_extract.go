// Copyright 2016-2022, Pulumi Corporation.
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

package main

import (
	"encoding/json"
	"io"
	"os"

	"github.com/blang/semver"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func newSchemaExtractCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract",
		Args:  cmdutil.ExactArgs(2),
		Short: "Extract a Pulumi package schema from a plugin",
		Long: `Extract a Pulumi package schema from a resource plugin.

Resolves the specified resource plugin and extracts its schema spec
in JSON format, printing it to stdout. If the plugin is not found,
attempts to install the plugin first.

To manage plugins, see:

    pulumi help plugin

For more information about package schemas, see:

https://www.pulumi.com/docs/guides/pulumi-packages/schema/
`,
		Example: "pulumi schema extract aws 4.37.1",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			packageName := args[0]
			packageVersion, err := semver.Parse(args[1])
			if err != nil {
				return err
			}
			return schemaExtract(os.Stdout, packageName, &packageVersion)
		}),
	}

	return cmd
}

func schemaExtract(writer io.Writer, packageName string, packageVersion *semver.Version) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	sink := cmdutil.Diag()

	ctx, err := plugin.NewContext(
		sink,
		sink,
		nil, /*Host*/
		nil, /*ConfigSource*/
		cwd,
		nil,  /*runtimeOptions*/
		true, /*disableProviderPreview*/
		nil,  /*opentracing.Span*/
	)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(ctx)

	pluginLoader := schema.NewPluginLoader(ctx.Host)

	pkg, err := pluginLoader.LoadPackage(packageName, packageVersion)
	if err != nil {
		return err
	}

	jsonBytes, err := pkg.MarshalJSON()
	if err != nil {
		return err
	}

	var jsonTree interface{}
	if err := json.Unmarshal(jsonBytes, &jsonTree); err != nil {
		return err
	}

	enc := json.NewEncoder(writer)
	enc.SetIndent("", "    ")
	if err := enc.Encode(jsonTree); err != nil {
		return err
	}

	return nil
}
