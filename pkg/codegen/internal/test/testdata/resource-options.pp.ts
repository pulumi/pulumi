import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const provider = new aws.Provider("bucket", {region: "us-west-2"});
const bucket1 = new aws.s3.Bucket("bucket", {}, {
    provider: provider,
    dependsOn: [provider],
    protect: true,
    ignoreChanges: [
        "bucket",
        "lifecycleRules[0]",
    ],
});
