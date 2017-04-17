// Copyright 2017 Pulumi, Inc. All rights reserved.

package s3

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/pulumi/coconut/pkg/resource"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
	"github.com/pulumi/coconut/pkg/util/mapper"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
	"golang.org/x/net/context"

	"github.com/pulumi/coconut/lib/aws/provider/awsctx"
)

const Bucket = tokens.Type("aws:s3/bucket:Bucket")

// NewBucketProvider creates a provider that handles S3 bucket operations.
func NewBucketProvider(ctx *awsctx.Context) cocorpc.ResourceProviderServer {
	return &buckProvider{ctx}
}

type buckProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *buckProvider) Check(ctx context.Context, req *cocorpc.CheckRequest) (*cocorpc.CheckResponse, error) {
	// Read in the properties, create and validate a new group, and return the failures (if any).
	contract.Assert(req.GetType() == string(Bucket))
	_, _, result := unmarshalBucket(req.GetProperties())
	return awsctx.NewCheckResponse(result), nil
}

// Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
// simply fetches it from the property bag; other times, the provider will assign this based on its own algorithm.
// In any case, resources with the same name must be safe to use interchangeably with one another.
func (p *buckProvider) Name(ctx context.Context, req *cocorpc.NameRequest) (*cocorpc.NameResponse, error) {
	return nil, nil // use the AWS provider default name
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *buckProvider) Create(ctx context.Context, req *cocorpc.CreateRequest) (*cocorpc.CreateResponse, error) {
	contract.Assert(req.GetType() == string(Bucket))

	// Read in the properties given by the request, validating as we go; if any fail, reject the request.
	buck, _, decerr := unmarshalBucket(req.GetProperties())
	if decerr != nil {
		// TODO: this is a good example of a "benign" (StateOK) error; handle it accordingly.
		return nil, decerr
	}

	// If an explicit name is given, use it.  Otherwise, auto-generate a name in part based on the resource name.
	// TODO: use the URN, not just the name, to enhance global uniqueness.
	// TODO: even for explicit names, we should consider mangling it somehow, to reduce multi-instancing conflicts.
	var id string
	if buck.BucketName != nil {
		id = *buck.BucketName
	} else {
		id = resource.NewUniqueHex(buck.Name+"-", maxBucketName, sha1.Size)
	}
	fmt.Printf("Creating S3 Bucket '%v' with name '%v'\n", buck.Name, id)
	create := &s3.CreateBucketInput{
		Bucket: aws.String(id),
		ACL:    buck.AccessControl,
	}

	// Now go ahead and perform the action.
	result, err := p.ctx.S3().CreateBucket(create)
	if err != nil {
		return nil, err
	}
	contract.Assert(result != nil)
	fmt.Printf("S3 Bucket created: %v; waiting for it to become active\n", id)

	// Wait for the bucket to be ready and then return the ID (just its name).
	if err = p.waitForBucketState(&id, true); err != nil {
		return nil, err
	}
	return &cocorpc.CreateResponse{Id: id}, nil
}

// Read reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *buckProvider) Read(ctx context.Context, req *cocorpc.ReadRequest) (*cocorpc.ReadResponse, error) {
	contract.Assert(req.GetType() == string(Bucket))
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *buckProvider) Update(ctx context.Context, req *cocorpc.UpdateRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(Bucket))
	return nil, errors.New("Not yet implemented")
}

// UpdateImpact checks what impacts a hypothetical update will have on the resource's properties.
func (p *buckProvider) UpdateImpact(
	ctx context.Context, req *cocorpc.UpdateRequest) (*cocorpc.UpdateImpactResponse, error) {
	contract.Assert(req.GetType() == string(Bucket))
	return nil, errors.New("Not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *buckProvider) Delete(ctx context.Context, req *cocorpc.DeleteRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(Bucket))

	// First, perform the deletion.
	id := aws.String(req.GetId())
	fmt.Printf("Deleting S3 Bucket '%v'\n", *id)
	if _, err := p.ctx.S3().DeleteBucket(&s3.DeleteBucketInput{
		Bucket: id,
	}); err != nil {
		return nil, err
	}

	fmt.Printf("S3 Bucket delete request submitted; waiting for it to delete\n")

	// Wait for the bucket to actually become deleted before returning.
	if err := p.waitForBucketState(id, false); err != nil {
		return nil, err
	}
	return &pbempty.Empty{}, nil
}

// bucket represents the state associated with an S3 bucket.
type bucket struct {
	Name          string  `json:"name"`                    // the bucket resource's name.
	BucketName    *string `json:"bucketName,omitempty"`    // the bucket's published name.
	AccessControl *string `json:"accessControl,omitempty"` // a canned ACL to grant to this bucket.
}

// constants for bucket property names.
const (
	BucketName          = "name"
	BucketBucketName    = "bucketName"
	BucketAccessControl = "accessControl"
)

// constants for the various bucket limits.
const (
	minBucketName = 3
	maxBucketName = 63 // TODO: consider supporting legacy us-east-1 (255) limits.
)

// unmarshalBucket decodes and validates a bucket property bag.
func unmarshalBucket(v *pbstruct.Struct) (bucket, resource.PropertyMap, mapper.DecodeError) {
	var buck bucket
	props := resource.UnmarshalProperties(v)
	result := mapper.MapIU(props.Mappable(), &buck)
	if name := buck.BucketName; name != nil {
		if len(*name) < minBucketName {
			if result == nil {
				result = mapper.NewDecodeErr(nil)
			}
			result.AddFailure(
				mapper.NewFieldErr(reflect.TypeOf(buck), BucketBucketName,
					fmt.Errorf("less than minimum length of %v", minBucketName)),
			)
		} else if len(*name) > maxBucketName {
			if result == nil {
				result = mapper.NewDecodeErr(nil)
			}
			result.AddFailure(
				mapper.NewFieldErr(reflect.TypeOf(buck), BucketBucketName,
					fmt.Errorf("exceeded maximum length of %v", maxBucketName)),
			)
		}
	}
	// TODO: by default, only up to 100 buckets in an account.
	// TODO: check the vailidity of names (see http://docs.aws.amazon.com/AmazonS3/latest/dev/BucketRestrictions.html).
	// TODO: check the validity of the access control field.
	return buck, props, result
}

func (p *buckProvider) waitForBucketState(name *string, exist bool) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			if _, err := p.ctx.S3().HeadBucket(&s3.HeadBucketInput{
				Bucket: name,
			}); err != nil {
				if erraws, iserraws := err.(awserr.Error); iserraws {
					if erraws.Code() == "NotFound" || erraws.Code() == "NoSuchBucket" {
						// The bucket is missing; if exist==false, we're good, otherwise keep retrying.
						return !exist, nil
					}
				}
				return false, err // anything other than "bucket missing" is a real error; propagate it.
			}

			// If we got here, the bucket was found; if exist==true, we're good; else, keep retrying.
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
		return fmt.Errorf("S3 bucket '%v' did not become %v", *name, reason)
	}
	return nil
}
