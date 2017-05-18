// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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
	"reflect"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/mapper"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/s3"
)

const (
	ObjectToken   = s3.ObjectToken
	ObjectIDDelim = "/" // the delimiter between bucket and key name.
)

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
func (p *objProvider) Check(ctx context.Context, obj *s3.Object) ([]mapper.FieldError, error) {
	var failures []mapper.FieldError
	if len(obj.Key) > maxObjectKey {
		failures = append(failures,
			mapper.NewFieldErr(reflect.TypeOf(obj), s3.Object_Key,
				fmt.Errorf("exceeded maximum length of %v", maxObjectKey)))
	}
	if match, _ := regexp.MatchString(objectKeyRegexp, obj.Key); !match {
		failures = append(failures,
			mapper.NewFieldErr(reflect.TypeOf(obj), s3.Object_Key,
				fmt.Errorf("contains invalid characters (must match '%v')", objectKeyRegexp)))
	}
	return failures, nil
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
	defer body.Close()

	// Now go ahead and perform the creation.
	buck := obj.Bucket.String()
	fmt.Printf("Creating S3 Object '%v' in bucket '%v'\n", obj.Key, buck)
	if _, err := p.ctx.S3().PutObject(&awss3.PutObjectInput{
		Bucket: aws.String(buck),
		Key:    aws.String(obj.Key),
		Body:   body,
	}); err != nil {
		return "", err
	}

	// Wait for the object to be ready and then return the ID (just its name).
	fmt.Printf("S3 Object created: %v; waiting for it to become active\n", obj.Key)
	if err := p.waitForObjectState(buck, obj.Key, true); err != nil {
		return "", err
	}
	return resource.ID(buck + ObjectIDDelim + obj.Key), nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *objProvider) Get(ctx context.Context, id resource.ID) (*s3.Object, error) {
	return nil, errors.New("Not yet implemented")
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *objProvider) InspectChange(ctx context.Context, id resource.ID,
	old *s3.Object, new *s3.Object, diff *resource.ObjectDiff) ([]string, error) {
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *objProvider) Update(ctx context.Context, id resource.ID,
	old *s3.Object, new *s3.Object, diff *resource.ObjectDiff) error {
	return errors.New("Not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *objProvider) Delete(ctx context.Context, id resource.ID) error {
	// First, perform the deletion.
	fmt.Printf("Deleting S3 Object '%v'\n", id)
	ids := string(id)
	delim := strings.Index(ids, ObjectIDDelim)
	contract.Assertf(delim != -1, "Missing object ID delimiter (`<bucket>%v<key>`)", ObjectIDDelim)
	bucket := ids[:delim]
	key := ids[delim+1:]
	if _, err := p.ctx.S3().DeleteObject(&awss3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err != nil {
		return err
	}

	// Wait for the bucket to actually become deleted before returning.
	fmt.Printf("S3 Object delete request submitted; waiting for it to delete\n")
	return p.waitForObjectState(bucket, key, false)
}

func (p *objProvider) waitForObjectState(bucket string, key string, exist bool) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			if _, err := p.ctx.S3().GetObject(&awss3.GetObjectInput{
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
