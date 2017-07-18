// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package lambda

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	awsiam "github.com/aws/aws-sdk-go/service/iam"
	awslambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	cloudwatchprovider "github.com/pulumi/lumi/lib/aws/provider/cloudwatch"
	iamprovider "github.com/pulumi/lumi/lib/aws/provider/iam"
	snsprovider "github.com/pulumi/lumi/lib/aws/provider/sns"
	"github.com/pulumi/lumi/lib/aws/provider/testutil"
	rpc "github.com/pulumi/lumi/lib/aws/rpc"
	"github.com/pulumi/lumi/lib/aws/rpc/cloudwatch"
	"github.com/pulumi/lumi/lib/aws/rpc/iam"
	"github.com/pulumi/lumi/lib/aws/rpc/lambda"
	"github.com/pulumi/lumi/lib/aws/rpc/sns"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	t.Parallel()

	prefix := resource.NewUniqueHex("lumitest", 20, 20)
	awsctx := testutil.CreateContext(t)
	defer func() {
		funcerr := cleanupFunctions(prefix, awsctx)
		assert.Nil(t, funcerr)
		roleerr := cleanupRoles(prefix, awsctx)
		assert.Nil(t, roleerr)
	}()

	sourceARN := rpc.ARN("arn:aws:s3:::elasticbeanstalk-us-east-1-111111111111")
	code := resource.Archive{
		Assets: map[string]resource.Asset{
			"index.js": {
				Text: "exports.handler = (ev, ctx, cb) => { console.log(ev); console.log(ctx); }",
			},
		},
	}

	resources := map[string]testutil.Resource{
		"role":         {Provider: iamprovider.NewRoleProvider(awsctx), Token: iam.RoleToken},
		"f":            {Provider: NewFunctionProvider(awsctx), Token: FunctionToken},
		"logcollector": {Provider: NewFunctionProvider(awsctx), Token: FunctionToken},
		"permission":   {Provider: NewPermissionProvider(awsctx), Token: PermissionToken},
		"loggroup": {Provider: cloudwatchprovider.NewLogGroupProvider(awsctx),
			Token: cloudwatchprovider.LogGroupToken},
		"filter": {Provider: cloudwatchprovider.NewLogSubscriptionFilterProvider(awsctx),
			Token: cloudwatchprovider.LogSubscriptionFilterToken},
		"deadlettertopic": {Provider: snsprovider.NewTopicProvider(awsctx), Token: snsprovider.TopicToken},
	}
	steps := []testutil.Step{
		{
			testutil.ResourceGenerator{
				Name: "role",
				Creator: func(ctx testutil.Context) interface{} {
					return &iam.Role{
						Name: aws.String(prefix),
						ManagedPolicyARNs: &[]rpc.ARN{
							rpc.ARN("arn:aws:iam::aws:policy/AWSLambdaFullAccess"),
						},
						AssumeRolePolicyDocument: map[string]interface{}{
							"Version": "2012-10-17",
							"Statement": []map[string]interface{}{
								{
									"Action": "sts:AssumeRole",
									"Principal": map[string]interface{}{
										"Service": "lambda.amazonaws.com",
									},
									"Effect": "Allow",
									"Sid":    "",
								},
							},
						},
					}
				},
			},
			testutil.ResourceGenerator{
				Name: "deadlettertopic",
				Creator: func(ctx testutil.Context) interface{} {
					return &sns.Topic{
						Name: aws.String(prefix),
					}
				},
			},
			testutil.ResourceGenerator{
				Name: "f",
				Creator: func(ctx testutil.Context) interface{} {
					return &lambda.Function{
						Name:    aws.String(prefix),
						Code:    code,
						Handler: "index.handler",
						Runtime: lambda.NodeJS6d10Runtime,
						Role:    ctx.GetResourceID("role"),
						DeadLetterConfig: &lambda.DeadLetterConfig{
							Target: ctx.GetResourceID("deadlettertopic"),
						},
					}
				},
			},
			testutil.ResourceGenerator{
				Name: "logcollector",
				Creator: func(ctx testutil.Context) interface{} {
					return &lambda.Function{
						Name:    aws.String(prefix),
						Code:    code,
						Handler: "index.handler",
						Runtime: lambda.NodeJS6d10Runtime,
						Role:    ctx.GetResourceID("role"),
					}
				},
			},
			testutil.ResourceGenerator{
				Name: "loggroup",
				Creator: func(ctx testutil.Context) interface{} {
					return &cloudwatch.LogGroup{
						Name: aws.String(prefix),
						LogGroupName: aws.String("/aws/lambda/" +
							ctx.GetOutputProps("f").Fields["functionName"].GetStringValue()),
						RetentionInDays: aws.Float64(float64(7)),
					}
				},
			},
			testutil.ResourceGenerator{
				Name: "permission",
				Creator: func(ctx testutil.Context) interface{} {
					sourceARN = rpc.ARN(string(ctx.GetResourceID("loggroup")) + ":*")
					return &lambda.Permission{
						Name:          aws.String(prefix),
						Function:      ctx.GetResourceID("logcollector"),
						Action:        "lambda:InvokeFunction",
						Principal:     "logs." + awsctx.Region() + ".amazonaws.com",
						SourceAccount: aws.String(awsctx.AccountID()),
						SourceARN:     &sourceARN,
					}
				},
			},
			testutil.ResourceGenerator{
				Name: "filter",
				Creator: func(ctx testutil.Context) interface{} {
					return &cloudwatch.LogSubscriptionFilter{
						Name:           aws.String(prefix),
						DestinationArn: string(ctx.GetResourceID("logcollector")),
						LogGroupName:   ctx.GetOutputProps("loggroup").Fields["logGroupName"].GetStringValue(),
					}
				},
			},
		},
	}

	props := testutil.ProviderTest(t, resources, steps)

	// Returned SHA256 must match what we uploaded
	byts, err := code.Bytes(resource.ZIPArchive)
	assert.NoError(t, err)
	sum := sha256.Sum256(byts)
	codeSHA256 := base64.StdEncoding.EncodeToString(sum[:])
	assert.Equal(t, codeSHA256, props["f"].Fields["codeSHA256"].GetStringValue())
}

