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
	"fmt"
	"net/url"

	"github.com/aws/aws-sdk-go/aws"
	awsiam "github.com/aws/aws-sdk-go/service/iam"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	awscommon "github.com/pulumi/lumi/lib/aws/rpc"
	"github.com/pulumi/lumi/lib/aws/rpc/iam"
)

const RoleToken = iam.RoleToken

// constants for the various role limits.
const (
	maxRoleName = 64
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
func (p *roleProvider) Check(ctx context.Context, obj *iam.Role, property string) error {
	// TODO[pulumi/lumi#221]: to use Switch Role, Path+RoleName cannot exceed 64 characters.  Warn?
	return nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *roleProvider) Create(ctx context.Context, obj *iam.Role) (resource.ID, error) {
	contract.Assertf(obj.Policies == nil, "Inline policies not yet supported")

	// A role uses its name as the unique ID, since the GetRole function uses it.  If an explicit name is given, use
	// it directly (at the risk of conflicts).  Otherwise, auto-generate a name in part based on the resource name.
	var name string
	if obj.RoleName != nil {
		name = *obj.RoleName
	} else {
		name = resource.NewUniqueHex(*obj.Name+"-", maxRoleName, sha1.Size)
	}

	// Serialize the policy document into a JSON blob.
	policyDocument, err := json.Marshal(obj.AssumeRolePolicyDocument)
	if err != nil {
		return "", err
	}

	// Now go ahead and perform the action.
	fmt.Printf("Creating IAM Role '%v' with name '%v'\n", *obj.Name, name)
	result, err := p.ctx.IAM().CreateRole(&awsiam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(string(policyDocument)),
		Path:     obj.Path,
		RoleName: aws.String(name),
	})
	if err != nil {
		return "", err
	}
	contract.Assert(result != nil)
	contract.Assert(result.Role != nil)
	contract.Assert(result.Role.Arn != nil)

	if obj.ManagedPolicyARNs != nil {
		for _, policyARN := range *obj.ManagedPolicyARNs {
			if _, atterr := p.ctx.IAM().AttachRolePolicy(&awsiam.AttachRolePolicyInput{
				RoleName:  aws.String(name),
				PolicyArn: aws.String(string(policyARN)),
			}); atterr != nil {
				return "", atterr
			}
		}
	}

	// Wait for the role to be ready and then return the ID (just its name).
	fmt.Printf("IAM Role created: %v; waiting for it to become active\n", name)
	if waiterr := p.waitForRoleState(name, true); waiterr != nil {
		return "", waiterr
	}
	return resource.ID(*result.Role.Arn), nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *roleProvider) Get(ctx context.Context, id resource.ID) (*iam.Role, error) {
	name, err := arn.ParseResourceName(id)
	if err != nil {
		return nil, err
	}
	getrole, err := p.ctx.IAM().GetRole(&awsiam.GetRoleInput{RoleName: aws.String(name)})
	if err != nil {
		if awsctx.IsAWSError(err, "NotFound", "NoSuchEntity") {
			return nil, nil
		}
		return nil, err
	} else if getrole == nil {
		return nil, nil
	}

	// If we got here, we found the role; populate the data structure accordingly.
	role := getrole.Role

	// Policy is a URL-encoded JSON blob, parse it.
	var policyDocument map[string]interface{}
	assumePolicyDocumentJSON, err := url.QueryUnescape(*role.AssumeRolePolicyDocument)
	if err != nil {
		return nil, err
	}
	if jsonerr := json.Unmarshal([]byte(assumePolicyDocumentJSON), &policyDocument); jsonerr != nil {
		return nil, jsonerr
	}

	// Now get a list of attached role policies.
	getpols, err := p.ctx.IAM().ListAttachedRolePolicies(&awsiam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(name),
	})
	if err != nil {
		return nil, err
	}
	var managedPolicies *[]awscommon.ARN
	if len(getpols.AttachedPolicies) > 0 {
		var policies []awscommon.ARN
		for _, policy := range getpols.AttachedPolicies {
			policies = append(policies, awscommon.ARN(aws.StringValue(policy.PolicyArn)))
		}
		managedPolicies = &policies
	}

	return &iam.Role{
		AssumeRolePolicyDocument: policyDocument,
		Path:              role.Path,
		RoleName:          role.RoleName,
		ManagedPolicyARNs: managedPolicies,
		ARN:               awscommon.ARN(aws.StringValue(role.Arn)),
	}, nil
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
	contract.Assertf(new.Policies == nil, "Inline policies not yet supported")
	name, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}

	if diff.Changed(iam.Role_AssumeRolePolicyDocument) {
		// Serialize the policy document into a JSON blob.
		policyDocument, err := json.Marshal(new.AssumeRolePolicyDocument)
		if err != nil {
			return err
		}

		// Now go ahead and perform the action.
		fmt.Printf("Updating IAM Role '%v' with name '%v'\n", new.Name, name)
		_, err = p.ctx.IAM().UpdateAssumeRolePolicy(&awsiam.UpdateAssumeRolePolicyInput{
			PolicyDocument: aws.String(string(policyDocument)),
			RoleName:       aws.String(name),
		})
		if err != nil {
			return err
		}
	}

	if diff.Changed(iam.Role_ManagedPolicyARNs) {
		var detaches []awscommon.ARN
		var attaches []awscommon.ARN
		if diff.Added(iam.Role_ManagedPolicyARNs) {
			for _, policy := range *new.ManagedPolicyARNs {
				attaches = append(attaches, policy)
			}
		}
		if diff.Deleted(iam.Role_ManagedPolicyARNs) {
			for _, policy := range *old.ManagedPolicyARNs {
				detaches = append(detaches, policy)
			}
		}
		if diff.Updated(iam.Role_ManagedPolicyARNs) {
			arrayDiff := diff.Updates[iam.Role_ManagedPolicyARNs].Array
			for i := range arrayDiff.Adds {
				attaches = append(attaches, (*new.ManagedPolicyARNs)[i])
			}
			for i := range arrayDiff.Deletes {
				detaches = append(detaches, (*old.ManagedPolicyARNs)[i])
			}
			for i := range arrayDiff.Updates {
				attaches = append(attaches, (*new.ManagedPolicyARNs)[i])
				detaches = append(detaches, (*old.ManagedPolicyARNs)[i])
			}
		}
		for _, policy := range detaches {
			_, err := p.ctx.IAM().DetachRolePolicy(&awsiam.DetachRolePolicyInput{
				PolicyArn: aws.String(string(policy)),
				RoleName:  aws.String(name),
			})
			if err != nil {
				return err
			}
		}
		for _, policy := range attaches {
			_, err := p.ctx.IAM().AttachRolePolicy(&awsiam.AttachRolePolicyInput{
				PolicyArn: aws.String(string(policy)),
				RoleName:  aws.String(name),
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *roleProvider) Delete(ctx context.Context, id resource.ID) error {
	name, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}

	// Get and detach all attached policies before deleteing
	attachedRolePolicies, err := p.ctx.IAM().ListAttachedRolePolicies(&awsiam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(name),
	})
	if err != nil {
		return err
	}
	if attachedRolePolicies != nil {
		for _, policy := range attachedRolePolicies.AttachedPolicies {
			if _, err := p.ctx.IAM().DetachRolePolicy(&awsiam.DetachRolePolicyInput{
				RoleName:  aws.String(name),
				PolicyArn: policy.PolicyArn,
			}); err != nil {
				return err
			}
		}
	}

	// Perform the deletion.
	fmt.Printf("Deleting IAM Role '%v'\n", name)
	if _, err := p.ctx.IAM().DeleteRole(&awsiam.DeleteRoleInput{RoleName: aws.String(name)}); err != nil {
		return err
	}

	// Wait for the role to actually become deleted before the operation is complete.
	fmt.Printf("IAM Role delete request submitted; waiting for it to delete\n")
	return p.waitForRoleState(name, false)
}

func (p *roleProvider) waitForRoleState(name string, exist bool) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			if _, err := p.ctx.IAM().GetRole(&awsiam.GetRoleInput{RoleName: aws.String(name)}); err != nil {
				if awsctx.IsAWSError(err, "NotFound", "NoSuchEntity") {
					// The role is missing; if exist==false, we're good, otherwise keep retrying.
					return !exist, nil
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
		return fmt.Errorf("IAM role '%v' did not become %v", name, reason)
	}
	return nil
}
