// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package iam

import (
	"crypto/sha1"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awsiam "github.com/aws/aws-sdk-go/service/iam"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/iam"
)

const InstanceProfileToken = iam.InstanceProfileToken

// constants for the various InstanceProfile limits.
const (
	maxInstanceProfileName = 64
)

// NewInstanceProfileProvider creates a provider that handles IAM InstanceProfile operations.
func NewInstanceProfileProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &InstanceProfileProvider{ctx}
	return iam.NewInstanceProfileProvider(ops)
}

type InstanceProfileProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *InstanceProfileProvider) Check(ctx context.Context, obj *iam.InstanceProfile, property string) error {
	// TODO[pulumi/lumi#221]: to use Switch InstanceProfile, Path+InstanceProfileName cannot exceed 64 characters.  Warn?
	return nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *InstanceProfileProvider) Create(ctx context.Context, obj *iam.InstanceProfile) (resource.ID, error) {
	// A InstanceProfile uses its name as the unique ID, since the GetInstanceProfile function uses it.  If an explicit
	// name is given, use it directly (at the risk of conflicts).  Otherwise, auto-generate a name in part based on the
	// resource name.
	var name string
	if obj.InstanceProfileName != nil {
		name = *obj.InstanceProfileName
	} else {
		name = resource.NewUniqueHex(*obj.Name+"-", maxInstanceProfileName, sha1.Size)
	}

	// Now go ahead and perform the action.
	fmt.Printf("Creating IAM InstanceProfile '%v' with name '%v'\n", *obj.Name, name)
	result, err := p.ctx.IAM().CreateInstanceProfile(&awsiam.CreateInstanceProfileInput{
		Path:                obj.Path,
		InstanceProfileName: aws.String(name),
	})
	if err != nil {
		return "", err
	}
	contract.Assert(result != nil)
	contract.Assert(result.InstanceProfile != nil)
	contract.Assert(result.InstanceProfile.Arn != nil)

	err = p.waitForInstanceProfileState(name, true)
	if err != nil {
		return "", err
	}

	for _, role := range obj.Roles {
		roleName, err := arn.ParseResourceName(role)
		if err != nil {
			return "", err
		}
		_, err = p.ctx.IAM().AddRoleToInstanceProfile(&awsiam.AddRoleToInstanceProfileInput{
			InstanceProfileName: aws.String(name),
			RoleName:            aws.String(roleName),
		})
		if err != nil {
			return "", err
		}
	}

	return resource.ID(*result.InstanceProfile.Arn), nil
}

// Query returns an (possibly empty) array of resource objects.
func (p *InstanceProfileProvider) Query(ctx context.Context) ([]*iam.InstanceProfileItem, error) {
	return nil, nil
}

