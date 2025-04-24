import * as pulumi from "@pulumi/pulumi";
import * as aws_native from "@pulumi/aws-native";

const role = new aws_native.iam.Role("role", {
    roleName: "ScriptIAMRole",
    assumeRolePolicyDocument: {
        Version: "2012-10-17",
        Statement: [{
            Effect: "Allow",
            Action: "sts:AssumeRole",
            Principal: {
                Service: [
                    "cloudformation.amazonaws.com",
                    "gamelift.amazonaws.com",
                ],
            },
        }],
    },
});
