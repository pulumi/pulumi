// Copyright 2017 Pulumi, Inc. All rights reserved.

package iam

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/pulumi/coconut/pkg/resource"
	"github.com/pulumi/coconut/pkg/util/contract"
	"github.com/pulumi/coconut/pkg/util/mapper"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
	"golang.org/x/net/context"

	"github.com/pulumi/coconut/lib/aws/provider/awsctx"
	awsrpc "github.com/pulumi/coconut/lib/aws/rpc"
	rpc "github.com/pulumi/coconut/lib/aws/rpc/iam"
)

const RoleToken = rpc.RoleToken

// constants for the various role limits.
const (
	maxRoleName = 64 // TODO: to use Switch Role, Path+RoleName cannot exceed 64 characters.  Warn?
)

// NewRoleProvider creates a provider that handles IAM role operations.
func NewRoleProvider(ctx *awsctx.Context) cocorpc.ResourceProviderServer {
	ops := &roleProvider{ctx}
	return rpc.NewRoleProvider(ops)
}

type roleProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *roleProvider) Check(ctx context.Context, obj *rpc.Role) ([]mapper.FieldError, error) {
	return nil, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *roleProvider) Create(ctx context.Context, obj *rpc.Role) (string, *rpc.RoleOuts, error) {
	contract.Assertf(obj.ManagedPolicyARNs == nil, "Managed policies not yet supported")
	contract.Assertf(obj.Policies == nil, "Inline policies not yet supported")

	// If an explicit name is given, use it.  Otherwise, auto-generate a name in part based on the resource name.
	// TODO: use the URN, not just the name, to enhance global uniqueness.
	// TODO: even for explicit names, we should consider mangling it somehow, to reduce multi-instancing conflicts.
	var name string
	if obj.RoleName != nil {
		name = *obj.RoleName
	} else {
		name = resource.NewUniqueHex(obj.Name+"-", maxRoleName, sha1.Size)
	}

	// Serialize the policy document into a JSON blob.
	policyDocument, err := json.Marshal(obj.AssumeRolePolicyDocument)
	if err != nil {
		return "", nil, err
	}

	// Now go ahead and perform the action.
	fmt.Printf("Creating IAM Role '%v' with name '%v'\n", obj.Name, name)
	var out *rpc.RoleOuts
	if result, err := p.ctx.IAM().CreateRole(&iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(string(policyDocument)),
		Path:     obj.Path,
		RoleName: aws.String(name),
	}); err != nil {
		return "", nil, err
	} else {
		contract.Assert(result != nil)
		out.ARN = awsrpc.ARN(*result.Role.Arn)
	}

	// Wait for the role to be ready and then return the ID (just its name).
	fmt.Printf("IAM Role created: %v; waiting for it to become active\n", name)
	if err = p.waitForRoleState(name, true); err != nil {
		return "", nil, err
	}
	return name, out, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *roleProvider) Get(ctx context.Context, id string) (*rpc.Role, error) {
	return nil, errors.New("Not yet implemented")
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *roleProvider) InspectChange(ctx context.Context, id string,
	old *rpc.Role, new *rpc.Role, diff *resource.ObjectDiff) ([]string, error) {
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *roleProvider) Update(ctx context.Context, id string,
	old *rpc.Role, new *rpc.Role, diff *resource.ObjectDiff) error {
	return errors.New("Not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *roleProvider) Delete(ctx context.Context, id string) error {
	// First, perform the deletion.
	fmt.Printf("Deleting IAM Role '%v'\n", id)
	if _, err := p.ctx.IAM().DeleteRole(&iam.DeleteRoleInput{RoleName: aws.String(id)}); err != nil {
		return err
	}

	// Wait for the role to actually become deleted before the operation is complete.
	fmt.Printf("IAM Role delete request submitted; waiting for it to delete\n")
	return p.waitForRoleState(id, false)
}

func (p *roleProvider) waitForRoleState(name string, exist bool) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			if _, err := p.ctx.IAM().GetRole(&iam.GetRoleInput{RoleName: aws.String(name)}); err != nil {
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
		return fmt.Errorf("IAM role '%v' did not become %v", name, reason)
	}
	return nil
}
