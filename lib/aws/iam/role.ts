// Copyright 2017 Pulumi, Inc. All rights reserved.

import {InlinePolicy, Policy} from "./policy";
import * as cloudformation from "../cloudformation";
import {ARN} from "../types";

// Role is an AWS Identity and Access Management (IAM) role.  Use an IAM role to enable applications running on an EC2
// instance to securely access your AWS resources.  For more information about IAM roles, see
// http://docs.aws.amazon.com/IAM/latest/UserGuide/WorkingWithRoles.html.
export class Role extends cloudformation.Resource implements RoleProperties {
    public assumeRolePolicyDocument: any;
    public readonly path?: string;
    public readonly roleName?: string;
    public managedPolicyARNs?: ARN[];
    public policies?: InlinePolicy[];

    constructor(name: string, args: RoleProperties) {
        super({
            name: name,
            resource: "AWS::IAM::Role",
        });
        this.assumeRolePolicyDocument = args.assumeRolePolicyDocument;
        this.path = args.path;
        this.roleName = args.roleName;
        this.managedPolicyARNs = args.managedPolicyARNs;
        this.policies = args.policies;
    }
}

export interface RoleProperties extends cloudformation.TagArgs {
    // assumeRolePolicyDocument is the trust policy associated with this role.
    assumeRolePolicyDocument: any; // TODO: schematize this.
    // path is the path associated with this role.  For more information about paths, see
    // http://docs.aws.amazon.com/IAM/latest/UserGuide/Using_Identifiers.html#Identifiers_FriendlyNames.
    readonly path?: string;
    // roleName is a name for the IAM role.  If you don't specify a name, a unique physical ID will be generated.
    // 
    // Important: If you specify a name, you cannot perform updates that require replacement of this resource.  You can
    // perform updates that require no or some interruption.  If you must replace the resource, specify a new name.
    //
    // If you specify a name, you must specify the `CAPABILITY_NAMED_IAM` value to acknowledge these capabilities.
    //
    // Warning: Naming an IAM resource can cause an unrecoverable error if you reuse the same code in multiple regions.
    // To prevent this, create a name that includes the region name itself, to create a region-specific name.
    readonly roleName?: string;
    // managedPolicies is one or more managed policies to attach to this role.
    managedPolicyARNs?: ARN[];
    // policies are the policies to associate with this role.
    policies?: InlinePolicy[];
}

