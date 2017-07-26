// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cloudwatch

import (
	"crypto/sha1"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	awscloudwatch "github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/cloudwatch"
)

const LogSubscriptionFilterToken = cloudwatch.LogSubscriptionFilterToken

// constants for the various logSubscriptionFilter limits.
const (
	maxLogSubscriptionFilterName = 512
)

// NewLogSubscriptionFilterProvider creates a provider that handles Cloudwatch LogSubscriptionFilter operations.
func NewLogSubscriptionFilterProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &logSubscriptionFilterProvider{ctx}
	return cloudwatch.NewLogSubscriptionFilterProvider(ops)
}

type logSubscriptionFilterProvider struct {
	ctx *awsctx.Context
}

func (p *logSubscriptionFilterProvider) newLogSubscriptionFilterID(logGroupName string,
	filterName string) resource.ID {
	return arn.NewResourceID("logs", p.ctx.Region(), p.ctx.AccountID(), "log-group",
		logGroupName+":subscription-filter:"+filterName)
}

func (p *logSubscriptionFilterProvider) parseLogSubscriptionFilterID(id resource.ID) (string, string, error) {
	parts, err := arn.ARN(id).Parse()
	if err != nil {
		return "", "", err
	}
	resParts := strings.Split(parts.Resource, ":")
	contract.Assert(len(resParts) == 4)
	logGroupName := resParts[1]
	filterName := resParts[3]
	return logGroupName, filterName, nil
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *logSubscriptionFilterProvider) Check(ctx context.Context, obj *cloudwatch.LogSubscriptionFilter,
	property string) error {
	return nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *logSubscriptionFilterProvider) Create(ctx context.Context,
	obj *cloudwatch.LogSubscriptionFilter) (resource.ID, error) {
	name := resource.NewUniqueHex(*obj.Name+"-", maxLogSubscriptionFilterName, sha1.Size)

	var roleArn *string
	if obj.RoleARN != nil {
		tmp := string(*obj.RoleARN)
		roleArn = &tmp
	}
	var distribution *string
	if obj.Distribution != nil {
		tmp := string(*obj.Distribution)
		distribution = &tmp
	}

	filter := &awscloudwatch.PutSubscriptionFilterInput{
		FilterName:     aws.String(name),
		LogGroupName:   aws.String(obj.LogGroupName),
		DestinationArn: aws.String(obj.DestinationArn),
		FilterPattern:  aws.String(obj.FilterPattern),
		RoleArn:        roleArn,
		Distribution:   distribution,
	}

	_, err := p.ctx.CloudwatchLogs().PutSubscriptionFilter(filter)
	if err != nil {
		return "", err
	}

	return p.newLogSubscriptionFilterID(obj.LogGroupName, name), nil
}

// Query returns an (possibly empty) array of resource objects.
func (p *logSubscriptionFilterProvider) Query(ctx context.Context) ([]*cloudwatch.LogSubscriptionFilterItem, error) {
	return nil, nil
}

