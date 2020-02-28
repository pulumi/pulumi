// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

import * as aws from "@pulumi/aws";
import * as policy from "@pulumi/policy";

const packName = process.env.TEST_POLICY_PACK;

if (!packName) {
    console.log("no policy name provided");
    process.exit(-1);

} else {
    const policies = new policy.PolicyPack(packName, {
        policies: [
            {
                name: "s3-no-public-read",
                description: "Prohibits setting the publicRead or publicReadWrite permission on AWS S3 buckets.",
                enforcementLevel: "mandatory",
                validateResource: policy.validateTypedResource(aws.s3.Bucket, (bucket, args, reportViolation) => {
                    if (bucket.acl === "public-read" || bucket.acl === "public-read-write") {
                        reportViolation(
                            "You cannot set public-read or public-read-write on an S3 bucket. " +
                            "Read more about ACLs here: " +
                            "https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html");
                    }
                }),
            },
        ],
    });
}
