// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package lambda

import (
	"crypto/sha1"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	awslambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/convutil"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
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
func (p *funcProvider) Check(ctx context.Context, obj *lambda.Function, property string) error {
	switch property {
	case lambda.Function_Runtime:
		if _, has := functionRuntimes[obj.Runtime]; !has {
			return fmt.Errorf("%v is not a valid runtime", obj.Runtime)
		}
	case lambda.Function_FunctionName:
		if name := obj.FunctionName; name != nil {
			var maxName int
			if strings.HasPrefix(*name, functionNameARNPrefix) {
				maxName = maxFunctionNameARN
			} else {
				maxName = maxFunctionName
			}
			if len(*name) > maxName {
				return fmt.Errorf("exceeded maximum length of %v", maxName)
			}
		}
	}
	return nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *funcProvider) Create(ctx context.Context, obj *lambda.Function) (resource.ID, error) {
	contract.Assertf(obj.VPCConfig == nil, "VPC config not yet supported")

	// If an explicit name is given, use it.  Otherwise, auto-generate a name in part based on the resource name.
	var name string
	if obj.FunctionName != nil {
		name = *obj.FunctionName
	} else {
		name = resource.NewUniqueHex(*obj.Name+"-", maxFunctionName, sha1.Size)
	}

	code, err := p.getCode(obj.Code)
	if err != nil {
		return "", err
	}

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
	var env *awslambda.Environment
	if obj.Environment != nil {
		env = &awslambda.Environment{
			Variables: aws.StringMap(*obj.Environment),
		}
	}
	var deadLetterConfig *awslambda.DeadLetterConfig
	if obj.DeadLetterConfig != nil {
		deadLetterConfig = &awslambda.DeadLetterConfig{
			TargetArn: aws.String(string(obj.DeadLetterConfig.Target)),
		}
	}

	// Now go ahead and create the resource.  Note that IAM profiles can take several seconds to propagate; see
	// http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html#launch-instance-with-role.
	fmt.Printf("Creating Lambda Function '%v' with name '%v'\n", *obj.Name, name)
	create := &awslambda.CreateFunctionInput{
		Code:             code,
		DeadLetterConfig: deadLetterConfig,
		Description:      obj.Description,
		Environment:      env,
		FunctionName:     aws.String(name),
		Handler:          aws.String(obj.Handler),
		KMSKeyArn:        obj.KMSKey.StringPtr(),
		MemorySize:       memsize,
		Role:             aws.String(string(obj.Role)),
		Runtime:          aws.String(string(obj.Runtime)),
		Timeout:          timeout,
	}
	var arn resource.ID
	if succ, err := awsctx.RetryProgUntil(
		p.ctx,
		func() (bool, error) {
			resp, err := p.ctx.Lambda().CreateFunction(create)
			if err != nil {
				if awsctx.IsAWSErrorMessage(err,
					"InvalidParameterValueException",
					"The role defined for the function cannot be assumed by Lambda.") {
					return false, nil // retry the condition.
				}
				return false, err
			} else if resp == nil || resp.FunctionArn == nil {
				return false, errors.New("Lambda Function created, but AWS did not respond with an ARN")
			}
			arn = resource.ID(*resp.FunctionArn)
			return true, nil
		},
		func(n int) bool {
			fmt.Printf("Lambda IAM role '%v' not yet ready; waiting for it to become usable...\n", obj.Role)
			return true
		},
	); err != nil {
		return "", err
	} else if !succ {
		return "", fmt.Errorf("Lambda IAM role '%v' did not become useable", obj.Role)
	}

	// Wait for the function to be ready and then return the function name as the ID.
	fmt.Printf("Lambda Function created: %v (ARN %v); waiting for it to become active\n", name, arn)
	if err := p.waitForFunctionState(name, true); err != nil {
		return "", err
	}
	return arn, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *funcProvider) Get(ctx context.Context, id resource.ID) (*lambda.Function, error) {
	name, err := arn.ParseResourceName(id)
	if err != nil {
		return nil, err
	}
	funcresp, err := p.ctx.Lambda().GetFunction(&awslambda.GetFunctionInput{FunctionName: aws.String(name)})
	if err != nil {
		if awsctx.IsAWSError(err, awslambda.ErrCodeResourceNotFoundException) {
			return nil, nil
		}
		return nil, err
	}

	// Note: We do not extract the funcresp.Code property, as this is a pre-signed S3
	// URL at which we could download the function source code, but is not stable across
	// calls to GetFunction.

	// Deserialize all configuration properties into prompt objects.
	contract.Assert(funcresp != nil)
	config := funcresp.Configuration
	contract.Assert(config != nil)
	var env *lambda.Environment
	if config.Environment != nil {
		envmap := lambda.Environment(aws.StringValueMap(config.Environment.Variables))
		env = &envmap
	}
	var deadLetterConfig *lambda.DeadLetterConfig
	if config.DeadLetterConfig != nil {
		deadLetterConfig = &lambda.DeadLetterConfig{
			Target: resource.ID(aws.StringValue(config.DeadLetterConfig.TargetArn)),
		}
	}

	return &lambda.Function{
		ARN:              awscommon.ARN(aws.StringValue(config.FunctionArn)),
		Version:          aws.StringValue(config.Version),
		CodeSHA256:       aws.StringValue(config.CodeSha256),
		LastModified:     aws.StringValue(config.LastModified),
		Handler:          aws.StringValue(config.Handler),
		Role:             resource.ID(aws.StringValue(config.Role)),
		Runtime:          lambda.Runtime(aws.StringValue(config.Runtime)),
		FunctionName:     config.FunctionName,
		Description:      config.Description,
		DeadLetterConfig: deadLetterConfig,
		Environment:      env,
		KMSKey:           resource.MaybeID(config.KMSKeyArn),
		MemorySize:       convutil.Int64PToFloat64P(config.MemorySize),
		Timeout:          convutil.Int64PToFloat64P(config.Timeout),
	}, nil
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *funcProvider) InspectChange(ctx context.Context, id resource.ID,
	old *lambda.Function, new *lambda.Function, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *funcProvider) Update(ctx context.Context, id resource.ID,
	old *lambda.Function, new *lambda.Function, diff *resource.ObjectDiff) error {
	contract.Assertf(new.DeadLetterConfig == nil, "Dead letter config not yet supported")
	contract.Assertf(new.VPCConfig == nil, "VPC config not yet supported")

	name, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}

	if diff.Changed(lambda.Function_Description) || diff.Changed(lambda.Function_Environment) ||
		diff.Changed(lambda.Function_Runtime) || diff.Changed(lambda.Function_Role) ||
		diff.Changed(lambda.Function_MemorySize) || diff.Changed(lambda.Function_Timeout) ||
		diff.Changed(lambda.Function_Environment) || diff.Changed(lambda.Function_DeadLetterConfig) {

		update := &awslambda.UpdateFunctionConfigurationInput{
			FunctionName: aws.String(name),
		}
		if diff.Changed(lambda.Function_Description) {
			update.Description = new.Description
		}
		if diff.Changed(lambda.Function_Handler) {
			update.Handler = aws.String(new.Handler)
		}
		if diff.Changed(lambda.Function_Runtime) {
			update.Runtime = aws.String(string(new.Runtime))
		}
		if diff.Changed(lambda.Function_Role) {
			update.Role = aws.String(string(new.Role))
		}
		if diff.Changed(lambda.Function_MemorySize) {
			if new.MemorySize != nil {
				sz := int64(*new.MemorySize)
				update.MemorySize = &sz
			}
		}
		if diff.Changed(lambda.Function_Timeout) {
			if new.Timeout != nil {
				to := int64(*new.Timeout)
				update.Timeout = &to
			}
		}
		if diff.Changed(lambda.Function_Environment) {
			if new.Environment != nil {
				update.Environment = &awslambda.Environment{
					Variables: aws.StringMap(*new.Environment),
				}
			} else {
				update.Environment = &awslambda.Environment{
					Variables: map[string]*string{},
				}
			}
		}
		if diff.Changed(lambda.Function_DeadLetterConfig) {
			if new.DeadLetterConfig != nil {
				update.DeadLetterConfig = &awslambda.DeadLetterConfig{
					TargetArn: aws.String(string(new.DeadLetterConfig.Target)),
				}
			}
		}

		fmt.Printf("Updating Lambda function configuration '%v'\n", name)
		if _, retryerr := awsctx.RetryUntil(p.ctx, func() (bool, error) {
			if _, upderr := p.ctx.Lambda().UpdateFunctionConfiguration(update); upderr != nil {
				if awsctx.IsAWSErrorMessage(upderr,
					"InvalidParameterValueException",
					"The role defined for the function cannot be assumed by Lambda.") {
					return false, nil
				}
				return true, upderr
			}
			return true, nil
		}); retryerr != nil {
			return retryerr
		}

		if succ, err := awsctx.RetryProgUntil(
			p.ctx,
			func() (bool, error) {
				_, err := p.ctx.Lambda().UpdateFunctionConfiguration(update)
				if err != nil {
					if awsctx.IsAWSErrorMessage(err,
						"InvalidParameterValueException",
						"The role defined for the function cannot be assumed by Lambda.") {
						return false, nil // retry the condition.
					}
					return false, err
				}
				return true, nil
			},
			func(n int) bool {
				fmt.Printf("Lambda IAM role '%v' not yet ready; waiting for it to become usable...\n", update.Role)
				return true
			},
		); err != nil {
			return err
		} else if !succ {
			return fmt.Errorf("Lambda IAM role '%v' did not become useable", update.Role)
		}
	}

	if diff.Changed(lambda.Function_Code) {
		code, err := p.getCode(new.Code)
		if err != nil {
			return err
		}
		update := &awslambda.UpdateFunctionCodeInput{
			FunctionName:    aws.String(name),
			S3Bucket:        code.S3Bucket,
			S3Key:           code.S3Key,
			S3ObjectVersion: code.S3ObjectVersion,
			ZipFile:         code.ZipFile,
		}
		fmt.Printf("Updating Lambda function code '%v'\n", name)
		if _, err := p.ctx.Lambda().UpdateFunctionCode(update); err != nil {
			return err
		}
	}
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *funcProvider) Delete(ctx context.Context, id resource.ID) error {
	name, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}

	// First, perform the deletion.
	fmt.Printf("Deleting Lambda Function '%v'\n", name)
	if _, err := p.ctx.Lambda().DeleteFunction(&awslambda.DeleteFunctionInput{
		FunctionName: aws.String(name),
	}); err != nil {
		return err
	}

	// Wait for the function to actually become deleted before returning.
	fmt.Printf("Lambda Function delete request submitted; waiting for it to delete\n")
	return p.waitForFunctionState(name, false)
}

