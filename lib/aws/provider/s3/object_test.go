// Copyright 2016-2017, Pulumi Corporation
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

package s3

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/provider/testutil"
	"github.com/pulumi/lumi/lib/aws/rpc/s3"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	t.Parallel()

	prefix := resource.NewUniqueHex("lumitest", 20, 20)
	ctx := testutil.CreateContext(t)
	defer func() {
		buckerr := cleanupBucket(prefix, ctx)
		assert.Nil(t, buckerr)
	}()
	str1 := "<h1>Hello world!</h1>"
	str2 := `{"hello": "world"}`
	source1 := resource.NewTextAsset(str1)
	source2 := resource.NewTextAsset(str2)

	resources := map[string]testutil.Resource{
		"bucket": {Provider: NewBucketProvider(ctx), Token: BucketToken},
		"object": {Provider: NewObjectProvider(ctx), Token: ObjectToken},
	}
	steps := []testutil.Step{
		// Create a bucket and object
		{
			testutil.ResourceGenerator{
				Name: "bucket",
				Creator: func(ctx testutil.Context) interface{} {
					return &s3.Bucket{
						Name: aws.String(prefix),
					}
				},
			},
			testutil.ResourceGenerator{
				Name: "object",
				Creator: func(ctx testutil.Context) interface{} {
					return &s3.Object{
						Bucket:      ctx.GetResourceID("bucket"),
						Key:         prefix,
						Source:      &source1,
						ContentType: aws.String("text/html"),
					}
				},
			},
		},
		// Update the object with new `source` content
		{
			testutil.ResourceGenerator{
				Name: "object",
				Creator: func(ctx testutil.Context) interface{} {
					return &s3.Object{
						Bucket:      ctx.GetResourceID("bucket"),
						Key:         prefix,
						Source:      &source2,
						ContentType: aws.String("application/json"),
					}
				},
			},
		},
	}

	props := testutil.ProviderTest(t, resources, steps)

	assert.Equal(t, "application/json", props["object"].Fields["contentType"].GetStringValue())
	assert.Equal(t, len(str2), int(props["object"].Fields["contentLength"].GetNumberValue()),
		"expected object content-length to equal len(%q)", str2)
}

func cleanupBucket(prefix string, ctx *awsctx.Context) error {
	fmt.Printf("Cleaning up buckets with name prefix:%v\n", prefix)
	list, err := ctx.S3().ListBuckets(&awss3.ListBucketsInput{})
	if err != nil {
		return err
	}
	cleaned := 0
	for _, buck := range list.Buckets {
		if strings.HasPrefix(aws.StringValue(buck.Name), prefix) {
			objList, err := ctx.S3().ListObjects(&awss3.ListObjectsInput{
				Bucket: buck.Name,
			})
			if err != nil {
				return err
			}
			for _, obj := range objList.Contents {
				_, err = ctx.S3().DeleteObject(&awss3.DeleteObjectInput{
					Bucket: buck.Name,
					Key:    obj.Key,
				})
				if err != nil {
					return err
				}
			}
			_, err = ctx.S3().DeleteBucket(&awss3.DeleteBucketInput{
				Bucket: buck.Name,
			})
			if err != nil {
				return err
			}
			cleaned++
		}
	}
	fmt.Printf("Cleaned up %v buckets\n", cleaned)
	return nil
}
