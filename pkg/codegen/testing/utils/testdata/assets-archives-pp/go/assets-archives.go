package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		siteBucket, err := s3.NewBucket(ctx, "siteBucket", nil)
		if err != nil {
			return err
		}
		_, err = s3.NewBucketObject(ctx, "testFileAsset", &s3.BucketObjectArgs{
			Bucket: siteBucket.ID(),
			Source: pulumi.NewFileAsset("file.txt"),
		})
		if err != nil {
			return err
		}
		_, err = s3.NewBucketObject(ctx, "testStringAsset", &s3.BucketObjectArgs{
			Bucket: siteBucket.ID(),
			Source: pulumi.NewStringAsset("<h1>File contents</h1>"),
		})
		if err != nil {
			return err
		}
		_, err = s3.NewBucketObject(ctx, "testRemoteAsset", &s3.BucketObjectArgs{
			Bucket: siteBucket.ID(),
			Source: pulumi.NewRemoteAsset("https://pulumi.test"),
		})
		if err != nil {
			return err
		}
		_, err = lambda.NewFunction(ctx, "testFileArchive", &lambda.FunctionArgs{
			Role: siteBucket.Arn,
			Code: pulumi.NewFileArchive("file.tar.gz"),
		})
		if err != nil {
			return err
		}
		_, err = lambda.NewFunction(ctx, "testRemoteArchive", &lambda.FunctionArgs{
			Role: siteBucket.Arn,
			Code: pulumi.NewRemoteArchive("https://pulumi.test/foo.tar.gz"),
		})
		if err != nil {
			return err
		}
		_, err = lambda.NewFunction(ctx, "testAssetArchive", &lambda.FunctionArgs{
			Role: siteBucket.Arn,
			Code: pulumi.NewAssetArchive(map[string]interface{}{
				"file.txt":   pulumi.NewFileAsset("file.txt"),
				"string.txt": pulumi.NewStringAsset("<h1>File contents</h1>"),
				"remote.txt": pulumi.NewRemoteAsset("https://pulumi.test"),
				"file.tar":   pulumi.NewFileArchive("file.tar.gz"),
				"remote.tar": pulumi.NewRemoteArchive("https://pulumi.test/foo.tar.gz"),
				".nestedDir": pulumi.NewAssetArchive(map[string]interface{}{
					"file.txt":   pulumi.NewFileAsset("file.txt"),
					"string.txt": pulumi.NewStringAsset("<h1>File contents</h1>"),
					"remote.txt": pulumi.NewRemoteAsset("https://pulumi.test"),
					"file.tar":   pulumi.NewFileArchive("file.tar.gz"),
					"remote.tar": pulumi.NewRemoteArchive("https://pulumi.test/foo.tar.gz"),
				}),
			}),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
