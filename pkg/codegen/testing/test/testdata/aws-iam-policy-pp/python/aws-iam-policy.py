import pulumi
import json
import pulumi_aws as aws

# Create a policy with multiple Condition keys
policy = aws.iam.Policy("policy",
    path="/",
    description="My test policy",
    policy=json.dumps({
        "Version": "2012-10-17",
        "Statement": [{
            "Effect": "Allow",
            "Principal": "*",
            "Action": ["s3:GetObject"],
            "Resource": ["arn:aws:s3:::some-aws-bucket/*"],
            "Condition": {
                "Foo": {
                    "Bar": [
                        "iamuser-admin",
                        "iamuser2-admin",
                    ],
                },
                "Baz": {
                    "Qux": ["iamuser3-admin"],
                },
            },
        }],
    }))
