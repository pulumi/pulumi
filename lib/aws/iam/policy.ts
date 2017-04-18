// Copyright 2017 Pulumi, Inc. All rights reserved.

import {Group} from "./group";
import {Role} from "./role";
import {User} from "./user";
import * as cloudformation from "../cloudformation";

// The Policy resource associates an IAM policy with IAM users, roles, or groups.  For more information about IAM
// policies, see http://docs.aws.amazon.com/IAM/latest/UserGuide/policies_overview.html.
export class Policy extends cloudformation.Resource implements PolicyProperties {
    public policyDocument: any;
    public policyName: string;
    public groups?: Group[];
    public roles?: Role[];
    public users?: User[];

    constructor(name: string, args: PolicyProperties) {
        super({
            name: name,
            resource: "AWS::IAM::Policy",
        });
        this.policyDocument = args.policyDocument;
        this.policyName = args.policyName;
        this.groups = args.groups;
        this.roles = args.roles;
        this.users = args.users;
    }
}

export interface PolicyProperties extends cloudformation.TagArgs {
    // policyDocument is a policy document that contains permissions to add to the specified users, roles, or groups.
    policyDocument: any; // TODO: schematize this.
    // policyName is the name of the policy.  If you specify multiple policies for an entity, specify unique names.  For
    // example, if you specify a list of policies for an IAM role, each policy must have a unique name.
    policyName: string;
    // groups are the groups to which you want to add this policy.
    groups?: Group[];
    // roles are the roles to which you want to attach this policy.
    roles?: Role[];
    // users are the users for whom you want to add this policy.
    users?: User[];
}

// InlinePolicies are attached to Policis, Groups, and User resources, to describe what actions are allowed on them.
// For more information on policies, please see http://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies.html.
export interface InlinePolicy {
    // policyDocument is a policy document that describes what actions are allowed on which resources.
    policyDocument: any; // TODO: schematize this.
    // policyName is the unique name of the policy.
    policyName: string;
}

