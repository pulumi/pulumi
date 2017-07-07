// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package awsctx

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elasticbeanstalk"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/pulumi/lumi/pkg/resource/provider"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// Context represents state shared amongst all parties in this process.  In particular, it wraps an AWS session
// object and offers convenient wrappers for creating connections to the various sub-services (EC2, S3, etc).
type Context struct {
	sess *session.Session // a global session object, shared amongst all service connections.

	accountID   string // the currently authenticated account's ID.
	accountRole string // the currently authenticated account's IAM role.

	// per-service connections (lazily allocated and reused);
	apigateway       *apigateway.APIGateway
	cloudwatchlogs   *cloudwatchlogs.CloudWatchLogs
	dynamodb         *dynamodb.DynamoDB
	ec2              *ec2.EC2
	elasticbeanstalk *elasticbeanstalk.ElasticBeanstalk
	iam              *iam.IAM
	lambda           *lambda.Lambda
	s3               *s3.S3
	sns              *sns.SNS
	sts              *sts.STS
}

const regionConfig = "aws:config:region"

func New(host *provider.HostClient) (*Context, error) {
	// Create an AWS session; note that this is safe to share among many operations.
	glog.V(5).Infof("Creating a new AWS session object w/ default credentials")
	// IDEA: consider verifying credentials, region, etc. here.
	// IDEA: currently we just inherit the standard AWS SDK credentials logic; eventually we will want more
	//     flexibility, I assume, including possibly reading from configuration dynamically.
	var config []*aws.Config
	if host != nil {
		reg, err := host.ReadLocation(regionConfig)
		if err != nil {
			return nil, err
		} else if !reg.IsNull() {
			if !reg.IsString() {
				return nil, errors.Errorf("Expected a string for AWS region config '%v'; got %v", regionConfig, reg)
			}
			config = append(config, &aws.Config{Region: aws.String(reg.StringValue())})
		}
	}
	sess, err := session.NewSession(config...)
	if err != nil {
		return nil, err
	}
	contract.Assert(sess != nil)

	// Allocate the context early since we are about to use it to access the IAM service.  Its usage is inherently
	// limited until we have finished construction (in other words, completion of the present function).
	ctx := &Context{sess: sess}
	// Query the IAM service to fetch the IAM user and role information.
	glog.V(5).Infof("Querying AWS STS for profile metadata")
	identity, err := ctx.STS().GetCallerIdentity(nil)
	if err != nil {
		return nil, err
	}
	contract.Assert(identity != nil)
	ctx.accountID = aws.StringValue(identity.Account)
	ctx.accountRole = aws.StringValue(identity.Arn)
	user := aws.StringValue(identity.UserId)
	glog.V(7).Infof("AWS STS identity received: %v (id=%v role=%v)", user, ctx.accountID, ctx.accountRole)

	return ctx, nil
}

func (ctx *Context) AccountID() string { return ctx.accountID }
func (ctx *Context) Region() string    { return *ctx.sess.Config.Region }

func (ctx *Context) APIGateway() *apigateway.APIGateway {
	contract.Assert(ctx.sess != nil)
	if ctx.apigateway == nil {
		ctx.apigateway = apigateway.New(ctx.sess)
	}
	return ctx.apigateway
}

func (ctx *Context) CloudwatchLogs() *cloudwatchlogs.CloudWatchLogs {
	contract.Assert(ctx.sess != nil)
	if ctx.cloudwatchlogs == nil {
		ctx.cloudwatchlogs = cloudwatchlogs.New(ctx.sess)
	}
	return ctx.cloudwatchlogs
}

func (ctx *Context) DynamoDB() *dynamodb.DynamoDB {
	contract.Assert(ctx.sess != nil)
	if ctx.dynamodb == nil {
		ctx.dynamodb = dynamodb.New(ctx.sess)
	}
	return ctx.dynamodb
}

func (ctx *Context) EC2() *ec2.EC2 {
	contract.Assert(ctx.sess != nil)
	if ctx.ec2 == nil {
		ctx.ec2 = ec2.New(ctx.sess)
	}
	return ctx.ec2
}

func (ctx *Context) ElasticBeanstalk() *elasticbeanstalk.ElasticBeanstalk {
	contract.Assert(ctx.sess != nil)
	if ctx.elasticbeanstalk == nil {
		ctx.elasticbeanstalk = elasticbeanstalk.New(ctx.sess)
	}
	return ctx.elasticbeanstalk
}

func (ctx *Context) IAM() *iam.IAM {
	contract.Assert(ctx.sess != nil)
	if ctx.iam == nil {
		ctx.iam = iam.New(ctx.sess)
	}
	return ctx.iam
}

func (ctx *Context) Lambda() *lambda.Lambda {
	contract.Assert(ctx.sess != nil)
	if ctx.lambda == nil {
		ctx.lambda = lambda.New(ctx.sess)
	}
	return ctx.lambda
}

func (ctx *Context) S3() *s3.S3 {
	contract.Assert(ctx.sess != nil)
	if ctx.s3 == nil {
		ctx.s3 = s3.New(ctx.sess)
	}
	return ctx.s3
}

func (ctx *Context) SNS() *sns.SNS {
	contract.Assert(ctx.sess != nil)
	if ctx.sns == nil {
		ctx.sns = sns.New(ctx.sess)
	}
	return ctx.sns
}

func (ctx *Context) STS() *sts.STS {
	contract.Assert(ctx.sess != nil)
	if ctx.sts == nil {
		ctx.sts = sts.New(ctx.sess)
	}
	return ctx.sts
}

// Request manufactures a standard Golang context object for a request within this overall AWS context.
func (ctx *Context) Request() context.Context {
	// IDEA: unify this with the gRPC context; this will be easier once gRPC moves to the standard Golang context.
	return context.Background()
}
