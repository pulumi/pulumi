// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package iam

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// The User resource creates an AWS Identity and Access Management (IAM) user.
type User struct {
	idl.NamedResource
	// UserName is a name for the IAM group.  If you don't specify a name, a unique physical ID will be generated.
	//
	// Important: if you specify a name, you cannot perform updates that require replacement of this resource.  You can
	// perform updates that require no or some interruption.  If you must replace this resource, specify a new name.
	//
	// If you specify a new name, you must specify the `CAPABILITY_NAMED_IAM` value to acknowledge your capabilities.
	//
	// Warning: Naming an IAM resource can cause an unrecoverable error if you reuse the same code in multiple regions.
	// To prevent this, create a name that includes the region name itself, to create a region-specific name.
	UserName *string `lumi:"userName,replaces,optional"`
	// Groups is a list of groups to which you want to add the user.
	Groups *[]*Group `lumi:"groups,optional"`
	// LoginProfile creates a login profile so that the user can access the AWS Management Console.
	LoginProfile *LoginProfile `lumi:"loginProfile,optional"`
	// ManagedPolicies is one or more managed policies to attach to this role.
	ManagedPolicies *[]*Policy `lumi:"managedPolicies,optional"`
	// Path is the path associated with this role.  For more information about paths, see
	// http://docs.aws.amazon.com/IAM/latest/UserGuide/Using_Identifiers.html#Identifiers_FriendlyNames.
	Path *string `lumi:"path,optional"`
	// Policies are the policies to associate with this role.
	Policies *[]InlinePolicy `lumi:"policies,optional"`
}

type LoginProfile struct {
	// Password is the password for the user.
	Password string `lumi:"password"`
	// PasswordResetRequired specifies whether the user is required to set a new password the next time the user logs
	// into the AWS Management Console.
	PasswordResetRequired *bool `lumi:"passwordResetRequired,optional"`
}
