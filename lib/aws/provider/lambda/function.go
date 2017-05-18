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

package lambda

import (
	"crypto/sha1"
	"fmt"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awsiam "github.com/aws/aws-sdk-go/service/iam"
	awslambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/mapper"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	awscommon "github.com/pulumi/lumi/lib/aws/rpc"
	"github.com/pulumi/lumi/lib/aws/rpc/lambda"
)

const FunctionToken = lambda.FunctionToken

// constants for the various function limits.
const (
	maxFunctionName       = 64
	maxFunctionNameARN    = 140
	functionNameARNPrefix = "arn:aws:lambda:"
)

var functionRuntimes = map[lambda.Runtime]bool{
	lambda.NodeJSRuntime:        true,
	lambda.NodeJS4d3Runtime:     true,
	lambda.NodeJS4d3EdgeRuntime: true,
	lambda.NodeJS6d10Runtime:    true,
	lambda.Java8Runtime:         true,
	lambda.Python2d7Runtime:     true,
	lambda.DotnetCore1d0Runtime: true,
}

// NewFunctionProvider creates a provider that handles Lambda function operations.
func NewFunctionProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &funcProvider{ctx}
	return lambda.NewFunctionProvider(ops)
}

type funcProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *funcProvider) Check(ctx context.Context, obj *lambda.Function) ([]mapper.FieldError, error) {
	var failures []mapper.FieldError
	if _, has := functionRuntimes[obj.Runtime]; !has {
		failures = append(failures,
			mapper.NewFieldErr(reflect.TypeOf(obj), lambda.Function_Runtime,
				fmt.Errorf("%v is not a valid runtime", obj.Runtime)))
	}
	if name := obj.FunctionName; name != nil {
		var maxName int
		if strings.HasPrefix(*name, functionNameARNPrefix) {
			maxName = maxFunctionNameARN
		} else {
			maxName = maxFunctionName
		}
		if len(*name) > maxName {
			failures = append(failures,
				mapper.NewFieldErr(reflect.TypeOf(obj), lambda.Function_FunctionName,
					fmt.Errorf("exceeded maximum length of %v", maxName)))
		}
	}
	return failures, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *funcProvider) Create(ctx context.Context, obj *lambda.Function) (resource.ID, *lambda.FunctionOuts, error) {
	// If an explicit name is given, use it.  Otherwise, auto-generate a name in part based on the resource name.
	// TODO: use the URN, not just the name, to enhance global uniqueness.
	// TODO: even for explicit names, we should consider mangling it somehow, to reduce multi-instancing conflicts.
	var id resource.ID
	if obj.FunctionName != nil {
		id = resource.ID(*obj.FunctionName)
	} else {
		id = resource.NewUniqueHexID(obj.Name+"-", maxFunctionName, sha1.Size)
	}

	// Fetch the IAM role's ARN.
	// TODO[lumi/pulumi#90]: as soon as we can read output properties, this shouldn't be necessary.
	var roleARN *string
	if role, err := p.ctx.IAM().GetRole(&awsiam.GetRoleInput{RoleName: obj.Role.StringPtr()}); err != nil {
		return "", nil, err
	} else {
		roleARN = role.Role.Arn
	}

	// Figure out the kind of asset.  In addition to the usual suspects, we permit s3:// references.
	var code *awslambda.FunctionCode
	if uri, isuri, err := obj.Code.GetURIURL(); err != nil {
		return "", nil, err
	} else if isuri && uri.Scheme == "s3" {
		// TODO: it's odd that an S3 reference must *already* be a zipfile, whereas others are zipped on the fly.
		code = &awslambda.FunctionCode{
			S3Bucket: aws.String(uri.Host),
			S3Key:    aws.String(uri.Path),
			// TODO: S3ObjectVersion; encode as the #?
		}
	} else {
		zip := obj.Code.Bytes(resource.ZIPArchive)
		code = &awslambda.FunctionCode{ZipFile: zip}
	}

	var dlqcfg *awslambda.DeadLetterConfig
	var env *awslambda.Environment
	var vpccfg *awslambda.VpcConfig

	// Convert float fields to in64 if they are non-nil.
	var memsize *int64
	if obj.MemorySize != nil {
		sz := int64(*obj.MemorySize)
		memsize = &sz
	}
	var timeout *int64
	if obj.Timeout != nil {
		to := int64(*obj.Timeout)
		timeout = &to
	}

	// Now go ahead and create the resource.  Note that IAM profiles can take several seconds to propagate; see
	// http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html#launch-instance-with-role.
	fmt.Printf("Creating Lambda Function '%v' with name '%v'\n", obj.Name, id)
	create := &awslambda.CreateFunctionInput{
		Code:             code,
		DeadLetterConfig: dlqcfg,
		Description:      obj.Description,
		Environment:      env,
		FunctionName:     id.StringPtr(),
		Handler:          aws.String(obj.Handler),
		KMSKeyArn:        obj.KMSKey.StringPtr(),
		MemorySize:       memsize,
		Publish:          nil, // ???
		Role:             roleARN,
		Runtime:          aws.String(string(obj.Runtime)),
		Timeout:          timeout,
		VpcConfig:        vpccfg,
	}
	var out *lambda.FunctionOuts
	if succ, err := awsctx.RetryProgUntil(
		p.ctx,
		func() (bool, error) {
			result, err := p.ctx.Lambda().CreateFunction(create)
			if err != nil {
				if erraws, iserraws := err.(awserr.Error); iserraws {
					if erraws.Code() == "InvalidParameterValueException" &&
						erraws.Message() == "The role defined for the function cannot be assumed by Lambda." {
						return false, nil // retry the condition.
					}
				}
				return false, err
			}
			out = &lambda.FunctionOuts{ARN: awscommon.ARN(*result.FunctionArn)}
			return true, nil
		},
		func(n int) bool {
			fmt.Printf("Lambda IAM role '%v' not yet ready; waiting for it to become usable...\n", *roleARN)
			return true
		},
	); err != nil {
		return "", nil, err
	} else if !succ {
		return "", nil, fmt.Errorf("Lambda IAM role '%v' did not become useable", *roleARN)
	}

	// Wait for the function to be ready and then return the function name as the ID.
	fmt.Printf("Lambda Function created: %v; waiting for it to become active\n", id)
	if err := p.waitForFunctionState(id, true); err != nil {
		return "", nil, err
	}
	return id, out, nil
}

