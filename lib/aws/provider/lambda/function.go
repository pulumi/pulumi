// Copyright 2017 Pulumi, Inc. All rights reserved.

package lambda

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/lambda"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/pulumi/coconut/pkg/resource"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
	"github.com/pulumi/coconut/pkg/util/mapper"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
	"golang.org/x/net/context"

	"github.com/pulumi/coconut/lib/aws/provider/awsctx"
)

const Function = tokens.Type("aws:lambda/function:Function")

// NewFunctionProvider creates a provider that handles Lambda function operations.
func NewFunctionProvider(ctx *awsctx.Context) cocorpc.ResourceProviderServer {
	return &funcProvider{ctx}
}

type funcProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *funcProvider) Check(ctx context.Context, req *cocorpc.CheckRequest) (*cocorpc.CheckResponse, error) {
	// Read in the properties, create and validate a new group, and return the failures (if any).
	contract.Assert(req.GetType() == string(Function))
	_, _, result := unmarshalFunction(req.GetProperties())
	return resource.NewCheckResponse(result), nil
}

// Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
// simply fetches it from the property bag; other times, the provider will assign this based on its own algorithm.
// In any case, resources with the same name must be safe to use interchangeably with one another.
func (p *funcProvider) Name(ctx context.Context, req *cocorpc.NameRequest) (*cocorpc.NameResponse, error) {
	return nil, nil // use the AWS provider default name
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *funcProvider) Create(ctx context.Context, req *cocorpc.CreateRequest) (*cocorpc.CreateResponse, error) {
	contract.Assert(req.GetType() == string(Function))

	// Read in the properties given by the request, validating as we go; if any fail, reject the request.
	fun, _, decerr := unmarshalFunction(req.GetProperties())
	if decerr != nil {
		// TODO: this is a good example of a "benign" (StateOK) error; handle it accordingly.
		return nil, decerr
	}

	// If an explicit name is given, use it.  Otherwise, auto-generate a name in part based on the resource name.
	// TODO: use the URN, not just the name, to enhance global uniqueness.
	// TODO: even for explicit names, we should consider mangling it somehow, to reduce multi-instancing conflicts.
	var name string
	if fun.FunctionName != nil {
		name = *fun.FunctionName
	} else {
		name = resource.NewUniqueHex(fun.Name+"-", maxFunctionName, sha1.Size)
	}

	// Fetch the IAM role's ARN.
	// TODO[coconut/pulumi#90]: as soon as we can read output properties, this shouldn't be necessary.
	role, err := p.ctx.IAM().GetRole(&iam.GetRoleInput{RoleName: fun.Role.StringPtr()})
	if err != nil {
		return nil, err
	}
	roleARN := role.Role.Arn

	// Figure out the kind of asset.  In addition to the usual suspects, we permit s3:// references.
	var code *lambda.FunctionCode
	uri, isuri, err := fun.Code.GetURIURL()
	if err != nil {
		return nil, err
	}
	if isuri && uri.Scheme == "s3" {
		// TODO: it's odd that an S3 reference must *already* be a zipfile, whereas others are zipped on the fly.
		code = &lambda.FunctionCode{
			S3Bucket: aws.String(uri.Host),
			S3Key:    aws.String(uri.Path),
			// TODO: S3ObjectVersion; encode as the #?
		}
	} else {
		zip, err := zipCodeAsset(fun.Code, "index.js") // TODO: don't hard-code the filename.
		if err != nil {
			return nil, err
		}
		code = &lambda.FunctionCode{ZipFile: zip}
	}

	var dlqcfg *lambda.DeadLetterConfig
	var env *lambda.Environment
	var vpccfg *lambda.VpcConfig

	// Convert float fields to in64 if they are non-nil.
	var memsize *int64
	if fun.MemorySize != nil {
		sz := int64(*fun.MemorySize)
		memsize = &sz
	}
	var timeout *int64
	if fun.Timeout != nil {
		to := int64(*fun.Timeout)
		timeout = &to
	}

	// Now go ahead and create the resource.  Note that IAM profiles can take several seconds to propagate; see
	// http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html#launch-instance-with-role.
	fmt.Printf("Creating Lambda Function '%v' with name '%v'\n", fun.Name, name)
	create := &lambda.CreateFunctionInput{
		Code:             code,
		DeadLetterConfig: dlqcfg,
		Description:      fun.Description,
		Environment:      env,
		FunctionName:     aws.String(name),
		Handler:          aws.String(fun.Handler),
		KMSKeyArn:        fun.KMSKeyID.StringPtr(),
		MemorySize:       memsize,
		Publish:          nil, // ???
		Role:             role.Role.Arn,
		Runtime:          aws.String(fun.Runtime),
		Timeout:          timeout,
		VpcConfig:        vpccfg,
	}
	var result *lambda.FunctionConfiguration
	succ, err := awsctx.RetryProgUntil(
		p.ctx,
		func() (bool, error) {
			var err error
			result, err = p.ctx.Lambda().CreateFunction(create)
			if err != nil {
				if erraws, iserraws := err.(awserr.Error); iserraws {
					if erraws.Code() == "InvalidParameterValueException" &&
						erraws.Message() == "The role defined for the function cannot be assumed by Lambda." {
						return false, nil // retry the condition.
					}
				}
				return false, err
			}
			return true, nil
		},
		func(n int) bool {
			fmt.Printf("Lambda IAM role '%v' not yet ready; waiting for it to become usable...\n", *roleARN)
			return true
		},
	)
	if err != nil {
		return nil, err
	}
	if !succ {
		return nil, fmt.Errorf("Lambda IAM role '%v' did not become useable", *roleARN)
	}

	// Wait for the function to be ready and then return the function name as the ID.
	fmt.Printf("Lambda Function created: %v; waiting for it to become active\n", name)
	if err = p.waitForFunctionState(name, true); err != nil {
		return nil, err
	}
	return &cocorpc.CreateResponse{
		Id: name,
		Outputs: resource.MarshalProperties(
			nil,
			resource.NewPropertyMap(
				functionOutput{
					ARN: *result.FunctionArn,
				},
			),
			resource.MarshalOptions{},
		),
	}, nil
}

