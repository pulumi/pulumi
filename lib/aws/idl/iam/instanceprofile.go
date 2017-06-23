// Copyright 2016-2017, Pulumi Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
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