func (p *funcProvider) waitForFunctionState(name string, exist bool) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			if _, err := p.ctx.Lambda().GetFunction(&awslambda.GetFunctionInput{
				FunctionName: aws.String(name),
			}); err != nil {
				if awsctx.IsAWSError(err, "NotFound", "ResourceNotFoundException") {
					// The function is missing; if exist==false, we're good, otherwise keep retrying.
					return !exist, nil
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
		return fmt.Errorf("Lambda Function '%v' did not become %v", name, reason)
	}
	return nil
}

func (p *funcProvider) getCode(codeArchive resource.Archive) (*awslambda.FunctionCode, error) {
	// Figure out the kind of asset.  In addition to the usual suspects, we permit s3:// references.
	if uri, isuri, err := codeArchive.GetURIURL(); err != nil {
		return nil, err
	} else if isuri && uri.Scheme == "s3" {
		return &awslambda.FunctionCode{
			S3Bucket: aws.String(uri.Host),
			S3Key:    aws.String(uri.Path),
			// TODO[pulumi/lumi#222]: S3ObjectVersion; encode as the #?
		}, nil
	} else {
		zip, err := codeArchive.Bytes(resource.ZIPArchive)
		if err != nil {
			return nil, err
		}
		return &awslambda.FunctionCode{ZipFile: zip}, nil
	}
}
