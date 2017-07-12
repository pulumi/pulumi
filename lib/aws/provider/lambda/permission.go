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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	awslambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	awscommon "github.com/pulumi/lumi/lib/aws/rpc"
	"github.com/pulumi/lumi/lib/aws/rpc/lambda"
)

const PermissionToken = lambda.PermissionToken

const (
	maxStatementID      = 100
	actionRegexp        = "(lambda:[*]|lambda:[a-zA-Z]+|[*])"
	sourceAccountRegexp = "\\d{12}"
	sourceARNRegexp     = "arn:aws:([a-zA-Z0-9\\-])+:([a-z]{2}-[a-z]+-\\d{1})?:(\\d{12})?:(.*)"
)

type policy struct {
	Version   string
	ID        string `json:"Id"`
	Statement []statement
}

type statement struct {
	Sid       string
	Effect    string
	Principal principal
	Action    string
	Resource  string
	Condition condition
}

type principal struct {
	Service string
}

type condition struct {
	ArnLike      *arnLike      `json:"ArnLike,omitempty"`
	StringEquals *stringEquals `json:"StringEquals,omitempty"`
}

type arnLike struct {
	AWSSourceArn *string `json:"AWS:SourceArn,omitempty"`
}

type stringEquals struct {
	AWSSourceAccount *string `json:"AWS:SourceAccount,omitempty"`
}

// NewPermissionID returns an AWS APIGateway Deployment ARN ID for the given restAPIID and deploymentID
func NewPermissionID(region, account, functionName, statementID string) resource.ID {
	return arn.NewID("lambda", region, account, "function:"+functionName+":policy:"+statementID)
}

// ParsePermissionID parses an AWS APIGateway Deployment ARN ID to extract the restAPIID and deploymentID
func ParsePermissionID(id resource.ID) (string, string, error) {
	res, err := arn.ParseResourceName(id)
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(res, ":")
	if len(parts) != 3 || parts[1] != "policy" {
		return "", "", fmt.Errorf("expected Permission ARN of the form %v: %v",
			"arn:aws:lambda:region:account:function:function-name:policy:statement-id", id)
	}
	return parts[0], parts[2], nil
}

// NewPermissionProvider creates a provider that handles Lambda permission operations.
func NewPermissionProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &permissionProvider{ctx}
	return lambda.NewPermissionProvider(ops)
}

type permissionProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *permissionProvider) Check(ctx context.Context, obj *lambda.Permission, property string) error {
	switch property {
	case lambda.Permission_Action:
		if matched, err := regexp.MatchString(actionRegexp, obj.Action); err != nil || !matched {
			return fmt.Errorf("did not match regexp %v", actionRegexp)
		}
	case lambda.Permission_SourceAccount:
		if obj.SourceAccount != nil {
			if matched, err := regexp.MatchString(sourceAccountRegexp, *obj.SourceAccount); err != nil || !matched {
				return fmt.Errorf("did not match regexp %v", sourceAccountRegexp)
			}
		}
	case lambda.Permission_SourceARN:
		if obj.SourceARN != nil {
			if matched, err := regexp.MatchString(sourceARNRegexp, string(*obj.SourceARN)); err != nil || !matched {
				return fmt.Errorf("did not match regexp %v", sourceARNRegexp)
			}
		}
	}
	return nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *permissionProvider) Create(ctx context.Context, obj *lambda.Permission) (resource.ID, error) {
	// Auto-generate a name in part based on the resource name.
	statementID := resource.NewUniqueHex(*obj.Name+"-", maxStatementID, sha1.Size)
	functionName, err := arn.ParseResourceName(obj.Function)
	if err != nil {
		return "", err
	}
	fmt.Printf("Creating Lambda Permission '%v' with statement ID '%v'\n", *obj.Name, statementID)
	create := &awslambda.AddPermissionInput{
		Action:        aws.String(obj.Action),
		FunctionName:  aws.String(functionName),
		Principal:     aws.String(obj.Principal),
		SourceAccount: obj.SourceAccount,
		StatementId:   aws.String(statementID),
	}
	if obj.SourceARN != nil {
		create.SourceArn = aws.String(string(*obj.SourceARN))
	}
	_, err = p.ctx.Lambda().AddPermission(create)
	if err != nil {
		return "", err
	}

	return NewPermissionID(p.ctx.Region(), p.ctx.AccountID(), functionName, statementID), nil
}

