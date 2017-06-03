package lambda

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	awsiam "github.com/aws/aws-sdk-go/service/iam"
	awslambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	iamprovider "github.com/pulumi/lumi/lib/aws/provider/iam"
	"github.com/pulumi/lumi/lib/aws/provider/testutil"
	rpc "github.com/pulumi/lumi/lib/aws/rpc"
	"github.com/pulumi/lumi/lib/aws/rpc/iam"
	"github.com/pulumi/lumi/lib/aws/rpc/lambda"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/stretchr/testify/assert"
)

const RESOURCEPREFIX = "lumitest"

func Test(t *testing.T) {
	t.Parallel()

	ctx, err := awsctx.New()
	assert.Nil(t, err, "expected no error getting AWS context")

	cleanupFunctions(ctx)
	cleanupRoles(ctx)

	functionProvider := NewFunctionProvider(ctx)
	roleProvider := iamprovider.NewRoleProvider(ctx)

	resources := map[string]testutil.Resource{
		"role": {Provider: roleProvider, Token: iam.RoleToken},
		"f":    {Provider: functionProvider, Token: FunctionToken},
	}
	steps := []testutil.Step{
		testutil.Step{
			testutil.ResourceGenerator{
				Name: "role",
				Creator: func(ctx testutil.Context) interface{} {
					return &iam.Role{
						Name: aws.String(RESOURCEPREFIX),
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
				Name: "f",
				Creator: func(ctx testutil.Context) interface{} {
					return &lambda.Function{
						Name: aws.String(RESOURCEPREFIX),
						Code: resource.Archive{
							Assets: &map[string]*resource.Asset{
								"index.js": &resource.Asset{
									Text: aws.String("exports.handler = (ev, ctx, cb) => { console.log(ev); console.log(ctx); }"),
								},
							},
						},
						Handler: "index.handler",
						Runtime: lambda.NodeJS6d10Runtime,
						Role:    ctx.GetResourceID("role"),
					}
				},
			},
		},
	}

	testutil.ProviderTest(t, resources, steps)

}

func cleanupFunctions(ctx *awsctx.Context) {
	fmt.Printf("Cleaning up function with name:%v\n", RESOURCEPREFIX)
	list, err := ctx.Lambda().ListFunctions(&awslambda.ListFunctionsInput{})
	if err != nil {
		return
	}
	cleaned := 0
	for _, fnc := range list.Functions {
		if strings.HasPrefix(aws.StringValue(fnc.FunctionName), RESOURCEPREFIX) {
			_, err := ctx.Lambda().DeleteFunction(&awslambda.DeleteFunctionInput{
				FunctionName: fnc.FunctionName,
			})
			if err != nil {
				fmt.Printf("Unable to cleanip function %v: %v\n", fnc.FunctionName, err)
			} else {
				cleaned++
			}
		}
	}
	fmt.Printf("Cleaned up %v functions\n", cleaned)
}

func cleanupRoles(ctx *awsctx.Context) {
	fmt.Printf("Cleaning up roles with name:%v\n", RESOURCEPREFIX)
	list, err := ctx.IAM().ListRoles(&awsiam.ListRolesInput{})
	if err != nil {
		return
	}
	cleaned := 0
	for _, role := range list.Roles {
		if strings.HasPrefix(aws.StringValue(role.RoleName), RESOURCEPREFIX) {
			policies, err := ctx.IAM().ListAttachedRolePolicies(&awsiam.ListAttachedRolePoliciesInput{
				RoleName: role.RoleName,
			})
			if err != nil {
				fmt.Printf("Unable to cleanup role %v: %v\n", role.RoleName, err)
				continue
			}
			if policies != nil {
				for _, policy := range policies.AttachedPolicies {
					ctx.IAM().DetachRolePolicy(&awsiam.DetachRolePolicyInput{
						RoleName:  role.RoleName,
						PolicyArn: policy.PolicyArn,
					})
				}
			}
			_, err = ctx.IAM().DeleteRole(&awsiam.DeleteRoleInput{
				RoleName: role.RoleName,
			})
			if err != nil {
				fmt.Printf("Unable to cleanup role %v: %v\n", role.RoleName, err)
			} else {
				cleaned++
			}
		}
	}
	fmt.Printf("Cleaned up %v roles\n", cleaned)
}
