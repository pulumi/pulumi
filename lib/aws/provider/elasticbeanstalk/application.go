// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package elasticbeanstalk

import (
	"crypto/sha1"
	"fmt"

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
func (p *applicationProvider) Check(ctx context.Context, obj *elasticbeanstalk.Application, property string) error {
	switch property {
	case elasticbeanstalk.Application_ApplicationName:
		if name := obj.ApplicationName; name != nil {
			if len(*name) < minApplicationName {
				return fmt.Errorf("less than minimum length of %v", minApplicationName)
			}
			if len(*name) > maxApplicationName {
				return fmt.Errorf("exceeded maximum length of %v", maxApplicationName)
			}
		}
	case elasticbeanstalk.Application_Description:
		if description := obj.Description; description != nil {
			if len(*description) > maxDescription {
				return fmt.Errorf("exceeded maximum length of %v", maxDescription)
			}
		}
	}
	return nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *applicationProvider) Create(ctx context.Context, obj *elasticbeanstalk.Application) (resource.ID, error) {
	// If an explicit name is given, use it.  Otherwise, auto-generate a name in part based on the resource name.
	var name string
	if obj.ApplicationName != nil {
		name = *obj.ApplicationName
	} else {
		name = resource.NewUniqueHex(*obj.Name+"-", maxApplicationName, sha1.Size)
	}
	fmt.Printf("Creating ElasticBeanstalk Application '%v' with name '%v'\n", *obj.Name, name)
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

// Query returns an (possibly empty) array of resource objects.
func (p *applicationProvider) Query(ctx context.Context) ([]*elasticbeanstalk.ApplicationItem, error) {
	return nil, nil
}

/*
	resp, err := p.ctx.ElasticBeanstalk().DescribeApplications(&awselasticbeanstalk.DescribeApplicationsInput{})
	if err != nil {
		return nil, err
	} else if len(resp.Applications) == 0 {
		return nil, nil
	}

	var apps []*elasticbeanstalk.Application
	for _, app := range resp.Applications {
		apps = append(apps, &elasticbeanstalk.Application{
			ApplicationName: app.ApplicationName,
			Description:     app.Description,
		})
	}
	return apps, nil
}
*/

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *applicationProvider) Get(ctx context.Context, id resource.ID) (*elasticbeanstalk.Application, error) {
	/*
			queresp, err := p.Query(ctx)
			if err != nil {
				return nil, err
			}
			name, err := arn.ParseResourceName(id)
			for _, app := range queresp {
				if app.ApplicationName == aws.String(name) {
					return app, nil
				}
			}
			return nil, errors.New("No resource found with matching ID")
		}
	*/
	name, err := arn.ParseResourceName(id)
	if err != nil {
		return nil, err
	}
	resp, err := p.ctx.ElasticBeanstalk().DescribeApplications(&awselasticbeanstalk.DescribeApplicationsInput{
		ApplicationNames: []*string{aws.String(name)},
	})
	if err != nil {
		return nil, err
	} else if len(resp.Applications) == 0 {
		return nil, nil
	}
	contract.Assert(len(resp.Applications) == 1)
	app := resp.Applications[0]
	contract.Assert(aws.StringValue(app.ApplicationName) == name)
	return &elasticbeanstalk.Application{
		ApplicationName: app.ApplicationName,
		Description:     app.Description,
	}, nil
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
	if _, delerr := p.ctx.ElasticBeanstalk().DeleteApplication(&awselasticbeanstalk.DeleteApplicationInput{
		ApplicationName: aws.String(name),
	}); delerr != nil {
		return delerr
	}
	succ, err := awsctx.RetryUntilLong(p.ctx, func() (bool, error) {
		fmt.Printf("Waiting for application %v to become Terminated\n", name)
		if resp, geterr := p.getApplication(name); geterr != nil {
			return false, geterr
		} else if resp == nil {
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