/*
	instProfs, err := p.ctx.IAM().ListInstanceProfiles(&awsiam.ListInstanceProfilesInput{})
	if err != nil {
		return nil, err
	}
	var instanceProfiles []*iam.InstanceProfile

	for _, inst := range instProfs.InstanceProfiles {
		if inst == nil {
			return nil, nil
		}
		var roles []resource.ID
		for _, role := range inst.Roles {
			roles = append(roles, resource.ID(*role.Arn))
		}
		instanceProfiles = append(instanceProfiles, &iam.InstanceProfile{
			Path:                inst.Path,
			InstanceProfileName: inst.InstanceProfileName,
			Roles:               roles,
			ARN:                 awscommon.ARN(aws.StringValue(inst.Arn)),
		})
	}
	return instanceProfiles, nil
}
*/

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *InstanceProfileProvider) Get(ctx context.Context, id resource.ID) (*iam.InstanceProfile, error) {
	/*
			queresp, err := p.Query(ctx)
			if err != nil {
				return nil, err
			}
			name, err := arn.ParseResourceName(id)
			for _, instProf := range queresp {
				if *instProf.InstanceProfileName == name {
					return instProf, nil
				} // Return 'resource not found' error
			}
			return nil, errors.New("No resource found with matching ID")
		}
	*/
	/*
		name, err := arn.ParseResourceName(id)
		if err != nil {
			return nil, err
		}
		getInstanceProfile, err := p.ctx.IAM().GetInstanceProfile(&awsiam.GetInstanceProfileInput{
			InstanceProfileName: aws.String(name),
		})
		if err != nil {
			if awsctx.IsAWSError(err, "NotFound", "NoSuchEntity") {
				return nil, nil
			}
			return nil, err
		} else if getInstanceProfile == nil {
			return nil, nil
		}
		// If we got here, we found the InstanceProfile; populate the data structure accordingly.
		instanceProfile := getInstanceProfile.InstanceProfile
		var roles []resource.ID
		for _, role := range instanceProfile.Roles {
			roles = append(roles, resource.ID(*role.Arn))
		}

		return &iam.InstanceProfile{
			Path:                instanceProfile.Path,
			InstanceProfileName: instanceProfile.InstanceProfileName,
			Roles:               roles,
			ARN:                 awscommon.ARN(aws.StringValue(instanceProfile.Arn)),
		}, nil
	*/
	return nil, nil
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *InstanceProfileProvider) InspectChange(ctx context.Context, id resource.ID,
	old *iam.InstanceProfile, new *iam.InstanceProfile, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *InstanceProfileProvider) Update(ctx context.Context, id resource.ID,
	old *iam.InstanceProfile, new *iam.InstanceProfile, diff *resource.ObjectDiff) error {
	name, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}

	if diff.Changed(iam.InstanceProfile_Roles) {
		var removes []resource.ID
		var adds []resource.ID
		if diff.Added(iam.InstanceProfile_Roles) {
			adds = append(adds, new.Roles...)
		}
		if diff.Deleted(iam.InstanceProfile_Roles) {
			removes = append(removes, old.Roles...)
		}
		if diff.Updated(iam.InstanceProfile_Roles) {
			arrayDiff := diff.Updates[iam.InstanceProfile_Roles].Array
			for i := range arrayDiff.Adds {
				adds = append(adds, new.Roles[i])
			}
			for i := range arrayDiff.Deletes {
				removes = append(removes, old.Roles[i])
			}
			for i := range arrayDiff.Updates {
				adds = append(adds, new.Roles[i])
				removes = append(removes, old.Roles[i])
			}
		}
		for _, role := range removes {
			_, err := p.ctx.IAM().RemoveRoleFromInstanceProfile(&awsiam.RemoveRoleFromInstanceProfileInput{
				RoleName:            aws.String(string(role)),
				InstanceProfileName: aws.String(name),
			})
			if err != nil {
				return err
			}
		}
		for _, role := range adds {
			_, err := p.ctx.IAM().AddRoleToInstanceProfile(&awsiam.AddRoleToInstanceProfileInput{
				RoleName:            aws.String(string(role)),
				InstanceProfileName: aws.String(name),
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *InstanceProfileProvider) Delete(ctx context.Context, id resource.ID) error {
	name, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}

	// Remove the roles associated with this instance profile.
	result, err := p.ctx.IAM().GetInstanceProfile(&awsiam.GetInstanceProfileInput{
		InstanceProfileName: aws.String(name),
	})
	if err != nil {
		return err
	}
	contract.Assert(result != nil)
	contract.Assert(result.InstanceProfile != nil)
	for _, role := range result.InstanceProfile.Roles {
		_, err := p.ctx.IAM().RemoveRoleFromInstanceProfile(&awsiam.RemoveRoleFromInstanceProfileInput{
			InstanceProfileName: aws.String(name),
			RoleName:            role.RoleName,
		})
		if err != nil {
			return err
		}
	}

	// Perform the deletion.
	fmt.Printf("Deleting IAM InstanceProfile '%v'\n", name)
	if _, err := p.ctx.IAM().DeleteInstanceProfile(&awsiam.DeleteInstanceProfileInput{
		InstanceProfileName: aws.String(name),
	}); err != nil {
		return err
	}

	// Wait for the InstanceProfile to actually become deleted before the operation is complete.
	fmt.Printf("IAM InstanceProfile delete request submitted; waiting for it to delete\n")
	return p.waitForInstanceProfileState(name, false)
}

func (p *InstanceProfileProvider) waitForInstanceProfileState(name string, exist bool) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			if _, err := p.ctx.IAM().GetInstanceProfile(&awsiam.GetInstanceProfileInput{
				InstanceProfileName: aws.String(name),
			}); err != nil {
				if awsctx.IsAWSError(err, "NotFound", "NoSuchEntity") {
					// The InstanceProfile is missing; if exist==false, we're good, otherwise keep retrying.
					return !exist, nil
				}
				return false, err // anything other than "InstanceProfile missing" is a real error; propagate it.
			}

			// If we got here, the InstanceProfile was found; if exist==true, we're good; else, keep retrying.
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
		return fmt.Errorf("IAM InstanceProfile '%v' did not become %v", name, reason)
	}
	return nil
}
