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

package iam

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awsiam "github.com/aws/aws-sdk-go/service/iam"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/mapper"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	awscommon "github.com/pulumi/lumi/lib/aws/rpc"
	"github.com/pulumi/lumi/lib/aws/rpc/iam"
)

const RoleToken = iam.RoleToken

// constants for the various role limits.
const (
	maxRoleName = 64 // TODO: to use Switch Role, Path+RoleName cannot exceed 64 characters.  Warn?
)

// NewRoleProvider creates a provider that handles IAM role operations.
func NewRoleProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &roleProvider{ctx}
	return iam.NewRoleProvider(ops)
}

type roleProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *roleProvider) Check(ctx context.Context, obj *iam.Role) ([]mapper.FieldError, error) {
	return nil, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *roleProvider) Create(ctx context.Context, obj *iam.Role) (resource.ID, *iam.RoleOuts, error) {
	contract.Assertf(obj.ManagedPolicyARNs == nil, "Managed policies not yet supported")
	contract.Assertf(obj.Policies == nil, "Inline policies not yet supported")

	// If an explicit name is given, use it.  Otherwise, auto-generate a name in part based on the resource name.
	// TODO: use the URN, not just the name, to enhance global uniqueness.
	// TODO: even for explicit names, we should consider mangling it somehow, to reduce multi-instancing conflicts.
	var id resource.ID
	if obj.RoleName != nil {
		id = resource.ID(*obj.RoleName)
	} else {
		id = resource.NewUniqueHexID(obj.Name+"-", maxRoleName, sha1.Size)
	}

	// Serialize the policy document into a JSON blob.
	policyDocument, err := json.Marshal(obj.AssumeRolePolicyDocument)
	if err != nil {
		return "", nil, err
	}

	// Now go ahead and perform the action.
	fmt.Printf("Creating IAM Role '%v' with name '%v'\n", obj.Name, id)
	var out *iam.RoleOuts
	result, err := p.ctx.IAM().CreateRole(&awsiam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(string(policyDocument)),
		Path:     obj.Path,
		RoleName: id.StringPtr(),
	})
	if err != nil {
		return "", nil, err
	}
	contract.Assert(result != nil)
	out = &iam.RoleOuts{ARN: awscommon.ARN(*result.Role.Arn)}

	// Wait for the role to be ready and then return the ID (just its name).
	fmt.Printf("IAM Role created: %v; waiting for it to become active\n", id)
	if err = p.waitForRoleState(id, true); err != nil {
		return "", nil, err
	}
	return id, out, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *roleProvider) Get(ctx context.Context, id resource.ID) (*iam.Role, error) {
	return nil, errors.New("Not yet implemented")
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *roleProvider) InspectChange(ctx context.Context, id resource.ID,
	old *iam.Role, new *iam.Role, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *roleProvider) Update(ctx context.Context, id resource.ID,
	old *iam.Role, new *iam.Role, diff *resource.ObjectDiff) error {
	contract.Assertf(new.ManagedPolicyARNs == nil, "Managed policies not yet supported")
	contract.Assertf(new.Policies == nil, "Inline policies not yet supported")

	if diff.Changed(iam.Role_AssumeRolePolicyDocument) {
		// Serialize the policy document into a JSON blob.
		policyDocument, err := json.Marshal(new.AssumeRolePolicyDocument)
		if err != nil {
			return err
		}

		// Now go ahead and perform the action.
		fmt.Printf("Creating IAM Role '%v' with name '%v'\n", new.Name, id)
		_, err = p.ctx.IAM().UpdateAssumeRolePolicy(&awsiam.UpdateAssumeRolePolicyInput{
			PolicyDocument: aws.String(string(policyDocument)),
			RoleName:       id.StringPtr(),
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *roleProvider) Delete(ctx context.Context, id resource.ID) error {
	// First, perform the deletion.
	fmt.Printf("Deleting IAM Role '%v'\n", id)
	if _, err := p.ctx.IAM().DeleteRole(&awsiam.DeleteRoleInput{RoleName: id.StringPtr()}); err != nil {
		return err
	}

	// Wait for the role to actually become deleted before the operation is complete.
	fmt.Printf("IAM Role delete request submitted; waiting for it to delete\n")
	return p.waitForRoleState(id, false)
}

func (p *roleProvider) waitForRoleState(id resource.ID, exist bool) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			if _, err := p.ctx.IAM().GetRole(&awsiam.GetRoleInput{RoleName: id.StringPtr()}); err != nil {
				if erraws, iserraws := err.(awserr.Error); iserraws {
					if erraws.Code() == "NotFound" || erraws.Code() == "NoSuchEntity" {
						// The role is missing; if exist==false, we're good, otherwise keep retrying.
						return !exist, nil
					}
				}
				return false, err // anything other than "role missing" is a real error; propagate it.
			}

			// If we got here, the role was found; if exist==true, we're good; else, keep retrying.
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
		return fmt.Errorf("IAM role '%v' did not become %v", id, reason)
	}
	return nil
}
