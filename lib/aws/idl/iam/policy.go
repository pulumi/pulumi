// Copyright 2017 Pulumi, Inc. All rights reserved.

package iam

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// Policy associates an IAM policy with IAM users, roles, or groups.  For more information about IAM
// policies, see http://docs.aws.amazon.com/IAM/latest/UserGuide/policies_overview.html.
type Policy struct {
	idl.NamedResource
	// PolicyDocument is a policy document that contains permissions to add to the specified users, roles, or groups.
	PolicyDocument interface{} `lumi:"policyDocument"` // TODO: schematize this.
	// PolicyName is the name of the policy.  If you specify multiple policies for an entity, specify unique names.  For
	// example, if you specify a list of policies for an IAM role, each policy must have a unique name.
	PolicyName string `lumi:"policyName"`
	// Groups are the groups to which you want to add this policy.
	Groups *[]*Group `lumi:"groups,optional"`
	// Roles are the roles to which you want to attach this policy.
	Roles *[]*Role `lumi:"roles,optional"`
	// Users are the users for whom you want to add this policy.
	Users *[]*User `lumi:"users,optional"`
}

// InlinePolicies are attached to Policies, Groups, and User resources, to describe what actions are allowed on them.
// For more information on policies, please see http://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies.html.
type InlinePolicy struct {
	// PolicyDocument is a policy document that describes what actions are allowed on which resources.
	PolicyDocument interface{} `lumi:"policyDocument"` // TODO: schematize this.
	// PolicyName is the unique name of the policy.
	PolicyName string `lumi:"policyName"`
}
