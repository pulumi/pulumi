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
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	awsconf "methods-return-plain-resource/metaprovider"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		conf := config.New(ctx, "")
		profile := conf.Require("profile")
		region := conf.Require("region")

		configurer, err := awsconf.NewConfigurer(ctx, "configurer", &awsconf.ConfigurerArgs{
			AwsRegion:  pulumi.String(region),
			AwsProfile: pulumi.String(profile),
		})
		if err != nil {
			return err
		}

		awsProv, err := configurer.AwsProvider(ctx)
		if err != nil {
			return err
		}

		// Create an AWS resource (S3 Bucket)
		bucket, err := s3.NewBucket(ctx, "my-bucket-12709", nil, pulumi.Provider(awsProv))
		if err != nil {
			return err
		}

		ctx.Export("bucketID", bucket.ID())

		return nil
	})
}
