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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func testProvider(ctx context.Context, host plugin.Host, pCtx *plugin.Context, name string) error {
	providerLocation := filepath.Join("..", name)
	// NewProviderFromPath requires a "binary", so we use a fake one. It then uses the directory for
	// that to run the plugin.
	fakeProviderBinary := filepath.Join(providerLocation, "pulumi-bin")
	prov, err := plugin.NewProviderFromPath(host, pCtx, fakeProviderBinary)
	if err != nil {
		return err
	}
	defer prov.Close()
	_, err = prov.Configure(ctx, plugin.ConfigureRequest{})
	if err != nil {
		return err
	}
	readResult, err := prov.Read(ctx, plugin.ReadRequest{
		URN:  resource.NewURN("test", "test", "", "test:index:MyResource", "testResource"),
		Type: "test:index:MyResource",
		Name: "testResource",
		ID:   "testResource",
	})
	if err != nil {
		return err
	}
	rootDirectory, err := filepath.Abs(providerLocation)
	if err != nil {
		return err
	}
	if readResult.Outputs["PULUMI_ROOT_DIRECTORY"].StringValue() != rootDirectory {
		return fmt.Errorf("expected PULUMI_ROOT_DIRECTORY to be %s, got %s", rootDirectory, readResult.Outputs["PULUMI_ROOT_DIRECTORY"])
	}
	if readResult.Outputs["PULUMI_PROGRAM_DIRECTORY"].StringValue() != rootDirectory {
		return fmt.Errorf("expected PULUMI_PROGRAM_DIRECTORY to be %s, got %s", rootDirectory, readResult.Outputs["PULUMI_PROGRAM_DIRECTORY"])
	}

	return nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		sink := cmdutil.Diag()
		pCtx, err := plugin.NewContext(sink, sink, nil, nil, wd, nil, false, nil)
		if err != nil {
			return err
		}
		host, err := plugin.NewDefaultHost(pCtx, nil, false, nil, nil, nil, tokens.PackageName("test"))
		if err != nil {
			return err
		}

		err = testProvider(ctx.Context(), host, pCtx, "provider-nodejs")
		if err != nil {
			return err
		}

		err = testProvider(ctx.Context(), host, pCtx, "provider-go")
		if err != nil {
			return err
		}

		return nil
	})
}
