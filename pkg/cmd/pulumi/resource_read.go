// Copyright 2016-2018, Pulumi Corporation.
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
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newResourceReadCmd() *cobra.Command {

	var config []string

	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read resource",
		Args:  cmdutil.ExactArgs(4),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			packageVersion, err := semver.Parse(args[1])
			if err != nil {
				return err
			}
			return resourceRead(cmd.OutOrStdout(), config, args[0], &packageVersion, args[2], args[3])
		}),
	}

	cmd.PersistentFlags().StringArrayVarP(
		&config, "config", "c", []string{}, "Config values of the form key=value to set")

	return cmd
}

func resourceRead(writer io.Writer, config []string, packageName string, packageVersion *semver.Version, resourceType string, resourceId string) error {
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

	prov, err := ctx.Host.Provider(tokens.Package(packageName), packageVersion)
	if err != nil {
		return err
	}

	defer ctx.Host.CloseProvider(prov)

	configProperties := make(map[string]interface{})
	for _, kv := range config {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("could not parse config entry: %s", kv)
		}
		configProperties[parts[0]] = parts[1]
	}
	providerInputs := resource.NewPropertyMapFromMap(configProperties)

	providerUrn := resource.NewURN("test", "test", "", tokens.Type("pulumi:providers:"+packageName), "default")
	providerInputs, failures, err := prov.CheckConfig(providerUrn, nil, providerInputs, false)
	if err != nil {
		return err
	}

	if len(failures) != 0 {
		for _, failure := range failures {
			if failure.Property != "" {
				fprintf(writer, "Property %s has a problem: %s", failure.Property, failure.Reason)
			} else {
				fprintf(writer, "Provider has a problem: %s", failure.Reason)
			}
		}

		return fmt.Errorf("could not configure provider")
	}

	if err := prov.Configure(providerInputs); err != nil {
		return fmt.Errorf("could not configure provider: %v", err)
	}

	urn := resource.NewURN("test", "test", "", tokens.Type(resourceType), "test")
	readResult, _, err := prov.Read(urn, resource.ID(resourceId), nil, nil)
	if err != nil {
		return err
	}

	inputJson, err := json.MarshalIndent(&readResult.Inputs, "", "    ")
	if err != nil {
		return err
	}

	fprintf(writer, "%s\n", inputJson)

	return nil
}
