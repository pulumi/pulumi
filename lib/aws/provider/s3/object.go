// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package s3

import (
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/convutil"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/s3"
)

const ObjectToken = s3.ObjectToken

// constants for the various object constraints.
const (
	maxObjectKey    = 1024
	objectKeyRegexp = "[0-9a-zA-Z!-_.*'()]"
)

// NewObjectProvider creates a provider that handles S3 Object operations.
func NewObjectProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &objProvider{ctx}
	return s3.NewObjectProvider(ops)
}

type objProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *objProvider) Check(ctx context.Context, obj *s3.Object, property string) error {
	switch property {
	case s3.Object_Key:
		if len(obj.Key) > maxObjectKey {
			return fmt.Errorf("exceeded maximum length of %v", maxObjectKey)
		}
		if match, err := regexp.MatchString(objectKeyRegexp, obj.Key); err != nil {
			return err
		} else if !match {
			return fmt.Errorf("contains invalid characters (must match '%v')", objectKeyRegexp)
		}
	}
	return nil
}

// Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
// simply fetches it from the property bag; other times, the provider will assign this based on its own algorithm.
// In any case, resources with the same name must be safe to use interchangeably with one another.
func (p *objProvider) Name(ctx context.Context, obj *s3.Object) (string, error) {
	if obj.Key == "" {
		return "", errors.New("S3 Object's key was empty")
	}
	return obj.Key, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *objProvider) Create(ctx context.Context, obj *s3.Object) (resource.ID, error) {
	// Fetch the contents of the body by way of the source asset.
	body, err := obj.Source.Read()
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(body)

	// Now go ahead and perform the creation.
	buck, err := arn.ParseResourceName(obj.Bucket)
	if err != nil {
		return "", err
	}
	fmt.Printf("Creating S3 Object '%v' in bucket '%v'\n", obj.Key, buck)
	if _, err := p.ctx.S3().PutObject(&awss3.PutObjectInput{
		Bucket:             aws.String(buck),
		Key:                aws.String(obj.Key),
		Body:               body,
		ContentType:        obj.ContentType,
		ContentDisposition: obj.ContentDisposition,
		CacheControl:       obj.CacheControl,
		ContentEncoding:    obj.ContentEncoding,
		ContentLanguage:    obj.ContentLanguage,
		ContentLength:      convutil.Float64PToInt64P(obj.ContentLength),
	}); err != nil {
		return "", err
	}

	// Wait for the object to be ready and then return the ID (just its name).
	fmt.Printf("S3 Object created: %v; waiting for it to become active\n", obj.Key)
	if err := p.waitForObjectState(buck, obj.Key, true); err != nil {
		return "", err
	}
	return arn.NewS3ObjectID(buck, obj.Key), nil
}

// Query returns an (possibly empty) array of resource objects.
func (p *objProvider) Query(ctx context.Context) ([]*s3.ObjectItem, error) {
	return nil, nil
}

/*
	var objects []*s3.Object
	bucks, err := p.ctx.S3().ListBuckets(&awss3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}
	for _, buck := range bucks.Buckets {
		objs, err := p.ctx.S3().ListObjects(&awss3.ListObjectsInput{
			Bucket: buck.Name})
		if err != nil {
			return nil, err
		}
		for _, obj := range objs.Contents {
			objects = append(objects, &s3.Object{
				Bucket: resource.ID((arn.NewS3Bucket(*buck.Name))),
				Key:    *obj.Key,
			})
		}
	}
	return objects, nil
}
*/

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *objProvider) Get(ctx context.Context, id resource.ID) (*s3.Object, error) {
	/*
		queresp, err := p.Query(ctx)
		if err != nil {
			return nil, err
		}
		buck, key, err := arn.ParseResourceNamePair(id)
		if err != nil {
			return nil, err
		}
		for _, obj := range queresp {
			if string(obj.Bucket) == buck && obj.Key == key {
				return obj, nil
			}
		}
		return nil, errors.New("No resource with matching ID found")
	*/
	buck, key, err := arn.ParseResourceNamePair(id)
	if err != nil {
		return nil, err
	}
	resp, err := p.ctx.S3().GetObject(&awss3.GetObjectInput{
		Bucket: aws.String(buck),
		Key:    aws.String(key),
	})
	if err != nil {
		if awsctx.IsAWSError(err, "NotFound", "NoSuchKey") {
			return nil, nil
		}
		return nil, err
	}
	return &s3.Object{
		Bucket:             resource.ID(arn.NewS3Bucket(buck)),
		Key:                key,
		ContentType:        resp.ContentType,
		ContentDisposition: resp.ContentDisposition,
		CacheControl:       resp.CacheControl,
		ContentEncoding:    resp.ContentEncoding,
		ContentLanguage:    resp.ContentLanguage,
		ContentLength:      convutil.Int64PToFloat64P(resp.ContentLength),
	}, nil
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *objProvider) InspectChange(ctx context.Context, id resource.ID,
	old *s3.Object, new *s3.Object, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *objProvider) Update(ctx context.Context, id resource.ID,
	old *s3.Object, new *s3.Object, diff *resource.ObjectDiff) error {
	// The id is uniquely determined by `replace` properties, so update is the same as create, and we can expect
	// the resulting id to be unchanged.
	newid, err := p.Create(ctx, new)
	contract.Assert(id == newid)
	return err
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *objProvider) Delete(ctx context.Context, id resource.ID) error {
	buck, key, err := arn.ParseResourceNamePair(id)
	if err != nil {
		return err
	}

	// First, perform the deletion.
	fmt.Printf("Deleting S3 Object '%v'\n", id)
	if _, err := p.ctx.S3().DeleteObject(&awss3.DeleteObjectInput{
		Bucket: aws.String(buck),
		Key:    aws.String(key),
	}); err != nil {
		return err
	}

	// Wait for the bucket to actually become deleted before returning.
	fmt.Printf("S3 Object delete request submitted; waiting for it to delete\n")
	return p.waitForObjectState(buck, key, false)
}

func (p *objProvider) waitForObjectState(bucket string, key string, exist bool) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			if _, err := p.ctx.S3().GetObject(&awss3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			}); err != nil {
				if awsctx.IsAWSError(err, "NotFound", "NoSuchKey") {
					// The object is missing; if exist==false, we're good, otherwise keep retrying.
					return !exist, nil
				}
				return false, err // anything other than "object missing" is a real error; propagate it.
			}

			// If we got here, the object was found; if exist==true, we're good; else, keep retrying.
			return exist, nil
		},
	)
	if err != nil {
		return err
	} else if !succ {
		var reason string
		if exist {
			reason = "created"
		} else {
			reason = "deleted"
		}
		return fmt.Errorf("S3 Object '%v' in bucket '%v' did not become %v", key, bucket, reason)
	}
	return nil
}
