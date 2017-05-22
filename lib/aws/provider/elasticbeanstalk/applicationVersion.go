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

package elasticbeanstalk

import (
	"crypto/sha1"
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	awselasticbeanstalk "github.com/aws/aws-sdk-go/service/elasticbeanstalk"
	"github.com/pkg/errors"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/mapper"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"strings"

	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/elasticbeanstalk"
)

const ApplicationVersionToken = elasticbeanstalk.ApplicationVersionToken

// NewApplicationVersionProvider creates a provider that handles ElasticBeanstalk applicationVersion operations.
func NewApplicationVersionProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &applicationVersionProvider{ctx}
	return elasticbeanstalk.NewApplicationVersionProvider(ops)
}

type applicationVersionProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *applicationVersionProvider) Check(ctx context.Context, obj *elasticbeanstalk.ApplicationVersion) ([]mapper.FieldError, error) {
	var failures []mapper.FieldError
	if description := obj.Description; description != nil {
		if len(*description) > maxDescription {
			failures = append(failures,
				mapper.NewFieldErr(reflect.TypeOf(obj), elasticbeanstalk.ApplicationVersion_Description,
					fmt.Errorf("exceeded maximum length of %v", maxDescription)))
		}
	}
	// TODO: The SourceBundle S3 bucket must be in the same region as the environment
	return failures, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *applicationVersionProvider) Create(ctx context.Context, obj *elasticbeanstalk.ApplicationVersion) (resource.ID, error) {
	// TODO: use the URN, not just the name, to enhance global uniqueness.
	versionLabel := resource.NewUniqueHex(obj.Name+"-", maxApplicationName, sha1.Size)

	s3ObjectID := obj.SourceBundle.String()
	s3Parts := strings.SplitN(s3ObjectID, "/", 2)
	contract.Assertf(len(s3Parts) == 2, "Expected S3 Object resource ID to be of the form <bucket>/<key>")
	create := &awselasticbeanstalk.CreateApplicationVersionInput{
		ApplicationName: obj.Application.StringPtr(),
		Description:     obj.Description,
		SourceBundle: &awselasticbeanstalk.S3Location{
			S3Bucket: aws.String(s3Parts[0]),
			S3Key:    aws.String(s3Parts[1]),
		},
		VersionLabel: aws.String(versionLabel),
	}
	fmt.Printf("Creating ElasticBeanstalk ApplicationVersion '%v' with version label '%v'\n", obj.Name, versionLabel)
	_, err := p.ctx.ElasticBeanstalk().CreateApplicationVersion(create)
	if err != nil {
		return "", err
	}
	return resource.ID(versionLabel), nil
}

// Read reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *applicationVersionProvider) Get(ctx context.Context, id resource.ID) (*elasticbeanstalk.ApplicationVersion, error) {
	// TODO: Can almost just use p.getApplicationVersion to implement this, but there is no way to get the `resource.ID`
	// for the SourceBundle S3 object returned from the AWS API.
	return nil, errors.New("Not yet implemented")
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *applicationVersionProvider) InspectChange(ctx context.Context, id resource.ID,
	old *elasticbeanstalk.ApplicationVersion, new *elasticbeanstalk.ApplicationVersion, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *applicationVersionProvider) Update(ctx context.Context, id resource.ID,
	old *elasticbeanstalk.ApplicationVersion, new *elasticbeanstalk.ApplicationVersion, diff *resource.ObjectDiff) error {
	if new.Description != old.Description {
		description := new.Description
		if description == nil {
			description = aws.String("")
		}
		_, err := p.ctx.ElasticBeanstalk().UpdateApplicationVersion(&awselasticbeanstalk.UpdateApplicationVersionInput{
			ApplicationName: new.Application.StringPtr(),
			Description:     description,
			VersionLabel:    id.StringPtr(),
		})
		return err
	}
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *applicationVersionProvider) Delete(ctx context.Context, id resource.ID) error {
	applicationVersion, err := p.getApplicationVersion(id)
	if err != nil {
		return err
	}
	fmt.Printf("Deleting ElasticBeanstalk ApplicationVersion '%v'\n", id)
	_, err = p.ctx.ElasticBeanstalk().DeleteApplicationVersion(&awselasticbeanstalk.DeleteApplicationVersionInput{
		ApplicationName: applicationVersion.ApplicationName,
		VersionLabel:    id.StringPtr(),
	})
	return err
}

func (p *applicationVersionProvider) getApplicationVersion(id resource.ID) (*awselasticbeanstalk.ApplicationVersionDescription, error) {
	resp, err := p.ctx.ElasticBeanstalk().DescribeApplicationVersions(&awselasticbeanstalk.DescribeApplicationVersionsInput{
		VersionLabels: []*string{id.StringPtr()},
	})
	if err != nil {
		return nil, err
	}
	applicationVersions := resp.ApplicationVersions
	if len(applicationVersions) > 1 {
		return nil, fmt.Errorf("More than one application version found with version label %v", id.String())
	} else if len(applicationVersions) == 0 {
		return nil, fmt.Errorf("No application version found with version label %v", id.String())
	}
	applicationVersion := applicationVersions[0]
	return applicationVersion, nil
}
