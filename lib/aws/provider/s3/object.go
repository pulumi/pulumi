// Copyright 2017 Pulumi, Inc. All rights reserved.

package s3

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

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

const (
	Object        = tokens.Type("aws:s3/object:Object")
	ObjectIDDelim = "/" // the delimiter between bucket and key name.
)

// NewObjectProvider creates a provider that handles S3 Object operations.
func NewObjectProvider(ctx *awsctx.Context) cocorpc.ResourceProviderServer {
	return &objProvider{ctx}
}

type objProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *objProvider) Check(ctx context.Context, req *cocorpc.CheckRequest) (*cocorpc.CheckResponse, error) {
	// Read in the properties, create and validate a new group, and return the failures (if any).
	contract.Assert(req.GetType() == string(Object))
	_, _, result := unmarshalObject(req.GetProperties())
	return awsctx.NewCheckResponse(result), nil
}

// Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
// simply fetches it from the property bag; other times, the provider will assign this based on its own algorithm.
// In any case, resources with the same name must be safe to use interchangeably with one another.
func (p *objProvider) Name(ctx context.Context, req *cocorpc.NameRequest) (*cocorpc.NameResponse, error) {
	contract.Assert(req.GetType() == string(Object))
	if keyprop, has := req.GetProperties().Fields[ObjectKey]; has {
		key := resource.UnmarshalPropertyValue(keyprop)
		if key.IsString() {
			return &cocorpc.NameResponse{Name: key.StringValue()}, nil
		} else {
			return nil, fmt.Errorf(
				"Resource '%v' had a key property '%v', but it wasn't a string", Object, ObjectKey)
		}
	}
	return nil, fmt.Errorf("Resource '%v' was missing a key property '%v'", Object, ObjectKey)
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *objProvider) Create(ctx context.Context, req *cocorpc.CreateRequest) (*cocorpc.CreateResponse, error) {
	contract.Assert(req.GetType() == string(Object))

	// Read in the properties given by the request, validating as we go; if any fail, reject the request.
	obj, _, decerr := unmarshalObject(req.GetProperties())
	if decerr != nil {
		// TODO: this is a good example of a "benign" (StateOK) error; handle it accordingly.
		return nil, decerr
	}

	// Fetch the contents of the body by way of the source asset.
	body, err := obj.Source.Read()
	if err != nil {
		return nil, err
	}
	defer body.Close()

	// Now go ahead and perform the creation.
	fmt.Printf("Creating S3 Object '%v' in bucket '%v'\n", obj.Key, obj.Bucket)
	result, err := p.ctx.S3().PutObject(&s3.PutObjectInput{
		Bucket: aws.String(obj.Bucket),
		Key:    aws.String(obj.Key),
		Body:   body,
	})
	if err != nil {
		return nil, err
	}
	contract.Assert(result != nil)
	fmt.Printf("S3 Object created: %v; waiting for it to become active\n", obj.Key)

	// Wait for the object to be ready and then return the ID (just its name).
	if err = p.waitForObjectState(obj.Bucket, obj.Key, true); err != nil {
		return nil, err
	}
	return &cocorpc.CreateResponse{Id: obj.Bucket + ObjectIDDelim + obj.Key}, nil
}

// Read reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *objProvider) Read(ctx context.Context, req *cocorpc.ReadRequest) (*cocorpc.ReadResponse, error) {
	contract.Assert(req.GetType() == string(Object))
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *objProvider) Update(ctx context.Context, req *cocorpc.UpdateRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(Object))
	return nil, errors.New("Not yet implemented")
}

// UpdateImpact checks what impacts a hypothetical update will have on the resource's properties.
func (p *objProvider) UpdateImpact(
	ctx context.Context, req *cocorpc.UpdateRequest) (*cocorpc.UpdateImpactResponse, error) {
	contract.Assert(req.GetType() == string(Object))
	return nil, errors.New("Not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *objProvider) Delete(ctx context.Context, req *cocorpc.DeleteRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(Object))

	// First, perform the deletion.
	id := req.GetId()
	fmt.Printf("Deleting S3 Object '%v'\n", id)
	delim := strings.Index(id, ObjectIDDelim)
	contract.Assertf(delim != -1, "Missing object ID delimiter (`<bucket>%v<key>`)", ObjectIDDelim)
	bucket := id[:delim]
	key := id[delim+1:]
	if _, err := p.ctx.S3().DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err != nil {
		return nil, err
	}

	fmt.Printf("S3 Object delete request submitted; waiting for it to delete\n")

	// Wait for the bucket to actually become deleted before returning.
	if err := p.waitForObjectState(bucket, key, false); err != nil {
		return nil, err
	}
	return &pbempty.Empty{}, nil
}

// object represents the state associated with an S3 Object.
type object struct {
	Key    string         `json:"key"`    // the object resource's name.
	Bucket string         `json:"bucket"` // the object's bucket ID (name).
	Source resource.Asset `json:"source"` // the source asset for the bucket's contents.
}

// constants for object property names.
const (
	ObjectKey    = "key"
	ObjectBucket = "bucket"
	ObjectSource = "source"
)

// constants for the various object constraints.
const (
	maxObjectKey    = 1024
	objectKeyRegexp = "[0-9a-zA-Z!-_.*'()]"
)

// unmarshalObject decodes and validates a object property bag.
func unmarshalObject(v *pbstruct.Struct) (object, resource.PropertyMap, mapper.DecodeError) {
	var obj object
	props := resource.UnmarshalProperties(v)
	result := mapper.MapIU(props.Mappable(), &obj)
	if len(obj.Key) > maxObjectKey {
		if result == nil {
			result = mapper.NewDecodeErr(nil)
		}
		result.AddFailure(
			mapper.NewFieldErr(reflect.TypeOf(obj), ObjectKey,
				fmt.Errorf("exceeded maximum length of %v", maxObjectKey)),
		)
	}
	if match, _ := regexp.MatchString(objectKeyRegexp, obj.Key); !match {
		if result == nil {
			result = mapper.NewDecodeErr(nil)
		}
		result.AddFailure(
			mapper.NewFieldErr(reflect.TypeOf(obj), ObjectKey,
				fmt.Errorf("contains invalid characters (must match '%v')", objectKeyRegexp)))
	}
	return obj, props, result
}

func (p *objProvider) waitForObjectState(bucket string, key string, exist bool) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			if _, err := p.ctx.S3().GetObject(&s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			}); err != nil {
				if erraws, iserraws := err.(awserr.Error); iserraws {
					if erraws.Code() == "NotFound" || erraws.Code() == "NoSuchKey" {
						// The object is missing; if exist==false, we're good, otherwise keep retrying.
						return !exist, nil
					}
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
