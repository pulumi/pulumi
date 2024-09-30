// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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

			err := pulumi.NewInputPropertiesError("failing for a reason", pulumi.InputPropertyErrorDetails{
				PropertyPath: "foo",
				Reason:       "the failure reason",
			})
			return nil, err
		},
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
