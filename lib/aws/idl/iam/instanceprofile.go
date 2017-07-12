// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package iam

import (
	aws "github.com/pulumi/lumi/lib/aws/idl"
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// InstanceProfile is an AWS Identity and Access Management (IAM) instance profile.  Use an IAM instance profile to
// enable applications running on an EC2 instance to securely access your AWS resources.  For more information about
// IAM instance profiles, see
// http://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html.
type InstanceProfile struct {
	idl.NamedResource
	// Path is the path associated with this instance profile.  For more information about paths, see
	// http://docs.aws.amazon.com/IAM/latest/UserGuide/Using_Identifiers.html#Identifiers_FriendlyNames.
	Path *string `lumi:"path,replaces,optional"`
	// The name of the instance profile that you want to create. This parameter allows a string consisting of upper and
	// lowercase alphanumeric characters with no spaces. You can also include any of the following characters: = , . @ -.
	InstanceProfileName *string `lumi:"instanceProfileName,replaces,optional"`
	// The name of an existing IAM role to associate with this instance profile. Currently, you can assign a maximum
	// of one role to an instance profile.
	Roles []*Role `lumi:"roles"`
	// The Amazon Resource Name (ARN) for the instance profile. For example,
	// `arn:aws:iam::1234567890:instance-profile/MyProfile-ASDNSDLKJ`.
	ARN aws.ARN `lumi:"arn,out"`
}
