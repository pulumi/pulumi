// Copyright 2017 Pulumi, Inc. All rights reserved.

package iam

import (
	aws "github.com/pulumi/lumi/lib/aws/idl"
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// Role is an AWS Identity and Access Management (IAM) role.  Use an IAM role to enable applications running on an EC2
// instance to securely access your AWS resources.  For more information about IAM roles, see
// http://docs.aws.amazon.com/IAM/latest/UserGuide/WorkingWithRoles.html.
type Role struct {
	idl.NamedResource
	// AssumeRolePolicyDocument is the trust policy associated with this role.
	AssumeRolePolicyDocument interface{} `lumi:"assumeRolePolicyDocument"` // TODO: schematize this.
	// Path is the path associated with this role.  For more information about paths, see
	// http://docs.aws.amazon.com/IAM/latest/UserGuide/Using_Identifiers.html#Identifiers_FriendlyNames.
	Path *string `lumi:"path,replaces,optional"`
	// RoleName is a name for the IAM role.  If you don't specify a name, a unique physical ID will be generated.
	//
	// Important: If you specify a name, you cannot perform updates that require replacement of this resource.  You can
	// perform updates that require no or some interruption.  If you must replace the resource, specify a new name.
	//
	// If you specify a name, you must specify the `CAPABILITY_NAMED_IAM` value to acknowledge these capabilities.
	//
	// Warning: Naming an IAM resource can cause an unrecoverable error if you reuse the same code in multiple regions.
	// To prevent this, create a name that includes the region name itself, to create a region-specific name.
	RoleName *string `lumi:"roleName,replaces,optional"`
	// managedPolicies is one or more managed policies to attach to this role.
	ManagedPolicyARNs *[]aws.ARN `lumi:"managedPolicyARNs,optional"`
	// Policies are the policies to associate with this role.
	Policies *[]InlinePolicy `lumi:"policies,optional"`
	// The Amazon Resource Name (ARN) for the instance profile.  For example,
	// `arn:aws:iam::1234567890:role/MyRole-AJJHDSKSDF`.
	ARN aws.ARN `lumi:"arn,out"`
}
