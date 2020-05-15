package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		logs, err := s3.NewBucket(ctx, "logs", nil)
		if err != nil {
			return err
		}
		bucket, err := s3.NewBucket(ctx, "bucket", nil)
		if err != nil {
			return err
		}
		return nil
	})
}