// Query returns an (possibly empty) array of resource objects.
func (p *permissionProvider) Query(ctx context.Context) ([]*lambda.Permission, error) {
	funcs := p.ctx.Lambda().ListFunctions().Functions
	var names []string
	for _, fun := range funcs {
		names = append(names, funcs.FunctionName)
	}

	var permissions []*lambda.Permission
	for _, functionName := range names {
		resp, err := p.ctx.Lambda().GetPolicy(&awslambda.GetPolicyInput{
			FunctionName: aws.String(functionName),
		})
		if err != nil {
			return nil, err
		}
		contract.Assert(resp != nil)
		contract.Assert(resp.Policy != nil)
		policyDoc := policy{}
		if jsonerr := json.Unmarshal([]byte(*resp.Policy), &policyDoc); jsonerr != nil {
			return nil, jsonerr
		}
		for _, statement := range policyDoc.Statement {
			if statement.Sid == statementID {
				permission := &lambda.Permission{
					Action:    statement.Action,
					Function:  resource.ID(statement.Resource),
					Principal: statement.Principal.Service,
				}
				// The statements generated by `lambda.AddPermission` will contain up to two Condition elements
				// of the following two forms, corresponding to the optional SourceARN and SourceAccount properties.
				//   "ArnLike": { "AWS:SourceArn": "<arn>" }
				// or:
				//   "StringEquals": { "AWS:SourceAccount": "<account-id>" }
				condition := statement.Condition
				if condition.ArnLike != nil && condition.ArnLike.AWSSourceArn != nil {
					sourceARN := awscommon.ARN(*condition.ArnLike.AWSSourceArn)
					permission.SourceARN = &sourceARN
				}
				if condition.StringEquals != nil && condition.StringEquals.AWSSourceAccount != nil {
					permission.SourceAccount = condition.StringEquals.AWSSourceAccount
				}
				permissions = append(permissions, permission)
			}
		}
	}
	return permissions, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *permissionProvider) Get(ctx context.Context, id resource.ID) (*lambda.Permission, error) {
	functionName, statementID, err := ParsePermissionID(id)
	if err != nil {
		return nil, err
	}
	resp, err := p.ctx.Lambda().GetPolicy(&awslambda.GetPolicyInput{
		FunctionName: aws.String(functionName),
	})
	if err != nil {
		return nil, err
	}
	contract.Assert(resp != nil)
	contract.Assert(resp.Policy != nil)
	policyDoc := policy{}
	if jsonerr := json.Unmarshal([]byte(*resp.Policy), &policyDoc); jsonerr != nil {
		return nil, jsonerr
	}
	for _, statement := range policyDoc.Statement {
		if statement.Sid == statementID {
			permission := &lambda.Permission{
				Action:    statement.Action,
				Function:  resource.ID(statement.Resource),
				Principal: statement.Principal.Service,
			}
			// The statements generated by `lambda.AddPermission` will contain up to two Condition elements
			// of the following two forms, corresponding to the optional SourceARN and SourceAccount properties.
			//   "ArnLike": { "AWS:SourceArn": "<arn>" }
			// or:
			//   "StringEquals": { "AWS:SourceAccount": "<account-id>" }
			condition := statement.Condition
			if condition.ArnLike != nil && condition.ArnLike.AWSSourceArn != nil {
				sourceARN := awscommon.ARN(*condition.ArnLike.AWSSourceArn)
				permission.SourceARN = &sourceARN
			}
			if condition.StringEquals != nil && condition.StringEquals.AWSSourceAccount != nil {
				permission.SourceAccount = condition.StringEquals.AWSSourceAccount
			}
			return permission, nil
		}
	}
	return nil, fmt.Errorf("No statement found for id '%v'", id)
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *permissionProvider) InspectChange(ctx context.Context, id resource.ID,
	old *lambda.Permission, new *lambda.Permission, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *permissionProvider) Update(ctx context.Context, id resource.ID,
	old *lambda.Permission, new *lambda.Permission, diff *resource.ObjectDiff) error {
	contract.Failf("No properties of Permission resource are updatable.")
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *permissionProvider) Delete(ctx context.Context, id resource.ID) error {
	functionName, statementID, err := ParsePermissionID(id)
	if err != nil {
		return err
	}
	fmt.Printf("Deleting Lambda Permission '%v'\n", statementID)
	_, remerr := p.ctx.Lambda().RemovePermission(&awslambda.RemovePermissionInput{
		FunctionName: aws.String(functionName),
		StatementId:  aws.String(statementID),
	})
	return remerr
}
