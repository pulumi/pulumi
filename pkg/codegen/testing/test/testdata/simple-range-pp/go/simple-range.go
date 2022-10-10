package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		var bucket []*s3.Bucket
		for index := 0; index < 10; index++ {
			key0 := index
			val0 := index
			__res, err := s3.NewBucket(ctx, fmt.Sprintf("bucket-%v", key0), &s3.BucketArgs{
				Website: &s3.BucketWebsiteArgs{
					IndexDocument: pulumi.String(fmt.Sprintf("index-%v.html", val0)),
				},
			})
			if err != nil {
				return err
			}
			bucket = append(bucket, __res)
		}
		return nil
	})
}