// Read reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *funcProvider) Get(ctx context.Context, req *cocorpc.GetRequest) (*cocorpc.GetResponse, error) {
	contract.Assert(req.GetType() == string(Function))
	return nil, errors.New("Not yet implemented")
}

// PreviewUpdate checks what impacts a hypothetical update will have on the resource's properties.
func (p *funcProvider) PreviewUpdate(
	ctx context.Context, req *cocorpc.UpdateRequest) (*cocorpc.PreviewUpdateResponse, error) {
	contract.Assert(req.GetType() == string(Function))
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *funcProvider) Update(ctx context.Context, req *cocorpc.UpdateRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(Function))
	return nil, errors.New("Not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *funcProvider) Delete(ctx context.Context, req *cocorpc.DeleteRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(Function))

	// First, perform the deletion.
	id := req.GetId()
	fmt.Printf("Deleting Lambda Function '%v'\n", id)
	if _, err := p.ctx.Lambda().DeleteFunction(&lambda.DeleteFunctionInput{
		FunctionName: aws.String(id),
	}); err != nil {
		return nil, err
	}

	// Wait for the function to actually become deleted before returning.
	fmt.Printf("Lambda Function delete request submitted; waiting for it to delete\n")
	if err := p.waitForFunctionState(id, false); err != nil {
		return nil, err
	}
	return &pbempty.Empty{}, nil
}

