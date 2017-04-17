// Copyright 2017 Pulumi, Inc. All rights reserved.

import {Group} from "./group";
import {InlinePolicy, Policy} from "./policy";
import * as cloudformation from "../cloudformation";

// The User resource creates an AWS Identity and Access Management (IAM) user.
export class User extends cloudformation.Resource implements UserProperties {
    public readonly userName?: string;
    public groups?: Group[];
    public loginProfile?: LoginProfile;
    public managedPolicies?: Policy[];
    public path?: string;
    public policies?: InlinePolicy[];

    constructor(name: string, args: UserProperties) {
        super({
            name: name,
            resource: "AWS::IAM::User",
        });
        this.userName = args.userName;
        this.groups = args.groups;
        this.loginProfile = args.loginProfile;
        this.managedPolicies = args.managedPolicies;
        this.path = args.path;
        this.policies = args.policies;
    }
}

export interface UserProperties extends cloudformation.TagArgs {
    // userName is a name for the IAM group.  If you don't specify a name, a unique physical ID will be generated.
    //
    // Important: if you specify a name, you cannot perform updates that require replacement of this resource.  You can
    // perform updates that require no or some interruption.  If you must replace this resource, specify a new name.
    //
    // If you specify a new name, you must specify the `CAPABILITY_NAMED_IAM` value to acknowledge your capabilities.
    //
    // Warning: Naming an IAM resource can cause an unrecoverable error if you reuse the same code in multiple regions.
    // To prevent this, create a name that includes the region name itself, to create a region-specific name.
    readonly userName?: string;
    // groups is a list of groups to which you want to add the user.
    groups?: Group[];
    // loginProfile creates a login profile so that the user can access the AWS Management Console.
    loginProfile?: LoginProfile;
    // managedPolicies is one or more managed policies to attach to this role.
    managedPolicies?: Policy[];
    // path is the path associated with this role.  For more information about paths, see
    // http://docs.aws.amazon.com/IAM/latest/UserGuide/Using_Identifiers.html#Identifiers_FriendlyNames.
    path?: string;
    // policies are the policies to associate with this role.
    policies?: InlinePolicy[];
}

export interface LoginProfile {
    // password is the password for the user.
    password: string;
    // passwordResetRequired specifies whether the user is required to set a new password the next time the user logs
    // into the AWS Management Console.
    passwordResetRequired?: boolean;
}

