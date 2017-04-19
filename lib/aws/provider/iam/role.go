// Copyright 2017 Pulumi, Inc. All rights reserved.

package iam

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"
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

const Role = tokens.Type("aws:iam/role:Role")

// NewRoleProvider creates a provider that handles IAM role operations.
func NewRoleProvider(ctx *awsctx.Context) cocorpc.ResourceProviderServer {
	return &roleProvider{ctx}
}

type roleProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *roleProvider) Check(ctx context.Context, req *cocorpc.CheckRequest) (*cocorpc.CheckResponse, error) {
	// Read in the properties, create and validate a new group, and return the failures (if any).
	contract.Assert(req.GetType() == string(Role))
	_, _, result := unmarshalRole(req.GetProperties())
	return resource.NewCheckResponse(result), nil
}

// Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
// simply fetches it from the property bag; other times, the provider will assign this based on its own algorithm.
// In any case, resources with the same name must be safe to use interchangeably with one another.
func (p *roleProvider) Name(ctx context.Context, req *cocorpc.NameRequest) (*cocorpc.NameResponse, error) {
	return nil, nil // use the AWS provider default name
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *roleProvider) Create(ctx context.Context, req *cocorpc.CreateRequest) (*cocorpc.CreateResponse, error) {
	contract.Assert(req.GetType() == string(Role))

	// Read in the properties given by the request, validating as we go; if any fail, reject the request.
	r, _, decerr := unmarshalRole(req.GetProperties())
	if decerr != nil {
		// TODO: this is a good example of a "benign" (StateOK) error; handle it accordingly.
		return nil, decerr
	}
	contract.Assertf(r.ManagedPolicies == nil, "Managed policies not yet supported")
	contract.Assertf(r.Policies == nil, "Inline policies not yet supported")

	// If an explicit name is given, use it.  Otherwise, auto-generate a name in part based on the resource name.
	// TODO: use the URN, not just the name, to enhance global uniqueness.
	// TODO: even for explicit names, we should consider mangling it somehow, to reduce multi-instancing conflicts.
	var id string
	if r.RoleName != nil {
		id = *r.RoleName
	} else {
		id = resource.NewUniqueHex(r.Name+"-", maxRoleName, sha1.Size)
	}

	// Serialize the policy document into a JSON blob.
	policyDocument, err := json.Marshal(r.PolicyDocument)
	if err != nil {
		return nil, err
	}

	// Now go ahead and perform the action.
	fmt.Printf("Creating IAM Role '%v' with name '%v'\n", r.Name, id)
	result, err := p.ctx.IAM().CreateRole(&iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(string(policyDocument)),
		Path:     r.Path,
		RoleName: aws.String(id),
	})
	if err != nil {
		return nil, err
	}
	contract.Assert(result != nil)
	fmt.Printf("IAM Role created: %v; waiting for it to become active\n", id)

	// Wait for the role to be ready and then return the ID (just its name).
	if err = p.waitForRoleState(&id, true); err != nil {
		return nil, err
	}
	return &cocorpc.CreateResponse{Id: id}, nil
}

// Read reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *roleProvider) Read(ctx context.Context, req *cocorpc.ReadRequest) (*cocorpc.ReadResponse, error) {
	contract.Assert(req.GetType() == string(Role))
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *roleProvider) Update(ctx context.Context, req *cocorpc.UpdateRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(Role))
	return nil, errors.New("Not yet implemented")
}

// UpdateImpact checks what impacts a hypothetical update will have on the resource's properties.
func (p *roleProvider) UpdateImpact(
	ctx context.Context, req *cocorpc.UpdateRequest) (*cocorpc.UpdateImpactResponse, error) {
	contract.Assert(req.GetType() == string(Role))
	return nil, errors.New("Not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *roleProvider) Delete(ctx context.Context, req *cocorpc.DeleteRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(Role))

	// First, perform the deletion.
	id := aws.String(req.GetId())
	fmt.Printf("Deleting IAM Role '%v'\n", *id)
	if _, err := p.ctx.IAM().DeleteRole(&iam.DeleteRoleInput{
		RoleName: id,
	}); err != nil {
		return nil, err
	}

	fmt.Printf("IAM Role delete request submitted; waiting for it to delete\n")

	// Wait for the role to actually become deleted before returning.
	if err := p.waitForRoleState(id, false); err != nil {
		return nil, err
	}
	return &pbempty.Empty{}, nil
}

// role represents the state associated with an AWS IAM Role.
type role struct {
	Name            string                 `json:"name"`                      // the role resource's name.
	PolicyDocument  map[string]interface{} `json:"assumeRolePolicyDocument"`  // the trust policy for this role.
	Path            *string                `json:"path,omitempty"`            // the path associated with this role.
	RoleName        *string                `json:"roleName,omitempty"`        // the role's published name.
	ManagedPolicies *[]string              `json:"managedPolicies,omitempty"` // one or more managed policies to attach.
	Policies        *[]inlinePolicy        `json:"policies,omitempty"`        // policies to associate with this role.
}

// constants for role property names.
const (
	RoleName     = "name"
	RoleRoleName = "roleName"
)

// constants for the various role limits.
const (
	maxRoleName = 64 // TODO: to use Switch Role, Path+RoleName cannot exceed 64 characters.  Warn?
)

// unmarshalRole decodes and validates a role property bag.
func unmarshalRole(v *pbstruct.Struct) (role, resource.PropertyMap, mapper.DecodeError) {
	var r role
	props := resource.UnmarshalProperties(v)
	result := mapper.MapIU(props.Mappable(), &r)
	if name := r.RoleName; name != nil {
		if len(*name) > maxRoleName {
			if result == nil {
				result = mapper.NewDecodeErr(nil)
			}
			result.AddFailure(
				mapper.NewFieldErr(reflect.TypeOf(r), RoleRoleName,
					fmt.Errorf("exceeded maximum length of %v", maxRoleName)),
			)
		}
	}
	// TODO: by default, only up to 100 roles in an account.
	// TODO: check the vailidity of names (see http://docs.aws.amazon.com/AmazonIAM/latest/dev/RoleRestrictions.html).
	return r, props, result
}

func (p *roleProvider) waitForRoleState(name *string, exist bool) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			if _, err := p.ctx.IAM().GetRole(&iam.GetRoleInput{
				RoleName: name,
			}); err != nil {
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
		return fmt.Errorf("IAM role '%v' did not become %v", *name, reason)
	}
	return nil
}
