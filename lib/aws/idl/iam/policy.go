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
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// Policy associates an IAM policy with IAM users, roles, or groups.  For more information about IAM
// policies, see http://docs.aws.amazon.com/IAM/latest/UserGuide/policies_overview.html.
type Policy struct {
	idl.NamedResource
	// PolicyDocument is a policy document that contains permissions to add to the specified users, roles, or groups.
	PolicyDocument interface{} `lumi:"policyDocument"` // IDEA: schematize this.
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
	PolicyDocument interface{} `lumi:"policyDocument"` // IDEA: schematize this.
	// PolicyName is the unique name of the policy.
	PolicyName string `lumi:"policyName"`
}
