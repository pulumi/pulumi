// Copyright 2017 Pulumi, Inc. All rights reserved.

package iam

// inlinePolicy represents a policy attached to a Policy, Group, and/or User resource.
type inlinePolicy struct {
	policyDocument map[string]interface{} `json:"policyDocument"` // a description of which actions are allowed.
	policyName     string                 `json:"policyName"`     // the unique name of this policy.
}
