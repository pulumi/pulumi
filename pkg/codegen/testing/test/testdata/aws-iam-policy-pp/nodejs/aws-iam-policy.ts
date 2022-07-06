import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

// Create a policy with multiple Condition keys
const policy = new aws.iam.Policy("policy", {
    path: "/",
    description: "My test policy",
    policy: JSON.stringify({
        Version: "2012-10-17",
        Statement: [{
            Effect: "Allow",
            Principal: "*",
            Action: ["s3:GetObject"],
            Resource: ["arn:aws:s3:::some-aws-bucket/*"],
            Condition: {
                Foo: {
                    Bar: [
                        "iamuser-admin",
                        "iamuser2-admin",
                    ],
                },
                Baz: {
                    Qux: ["iamuser3-admin"],
                },
            },
        }],
    }),
});