/*
	var subscriptionFilters []*cloudwatch.LogSubscriptionFilter
	logs, err := p.ctx.CloudwatchLogs().DescribeLogGroups(&awscloudwatch.DescribeLogGroupsInput{})
	if err != nil {
		return nil, err
	}
	for _, group := range logs.LogGroups {
		resp, err := p.ctx.CloudwatchLogs().DescribeSubscriptionFilters(&awscloudwatch.DescribeSubscriptionFiltersInput{
			LogGroupName: group.LogGroupName,
		})
		if err != nil {
			return nil, err
		} else if resp == nil {
			return nil, errors.New("Cloudwatch query returned an empty response")
		} else if len(resp.SubscriptionFilters) == 0 {
			return nil, nil
		} else if len(resp.SubscriptionFilters) > 1 {
			return nil, errors.New("Only one subscription filter expected per log group")
		}
		filter := resp.SubscriptionFilters[0]

		var distribution *cloudwatch.LogSubscriptionDistribution
		if filter.Distribution != nil {
			tmp := cloudwatch.LogSubscriptionDistribution(*filter.Distribution)
			distribution = &tmp
		}
		var roleARN *awscommon.ARN
		if filter.RoleArn != nil {
			tmp := awscommon.ARN(*filter.RoleArn)
			roleARN = &tmp
		}

		subscriptionFilters = append(subscriptionFilters, &cloudwatch.LogSubscriptionFilter{
			LogGroupName:   *group.LogGroupName,
			DestinationArn: aws.StringValue(filter.DestinationArn),
			CreationTime:   convutil.Int64PToFloat64P(filter.CreationTime),
			Distribution:   distribution,
			RoleARN:        roleARN,
		})
	}
	return subscriptionFilters, nil
}
*/

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *logSubscriptionFilterProvider) Get(ctx context.Context,
	id resource.ID) (*cloudwatch.LogSubscriptionFilter, error) {
	/*
				queresp, err := p.Query(ctx)
				if err != nil {
					return nil, err
				}
				logGroupName, _, err := p.parseLogSubscriptionFilterID(id)
				if err != nil {
					return nil, err
				}
				for _, logsub := range queresp {
					if logsub.LogGroupName == logGroupName {
						return logsub, nil
					} // Return 'resource not found' error
				}
				return nil, nil
			}

		logGroupName, filterName, err := p.parseLogSubscriptionFilterID(id)
		if err != nil {
			return nil, err
		}

		resp, err := p.ctx.CloudwatchLogs().DescribeSubscriptionFilters(&awscloudwatch.DescribeSubscriptionFiltersInput{
			LogGroupName: aws.String(logGroupName),
		})
		if err != nil {
			return nil, err
		} else if resp == nil {
			return nil, errors.New("Cloudwatch query returned an empty response")
		} else if len(resp.SubscriptionFilters) == 0 {
			return nil, nil
		} else if len(resp.SubscriptionFilters) > 1 {
			return nil, errors.New("Only one subscription filter expected per log group")
		}
		filter := resp.SubscriptionFilters[0]
		contract.Assert(*filter.FilterName == filterName)

		var distribution *cloudwatch.LogSubscriptionDistribution
		if filter.Distribution != nil {
			tmp := cloudwatch.LogSubscriptionDistribution(*filter.Distribution)
			distribution = &tmp
		}
		var roleARN *awscommon.ARN
		if filter.RoleArn != nil {
			tmp := awscommon.ARN(*filter.RoleArn)
			roleARN = &tmp
		}
		return &cloudwatch.LogSubscriptionFilter{
			LogGroupName:   logGroupName,
			DestinationArn: aws.StringValue(filter.DestinationArn),
			CreationTime:   convutil.Int64PToFloat64P(filter.CreationTime),
			Distribution:   distribution,
			RoleARN:        roleARN,
		}, nil
	*/
	return nil, nil
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *logSubscriptionFilterProvider) InspectChange(ctx context.Context, id resource.ID,
	old *cloudwatch.LogSubscriptionFilter, new *cloudwatch.LogSubscriptionFilter,
	diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *logSubscriptionFilterProvider) Update(ctx context.Context, id resource.ID,
	old *cloudwatch.LogSubscriptionFilter, new *cloudwatch.LogSubscriptionFilter, diff *resource.ObjectDiff) error {
	contract.Failf("Not yet implemented - log subscription filter update")
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *logSubscriptionFilterProvider) Delete(ctx context.Context, id resource.ID) error {
	logGroupName, filterName, err := p.parseLogSubscriptionFilterID(id)
	if err != nil {
		return err
	}
	fmt.Printf("Deleting Cloudwatch LogSubscriptionFilter '%v'\n", id)
	_, err = p.ctx.CloudwatchLogs().DeleteSubscriptionFilter(&awscloudwatch.DeleteSubscriptionFilterInput{
		LogGroupName: aws.String(logGroupName),
		FilterName:   aws.String(filterName),
	})
	return err
}
