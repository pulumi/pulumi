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
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
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
func (p *applicationVersionProvider) Check(ctx context.Context,
	obj *elasticbeanstalk.ApplicationVersion) ([]error, error) {
	var failures []error
	if description := obj.Description; description != nil {
		if len(*description) > maxDescription {
			failures = append(failures,
				resource.NewFieldError(reflect.TypeOf(obj), elasticbeanstalk.ApplicationVersion_Description,
					fmt.Errorf("exceeded maximum length of %v", maxDescription)))
		}
	}
	// TODO[pulumi/lumi#220]: validate that the SourceBundle S3 bucket is in the same region as the environment.
	return failures, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *applicationVersionProvider) Create(ctx context.Context,
	obj *elasticbeanstalk.ApplicationVersion) (resource.ID, error) {
	appname, err := arn.ParseResourceName(obj.Application)
	if err != nil {
		return "", err
	}

	// Autogenerate a version label that is unique.
	var versionLabel string
	if obj.VersionLabel != nil {
		versionLabel = *obj.VersionLabel
	} else {
		versionLabel = resource.NewUniqueHex(*obj.Name+"-", maxApplicationName, sha1.Size)
	}

	// Parse out the S3 bucket and key components so we can create the source bundle.
	s3buck, s3key, err := arn.ParseResourceNamePair(obj.SourceBundle)
	if err != nil {
		return "", err
	}

	fmt.Printf("Creating ElasticBeanstalk ApplicationVersion '%v' with version label '%v'\n", *obj.Name, versionLabel)
	if _, err := p.ctx.ElasticBeanstalk().CreateApplicationVersion(
		&awselasticbeanstalk.CreateApplicationVersionInput{
			ApplicationName: aws.String(appname),
			Description:     obj.Description,
			SourceBundle: &awselasticbeanstalk.S3Location{
				S3Bucket: aws.String(s3buck),
				S3Key:    aws.String(s3key),
			},
			VersionLabel: aws.String(versionLabel),
		},
	); err != nil {
		return "", err
	}

	return arn.NewElasticBeanstalkApplicationVersionID(p.ctx.Region(), p.ctx.AccountID(), appname, versionLabel), nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *applicationVersionProvider) Get(ctx context.Context,
	id resource.ID) (*elasticbeanstalk.ApplicationVersion, error) {
	idarn, err := arn.ARN(id).Parse()
	if err != nil {
		return nil, err
	}
	appname, version := idarn.ResourceNamePair()
	resp, err := p.ctx.ElasticBeanstalk().DescribeApplicationVersions(
		&awselasticbeanstalk.DescribeApplicationVersionsInput{
			ApplicationName: aws.String(appname),
			VersionLabels:   []*string{aws.String(version)},
		},
	)
	if err != nil {
		return nil, err
	} else if len(resp.ApplicationVersions) == 0 {
		return nil, nil
	}
	contract.Assert(len(resp.ApplicationVersions) == 1)
	vers := resp.ApplicationVersions[0]
	contract.Assert(aws.StringValue(vers.ApplicationName) == appname)
	appid := arn.NewElasticBeanstalkApplication(idarn.Region, idarn.AccountID, appname)
	contract.Assert(aws.StringValue(vers.VersionLabel) == version)

	s3buck := aws.StringValue(vers.SourceBundle.S3Bucket)
	s3key := aws.StringValue(vers.SourceBundle.S3Key)
	return &elasticbeanstalk.ApplicationVersion{
		VersionLabel: vers.VersionLabel,
		Application:  resource.ID(appid),
		Description:  vers.Description,
		SourceBundle: arn.NewS3ObjectID(s3buck, s3key),
	}, nil
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *applicationVersionProvider) InspectChange(ctx context.Context, id resource.ID,
	old *elasticbeanstalk.ApplicationVersion, new *elasticbeanstalk.ApplicationVersion,
	diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *applicationVersionProvider) Update(ctx context.Context, id resource.ID,
	old *elasticbeanstalk.ApplicationVersion, new *elasticbeanstalk.ApplicationVersion,
	diff *resource.ObjectDiff) error {
	appname, version, err := arn.ParseResourceNamePair(id)
	if err != nil {
		return err
	}
	if new.Description != old.Description {
		description := new.Description
		if description == nil {
			description = aws.String("")
		}
		_, err := p.ctx.ElasticBeanstalk().UpdateApplicationVersion(&awselasticbeanstalk.UpdateApplicationVersionInput{
			ApplicationName: aws.String(appname),
			Description:     description,
			VersionLabel:    aws.String(version),
		})
		return err
	}
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *applicationVersionProvider) Delete(ctx context.Context, id resource.ID) error {
	appname, version, err := arn.ParseResourceNamePair(id)
	if err != nil {
		return err
	}
	fmt.Printf("Deleting ElasticBeanstalk ApplicationVersion '%v'\n", id)
	_, err = p.ctx.ElasticBeanstalk().DeleteApplicationVersion(
		&awselasticbeanstalk.DeleteApplicationVersionInput{
			ApplicationName: aws.String(appname),
			VersionLabel:    aws.String(version),
		},
	)
	return err
}
