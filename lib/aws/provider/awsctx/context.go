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

package awsctx

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elasticbeanstalk"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang/glog"
	"github.com/pulumi/lumi/pkg/util/contract"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
)

// Context represents state shared amongst all parties in this process.  In particular, it wraps an AWS session
// object and offers convenient wrappers for creating connections to the various sub-services (EC2, S3, etc).
type Context struct {
	sess *session.Session // a global session object, shared amongst all service connections.

	accountID   string // the currently authenticated account's ID.
	accountRole string // the currently authenticated account's IAM role.

	// per-service connections (lazily allocated and reused);
	apigateway       *apigateway.APIGateway
	dynamodb         *dynamodb.DynamoDB
	ec2              *ec2.EC2
	elasticbeanstalk *elasticbeanstalk.ElasticBeanstalk
	iam              *iam.IAM
	lambda           *lambda.Lambda
	s3               *s3.S3
}

func New() (*Context, error) {
	// Create an AWS session; note that this is safe to share among many operations.
	glog.V(5).Infof("Creating a new AWS session object w/ default credentials")
	// TODO: consider verifying credentials, region, etc. here.
	// TODO: currently we just inherit the standard AWS SDK credentials logic; eventually we will want more
	//     flexibility, I assume, including possibly reading from configuration dynamically.
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	contract.Assert(sess != nil)

	// Allocate the context early since we are about to use it to access the IAM service.  Its usage is inherently
	// limited until we have finished construction (in other words, completion of the present function).
	ctx := &Context{sess: sess}

	// Query the IAM service to fetch the IAM user and role information.
	glog.V(5).Infof("Querying AWS IAM service for profile metadata")
	iaminfo, err := ctx.IAM().GetUser(nil)
	if err != nil {
		return nil, err
	}
	contract.Assert(iaminfo != nil)
	contract.Assert(iaminfo.User != nil)
	contract.Assert(iaminfo.User.Arn != nil)
	userARN := arn.ARN(*iaminfo.User.Arn)

	// Parse and store the ARN information on the context for convenient access.
	parsedARN, err := userARN.Parse()
	if err != nil {
		return nil, err
	}
	ctx.accountID = parsedARN.AccountID
	ctx.accountRole = parsedARN.Resource
	glog.V(7).Infof("AWS IAM profile ARN received: %v (id=%v role=%v)", userARN, ctx.accountID, ctx.accountRole)

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

// Request manufactures a standard Golang context object for a request within this overall AWS context.
func (ctx *Context) Request() context.Context {
	// TODO: unify this with the gRPC context; this will be easier once gRPC moves to the standard Golang context.
	return context.Background()
}