// function represents the state associated with an Lambda function.
type function struct {
	Name             string            `json:"name"`                       // the function resource's name.
	Code             resource.Asset    `json:"code"`                       // the function's code.
	Handler          string            `json:"handler"`                    // the name of the function's handler.
	Role             resource.ID       `json:"role"`                       // the AWS IAM execution role.
	Runtime          string            `json:"runtime"`                    // the language runtime.
	FunctionName     *string           `json:"functionName,omitempty"`     // the function's published name.
	DeadLetterConfig *deadLetterConfig `json:"deadLetterConfig,omitempty"` // a dead letter queue/topic config.
	Description      *string           `json:"description,omitempty"`      // an optional friendly description.
	Environment      *environment      `json:"environment,omitempty"`      // environment variables.
	KMSKeyID         *resource.ID      `json:"kmsKey,omitempty"`           // a KMS key for encrypting/decrypting.
	MemorySize       *float64          `json:"memorySize,omitempty"`       // maximum amount of memory in MB.
	Timeout          *float64          `json:"timeout,omitempty"`          // maximum execution time in seconds.
	VPCConfig        *vpcConfig        `json:"vpcConfig,omitempty"`        // optional VPC config for an ENI.
}

type deadLetterConfig struct {
	Target resource.ID `json:"target"` // the target SNS topic or SQS queue.
}

type environment map[string]string

type vpcConfig struct {
	SecurityGroups []resource.ID `json:"securityGroups"` // security groups for resources this function uses.
	Subnets        []resource.ID `json:"subnets"`        // subnets for resources this function uses.
}

// constants for function property names.
const (
	FunctionName         = "name"
	FunctionRuntime      = "runtime"
	FunctionFunctionName = "functionName"
)

// constants for the various function limits.
const (
	maxFunctionName       = 64
	maxFunctionNameARN    = 140
	functionNameARNPrefix = "arn:aws:lambda:"
)

type functionOutput struct {
	ARN string `json:"arn"`
}

var functionRuntimes = map[string]bool{
	"nodejs":         true,
	"nodejs4.3":      true,
	"nodejs4.3-edge": true,
	"nodejs6.10":     true,
	"java8":          true,
	"python2.7":      true,
	"dotnetcore1.0":  true,
}

// unmarshalFunction decodes and validates a function property bag.
func unmarshalFunction(v *pbstruct.Struct) (function, resource.PropertyMap, mapper.DecodeError) {
	var fun function
	props := resource.UnmarshalProperties(v)
	result := mapper.MapIU(props.Mappable(), &fun)
	if _, has := functionRuntimes[fun.Runtime]; !has {
		if result == nil {
			result = mapper.NewDecodeErr(nil)
		}
		result.AddFailure(
			mapper.NewFieldErr(reflect.TypeOf(fun), FunctionRuntime,
				fmt.Errorf("%v is not a valid runtime", fun.Runtime)),
		)
	}
	if name := fun.FunctionName; name != nil {
		var maxName int
		if strings.HasPrefix(*name, functionNameARNPrefix) {
			maxName = maxFunctionNameARN
		} else {
			maxName = maxFunctionName
		}
		if len(*name) > maxName {
			if result == nil {
				result = mapper.NewDecodeErr(nil)
			}
			result.AddFailure(
				mapper.NewFieldErr(reflect.TypeOf(fun), FunctionFunctionName,
					fmt.Errorf("exceeded maximum length of %v", maxName)),
			)
		}
	}
	return fun, props, result
}

func (p *funcProvider) waitForFunctionState(id string, exist bool) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			if _, err := p.ctx.Lambda().GetFunction(&lambda.GetFunctionInput{
				FunctionName: aws.String(id),
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

// zipCodeAsset zips up a single code asset using the given filename.
func zipCodeAsset(code resource.Asset, filename string) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	file, err := w.Create(filename)
	if err != nil {
		return nil, err
	}
	r, err := code.Read()
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(file, r); err != nil {
		return nil, err
	}
	err = w.Close()
	return buf.Bytes(), err
}
