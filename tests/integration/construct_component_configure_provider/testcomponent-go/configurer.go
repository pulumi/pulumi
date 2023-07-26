// Copyright 2016-2023, Pulumi Corporation.
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
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Configurer struct {
	pulumi.ResourceState
	AwsRegion  pulumi.StringOutput `pulumi:"awsRegion"`
	AwsProfile pulumi.StringOutput `pulumi:"awsProfile"`
}

type ConfigurerArgs struct {
	AwsRegion  pulumi.StringInput `pulumi:"awsRegion"`
	AwsProfile pulumi.StringInput `pulumi:"awsProfile"`
}

func NewConfigurer(
	ctx *pulumi.Context,
	name string,
	args *ConfigurerArgs,
	opts ...pulumi.ResourceOption,
) (*Configurer, error) {
	if args == nil {
		return nil, fmt.Errorf("args is required")
	}
	component := &Configurer{}
	err := ctx.RegisterComponentResource(configurerResourceToken, name, component, opts...)
	if err != nil {
		return nil, err
	}

	component.AwsRegion = args.AwsRegion.ToStringOutput()
	component.AwsProfile = args.AwsProfile.ToStringOutput()

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"awsRegion":  component.AwsRegion,
		"awsProfile": component.AwsProfile,
	}); err != nil {
		return nil, err
	}
	return component, nil
}

type AwsProviderArgs struct{}

type AwsProviderResult struct {
	Resource aws.ProviderOutput `pulumi:"resource"`
}

func (c *Configurer) AwsProvider(ctx *pulumi.Context, args *AwsProviderArgs) (*AwsProviderResult, error) {
	awsProv, err := aws.NewProvider(ctx, "aws-p", &aws.ProviderArgs{
		Region:  c.AwsRegion,
		Profile: c.AwsProfile,
	})
	if err != nil {
		return nil, err
	}

	return &AwsProviderResult{
		Resource: awsProv.ToProviderOutput(),
	}, nil
}
