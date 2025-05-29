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
	"errors"
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
	prov, err := plugin.NewProviderFromPath(host, pCtx, providerLocation)
	if err != nil {
		return err
	}
	defer prov.Close()
	_, err = prov.Configure(ctx, plugin.ConfigureRequest{})
	if err != nil {
		return err
	}
	constructResult, err := prov.Construct(ctx, plugin.ConstructRequest{
		Type:   "test:index:MyResource",
		Name:   "testResource",
		Inputs: resource.NewPropertyMapFromMap(map[string]any{}),
	})
	if err != nil {
		return err
	}
	if constructResult.Outputs["ITS_ALIVE"].StringValue() != "IT'S ALIVE!" {
		return errors.New("did not get expected response from provider")
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
		pCtx, err := plugin.NewContext(ctx.Context(), sink, sink, nil, nil, wd, nil, false, nil)
		if err != nil {
			return err
		}
		host, err := plugin.NewDefaultHost(pCtx, nil, false, nil, nil, nil, nil, tokens.PackageName("test"))
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

		err = testProvider(ctx.Context(), host, pCtx, "provider-python")
		if err != nil {
			return err
		}

		return nil
	})
}
