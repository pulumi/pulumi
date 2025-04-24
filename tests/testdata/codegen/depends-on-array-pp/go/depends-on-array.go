package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		myBucket, err := s3.NewBucket(ctx, "myBucket", &s3.BucketArgs{
			Website: &s3.BucketWebsiteArgs{
				IndexDocument: pulumi.String("index.html"),
			},
		})
		if err != nil {
			return err
		}
		ownershipControls, err := s3.NewBucketOwnershipControls(ctx, "ownershipControls", &s3.BucketOwnershipControlsArgs{
			Bucket: myBucket.ID(),
			Rule: &s3.BucketOwnershipControlsRuleArgs{
				ObjectOwnership: pulumi.String("ObjectWriter"),
			},
		})
		if err != nil {
			return err
		}
		publicAccessBlock, err := s3.NewBucketPublicAccessBlock(ctx, "publicAccessBlock", &s3.BucketPublicAccessBlockArgs{
			Bucket:          myBucket.ID(),
			BlockPublicAcls: pulumi.Bool(false),
		})
		if err != nil {
			return err
		}
		_, err = s3.NewBucketObject(ctx, "index.html", &s3.BucketObjectArgs{
			Bucket:      myBucket.ID(),
			Source:      pulumi.NewFileAsset("./index.html"),
			ContentType: pulumi.String("text/html"),
			Acl:         pulumi.String("public-read"),
		}, pulumi.DependsOn([]pulumi.Resource{
			publicAccessBlock,
			ownershipControls,
		}))
		if err != nil {
			return err
		}
		ctx.Export("bucketName", myBucket.ID())
		ctx.Export("bucketEndpoint", myBucket.WebsiteEndpoint.ApplyT(func(websiteEndpoint string) (string, error) {
			return fmt.Sprintf("http://%v", websiteEndpoint), nil
		}).(pulumi.StringOutput))
		return nil
	})
}
