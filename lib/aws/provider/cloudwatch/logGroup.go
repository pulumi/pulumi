// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cloudwatch

import (
	"crypto/sha1"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	awscloudwatch "github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/pkg/errors"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/convutil"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/cloudwatch"
)

const LogGroupToken = cloudwatch.LogGroupToken

// constants for the various logGroup limits.
const (
	minLogGroupName = 1
	maxLogGroupName = 512
)

var (
	logGroupNameRegexp = regexp.MustCompile(`[\.\-_/#A-Za-z0-9]+`)
)

// NewLogGroupProvider creates a provider that handles Cloudwatch LogGroup operations.
func NewLogGroupProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &logGroupProvider{ctx}
	return cloudwatch.NewLogGroupProvider(ops)
}

type logGroupProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *logGroupProvider) Check(ctx context.Context, obj *cloudwatch.LogGroup,
	property string) error {
	switch property {
	case cloudwatch.LogGroup_LogGroupName:
		if obj.LogGroupName != nil {
			if len(*obj.LogGroupName) < minLogGroupName {
				return fmt.Errorf("less than minimum length of %v", minLogGroupName)
			} else if len(*obj.LogGroupName) > maxLogGroupName {
				return fmt.Errorf("exceeded the maximum length of %v", maxLogGroupName)
			} else if !logGroupNameRegexp.MatchString(*obj.LogGroupName) {
				return fmt.Errorf("contains invalid characters (must match '%v')", logGroupNameRegexp)
			}
		}
	case cloudwatch.LogGroup_RetentionInDays:
		if obj.RetentionInDays != nil {
			switch int(*obj.RetentionInDays) {
			case 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653:
			default:
				return fmt.Errorf("not an allowed value for retentionInDays '%v'", int(*obj.RetentionInDays))
			}
		}
	}
	return nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *logGroupProvider) Create(ctx context.Context,
	obj *cloudwatch.LogGroup) (resource.ID, error) {
	// If an explicit name is given, use it.  Otherwise, auto-generate a name in part based on the resource name.
	var name string
	if obj.LogGroupName != nil {
		name = *obj.LogGroupName
	} else {
		name = resource.NewUniqueHex(*obj.Name+"-", maxLogGroupName, sha1.Size)
	}

	_, err := p.ctx.CloudwatchLogs().CreateLogGroup(&awscloudwatch.CreateLogGroupInput{
		LogGroupName: aws.String(name),
	})
	if err != nil {
		return "", err
	}

	if obj.RetentionInDays != nil {
		_, err := p.ctx.CloudwatchLogs().PutRetentionPolicy(&awscloudwatch.PutRetentionPolicyInput{
			LogGroupName:    aws.String(name),
			RetentionInDays: convutil.Float64PToInt64P(obj.RetentionInDays),
		})
		if err != nil {
			return "", err
		}
	}

	return arn.NewResourceID("logs", p.ctx.Region(), p.ctx.AccountID(), "log-group", name), nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *logGroupProvider) Get(ctx context.Context,
	id resource.ID) (*cloudwatch.LogGroup, error) {
	logGroupName, err := arn.ParseResourceName(id)
	if err != nil {
		return nil, err
	}

	resp, err := p.ctx.CloudwatchLogs().DescribeLogGroups(&awscloudwatch.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(logGroupName),
	})
	if err != nil {
		return nil, err
	} else if resp == nil {
		return nil, errors.New("Cloudwatch query returned an empty response")
	}
	var logGroup *awscloudwatch.LogGroup
	for _, group := range resp.LogGroups {
		if *group.LogGroupName == logGroupName {
			logGroup = group
		}
	}
	if logGroup == nil {
		return nil, nil
	}

	return &cloudwatch.LogGroup{
		LogGroupName:    aws.String(logGroupName),
		RetentionInDays: convutil.Int64PToFloat64P(logGroup.RetentionInDays),
	}, nil
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *logGroupProvider) InspectChange(ctx context.Context, id resource.ID,
	old *cloudwatch.LogGroup, new *cloudwatch.LogGroup,
	diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *logGroupProvider) Update(ctx context.Context, id resource.ID,
	old *cloudwatch.LogGroup, new *cloudwatch.LogGroup, diff *resource.ObjectDiff) error {
	logGroupName, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}
	if diff.Changed(cloudwatch.LogGroup_RetentionInDays) {
		if new.RetentionInDays != nil {
			_, err := p.ctx.CloudwatchLogs().PutRetentionPolicy(&awscloudwatch.PutRetentionPolicyInput{
				LogGroupName:    aws.String(logGroupName),
				RetentionInDays: convutil.Float64PToInt64P(new.RetentionInDays),
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *logGroupProvider) Delete(ctx context.Context, id resource.ID) error {
	logGroupName, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}

	fmt.Printf("Deleting Cloudwatch LogGroup '%v'\n", id)
	_, err = p.ctx.CloudwatchLogs().DeleteLogGroup(&awscloudwatch.DeleteLogGroupInput{
		LogGroupName: aws.String(logGroupName),
	})
	if err != nil {
		return err
	}

	return nil
}