// Read reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *funcProvider) Get(ctx context.Context, id resource.ID) (*lambda.Function, error) {
	return nil, nil
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *funcProvider) InspectChange(ctx context.Context, id resource.ID,
	old *lambda.Function, new *lambda.Function, diff *resource.ObjectDiff) ([]string, error) {
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *funcProvider) Update(ctx context.Context, id resource.ID,
	old *lambda.Function, new *lambda.Function, diff *resource.ObjectDiff) error {
	return errors.New("Not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *funcProvider) Delete(ctx context.Context, id resource.ID) error {
	// First, perform the deletion.
	fmt.Printf("Deleting Lambda Function '%v'\n", id)
	if _, err := p.ctx.Lambda().DeleteFunction(&awslambda.DeleteFunctionInput{
		FunctionName: id.StringPtr(),
	}); err != nil {
		return err
	}

	// Wait for the function to actually become deleted before returning.
	fmt.Printf("Lambda Function delete request submitted; waiting for it to delete\n")
	return p.waitForFunctionState(id, false)
}

func (p *funcProvider) waitForFunctionState(id resource.ID, exist bool) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			if _, err := p.ctx.Lambda().GetFunction(&awslambda.GetFunctionInput{
				FunctionName: id.StringPtr(),
			}); err != nil {
				if erraws, iserraws := err.(awserr.Error); iserraws {
					if erraws.Code() == "NotFound" || erraws.Code() == "ResourceNotFoundException" {
						// The function is missing; if exist==false, we're good, otherwise keep retrying.
						return !exist, nil
					}
				}
				return false, err // anything other than "function missing" is a real error; propagate it.
			}

			// If we got here, the function was found; if exist==true, we're good; else, keep retrying.
			return exist, nil
		},
	)
	if err != nil {
		return err
	} else if !succ {
		var reason string
		if exist {
			reason = "created"
		} else {
			reason = "deleted"
		}
		return fmt.Errorf("Lambda Function '%v' did not become %v", id, reason)
	}
	return nil
}
