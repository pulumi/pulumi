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
	"github.com/pulumi/lumi/pkg/util/mapper"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/elasticbeanstalk"
)

const ApplicationToken = elasticbeanstalk.ApplicationToken

// constants for the various application limits.
const (
	minApplicationName = 1
	maxApplicationName = 100
	maxDescription     = 200
)

// NewApplicationProvider creates a provider that handles ElasticBeanstalk application operations.
func NewApplicationProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &applicationProvider{ctx}
	return elasticbeanstalk.NewApplicationProvider(ops)
}

type applicationProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *applicationProvider) Check(ctx context.Context, obj *elasticbeanstalk.Application) ([]mapper.FieldError, error) {
	var failures []mapper.FieldError
	if name := obj.ApplicationName; name != nil {
		if len(*name) < minApplicationName {
			failures = append(failures,
				mapper.NewFieldErr(reflect.TypeOf(obj), elasticbeanstalk.Application_ApplicationName,
					fmt.Errorf("less than minimum length of %v", minApplicationName)))
		}
		if len(*name) > maxApplicationName {
			failures = append(failures,
				mapper.NewFieldErr(reflect.TypeOf(obj), elasticbeanstalk.Application_ApplicationName,
					fmt.Errorf("exceeded maximum length of %v", maxApplicationName)))
		}
	}
	if description := obj.Description; description != nil {
		if len(*description) > maxDescription {
			failures = append(failures,
				mapper.NewFieldErr(reflect.TypeOf(obj), elasticbeanstalk.Application_ApplicationName,
					fmt.Errorf("exceeded maximum length of %v", maxDescription)))
		}
	}
	return failures, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *applicationProvider) Create(ctx context.Context, obj *elasticbeanstalk.Application) (resource.ID, error) {
	// If an explicit name is given, use it.  Otherwise, auto-generate a name in part based on the resource name.
	var name string
	if obj.ApplicationName != nil {
		name = *obj.ApplicationName
	} else {
		name = resource.NewUniqueHex(obj.Name+"-", maxApplicationName, sha1.Size)
	}
	fmt.Printf("Creating ElasticBeanstalk Application '%v' with name '%v'\n", obj.Name, name)
	create := &awselasticbeanstalk.CreateApplicationInput{
		ApplicationName: aws.String(name),
		Description:     obj.Description,
	}
	_, err := p.ctx.ElasticBeanstalk().CreateApplication(create)
	if err != nil {
		return "", err
	}
	return arn.NewElasticBeanstalkApplicationID(p.ctx.Region(), p.ctx.AccountID(), name), nil
}

// Read reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *applicationProvider) Get(ctx context.Context, id resource.ID) (*elasticbeanstalk.Application, error) {
	return nil, errors.New("Not yet implemented")
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *applicationProvider) InspectChange(ctx context.Context, id resource.ID,
	old *elasticbeanstalk.Application, new *elasticbeanstalk.Application, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *applicationProvider) Update(ctx context.Context, id resource.ID,
	old *elasticbeanstalk.Application, new *elasticbeanstalk.Application, diff *resource.ObjectDiff) error {
	name, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}
	if new.Description != old.Description {
		description := new.Description
		if description == nil {
			// If a property (for example, description) is not provided, the value remains unchanged.
			// To clear these properties, specify an empty string.
			description = aws.String("")
		}
		_, err := p.ctx.ElasticBeanstalk().UpdateApplication(&awselasticbeanstalk.UpdateApplicationInput{
			ApplicationName: aws.String(name),
			Description:     description,
		})
		return err
	}
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *applicationProvider) Delete(ctx context.Context, id resource.ID) error {
	fmt.Printf("Deleting ElasticBeanstalk Application '%v'\n", id)
	name, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}
	if _, err := p.ctx.ElasticBeanstalk().DeleteApplication(&awselasticbeanstalk.DeleteApplicationInput{
		ApplicationName: aws.String(name),
	}); err != nil {
		return err
	}
	succ, err := awsctx.RetryUntilLong(p.ctx, func() (bool, error) {
		resp, err := p.getApplication(name)
		if err != nil {
			return false, err
		}
		if resp == nil {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	if !succ {
		return fmt.Errorf("Timed out waiting for environment to become ready")
	}
	return nil
}

func (p *applicationProvider) getApplication(name string) (*awselasticbeanstalk.ApplicationDescription, error) {
	resp, err := p.ctx.ElasticBeanstalk().DescribeApplications(&awselasticbeanstalk.DescribeApplicationsInput{
		ApplicationNames: []*string{aws.String(name)},
	})
	if err != nil {
		return nil, err
	}
	applications := resp.Applications
	if len(applications) > 1 {
		return nil, fmt.Errorf("More than one application found with name %v", name)
	}
	if len(applications) == 0 {
		return nil, nil
	}
	application := applications[0]
	return application, nil
}
