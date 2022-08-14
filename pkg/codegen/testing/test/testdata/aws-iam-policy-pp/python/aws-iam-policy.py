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
            "Action": "lambda:*",
            "Resource": "arn:aws:lambda:*:*:function:*",
            "Condition": {
                "StringEquals": {
                    "aws:RequestTag/Team": [
                        "iamuser-admin",
                        "iamuser2-admin",
                    ],
                },
                "ForAllValues:StringEquals": {
                    "aws:TagKeys": ["Team"],
                },
            },
        }],
    }))
pulumi.export("policyName", policy.name)
