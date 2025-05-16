package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		mybucket, err := s3.NewBucket(ctx, "mybucket", &s3.BucketArgs{
			Website: &*s3.BucketWebsiteArgs{
				IndexDocument: "index.html",
			},
		})
		if err != nil {
			return err
		}
		_, err = s3.NewBucketObject(ctx, "indexhtml", &s3.BucketObjectArgs{
			Bucket:      mybucket.ID(),
			Source:      pulumi.NewStringAsset("<h1>Hello, world!</h1>"),
			Acl:         "public-read",
			ContentType: "text/html",
		})
		if err != nil {
			return err
		}
		ctx.Export("bucketEndpoint", mybucket.WebsiteEndpoint.ApplyT(func(websiteEndpoint string) (string, error) {
			return fmt.Sprintf("http://%v", websiteEndpoint), nil
		}).(pulumi.StringOutput))
		return nil
	})
}
