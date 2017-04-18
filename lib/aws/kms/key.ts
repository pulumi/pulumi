// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as cloudformation from "../cloudformation";

// The Key resource creates a customer master key (CMK) in AWS Key Management Service (AWS KMS).  Users (customers) can
// use the master key to encrypt their data stored in AWS services that are integrated with AWS KMS or within their
// applications.  For more information, see http://docs.aws.amazon.com/kms/latest/developerguide/.
export class Key extends cloudformation.Resource implements KeyProperties {
    public keyPolicy: any;
    public description?: string;
    public enabled?: boolean;
    public enableKeyRotation?: boolean;

    constructor(name: string, args: KeyProperties) {
        super({
            name: name,
            resource: "AWS::KMS::Key",
        });
        this.keyPolicy = args.keyPolicy;
        this.description = args.description;
        this.enabled = args.enabled;
        this.enableKeyRotation = args.enableKeyRotation;
    }
}

export interface KeyProperties extends cloudformation.TagArgs {
    // keyPolicy attaches a KMS policy to this key.  Use a policy to specify who has permission to use the key and which
    // actions they can perform.  For more information, see
    // http://docs.aws.amazon.com/kms/latest/developerguide/key-policies.html.
    keyPolicy: any; // TODO: map the schema.
    // description is an optional description of the key.  Use a description that helps your users decide whether the
    // key is appropriate for a particular task.
    description?: string;
    // enabled indicates whether the key is available for use.  This value is `true` by default.
    enabled?: boolean;
    // enableKeyRotation indicates whether AWS KMS rotates the key.  This value is `false` by default.
    enableKeyRotation?: boolean;
}