func cleanupFunctions(prefix string, ctx *awsctx.Context) error {
	fmt.Printf("Cleaning up function with name:%v\n", prefix)
	list, err := ctx.Lambda().ListFunctions(&awslambda.ListFunctionsInput{})
	if err != nil {
		return err
	}
	cleaned := 0
	for _, fnc := range list.Functions {
		if strings.HasPrefix(aws.StringValue(fnc.FunctionName), prefix) {
			if _, delerr := ctx.Lambda().DeleteFunction(&awslambda.DeleteFunctionInput{
				FunctionName: fnc.FunctionName,
			}); delerr != nil {
				fmt.Printf("Unable to cleanup function %v: %v\n", fnc.FunctionName, delerr)
				return delerr
			}
			cleaned++
		}
	}
	fmt.Printf("Cleaned up %v functions\n", cleaned)
	return nil
}

func cleanupRoles(prefix string, ctx *awsctx.Context) error {
	fmt.Printf("Cleaning up roles with name:%v\n", prefix)
	list, err := ctx.IAM().ListRoles(&awsiam.ListRolesInput{})
	if err != nil {
		return err
	}
	cleaned := 0
	for _, role := range list.Roles {
		if strings.HasPrefix(aws.StringValue(role.RoleName), prefix) {
			policies, err := ctx.IAM().ListAttachedRolePolicies(&awsiam.ListAttachedRolePoliciesInput{
				RoleName: role.RoleName,
			})
			if err != nil {
				fmt.Printf("Unable to cleanup role %v: %v\n", role.RoleName, err)
				return err
			}
			if policies != nil {
				for _, policy := range policies.AttachedPolicies {
					if _, deterr := ctx.IAM().DetachRolePolicy(&awsiam.DetachRolePolicyInput{
						RoleName:  role.RoleName,
						PolicyArn: policy.PolicyArn,
					}); deterr != nil {
						return deterr
					}
				}
			}
			if _, delerr := ctx.IAM().DeleteRole(&awsiam.DeleteRoleInput{
				RoleName: role.RoleName,
			}); delerr != nil {
				fmt.Printf("Unable to cleanup role %v: %v\n", role.RoleName, err)
				return delerr
			}
			cleaned++
		}
	}
	fmt.Printf("Cleaned up %v roles\n", cleaned)
	return nil
}
