// Copyright 2017 Pulumi, Inc. All rights reserved.

import {CannedACL} from "./acl";
import * as cloudformation from "../cloudformation";

// Bucket represents an Amazon Simple Storage Service (Amazon S3) bucket.
export class Bucket extends cloudformation.Resource implements BucketProperties {
    public readonly bucketName?: string;
    public accessControl?: CannedACL;

    constructor(name: string, args: BucketProperties) {
        super({
            name: name,
            resource: "AWS::S3::Bucket",
        });
        this.bucketName = args.bucketName;
        this.accessControl = args.accessControl;
    }
}

export interface BucketProperties extends cloudformation.TagArgs {
    // bucketName is a name for the bucket.  If you don't specify a name, a unique physical ID is generated.  The name
    // must contain only lowercase letters, numbers, periods (`.`), and dashes (`-`).
    readonly bucketName?: string;
    // accessControl is a canned access control list (ACL) that grants predefined permissions to the bucket.
    accessControl?: CannedACL;

    // TODO: support all the various configuration settings (CORS, lifecycle, logging, and so on).
}

