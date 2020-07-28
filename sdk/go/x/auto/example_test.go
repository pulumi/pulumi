package auto

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func Example() error {
	// This stack creates a bucket
	bucketProj := ProjectSpec{
		Name: "bucket_provider",
		// InlineSource is the function defining resources in the Pulumi project
		InlineSource: func(ctx *pulumi.Context) error {
			bucket, err := s3.NewBucket(ctx, "bucket", nil)
			if err != nil {
				return err
			}
			ctx.Export("bucketName", bucket.Bucket)
			return nil
		},
	}
	// define config to be used in the stack
	awsStackConfig := &StackOverrides{
		Config: map[string]string{"aws:region": "us-west-2"},
	}
	bucketStack := StackSpec{
		Name:      "dev_bucket",
		Project:   bucketProj,
		Overrides: awsStackConfig,
	}

	// initialize an instance of the stack
	s, err := NewStack(bucketStack)
	if err != nil {
		return err
	}

	// -- pulumi up --
	bucketRes, err := s.Up()
	if err != nil {
		return err
	}

	// This stack puts an object in the bucket created in the previous stack
	objProj := ProjectSpec{
		Name: "object_provider",
		InlineSource: func(ctx *pulumi.Context) error {
			obj, err := s3.NewBucketObject(ctx, "object", &s3.BucketObjectArgs{
				Bucket:  pulumi.String(bucketRes.Outputs["bucketName"].(string)),
				Content: pulumi.String("Hello World!"),
			})
			if err != nil {
				return err
			}
			ctx.Export("objKey", obj.Key)
			return nil
		},
	}

	objStack := StackSpec{
		Name:      "dev_obj",
		Project:   objProj,
		Overrides: awsStackConfig,
	}

	// initialize stack
	os, err := NewStack(objStack)
	if err != nil {
		return err
	}

	// -- pulumi up --
	objRes, err := os.Up()
	if err != nil {
		return err
	}
	// Success!
	fmt.Println(objRes.Summary.Result)

}
