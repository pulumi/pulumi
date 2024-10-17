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
//go:build !all
// +build !all

package main

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	perrors "github.com/pulumi/pulumi/sdk/v3/go/pulumi/errors"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
)

const (
	providerName = "testcomponent"
	version      = "0.0.1"
)

func main() {
	if err := provider.MainWithOptions(provider.Options{
		Name:    providerName,
		Version: version,
		Construct: func(ctx *pulumi.Context, typ, name string, inputs pulumiprovider.ConstructInputs,
			options pulumi.ResourceOption,
		) (*pulumiprovider.ConstructResult, error) {
			if typ != "testcomponent:index:Component" {
				return nil, fmt.Errorf("unknown resource type %s", typ)
			}

			err := perrors.NewInputPropertiesError("failing for a reason", perrors.InputPropertyErrorDetails{
				PropertyPath: "foo",
				Reason:       "the failure reason",
			})
			return nil, err
		},
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
