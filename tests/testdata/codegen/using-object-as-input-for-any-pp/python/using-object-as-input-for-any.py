import pulumi
import pulumi_aws_native as aws_native

role = aws_native.iam.Role("role",
    role_name="ScriptIAMRole",
    assume_role_policy_document={
        "Version": "2012-10-17",
        "Statement": [{
            "Effect": "Allow",
            "Action": "sts:AssumeRole",
            "Principal": {
                "Service": [
                    "cloudformation.amazonaws.com",
                    "gamelift.amazonaws.com",
                ],
            },
        }],
    })
