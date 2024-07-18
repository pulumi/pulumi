import pulumi
import pulumi_aws as aws

iam_for_lambda = aws.iam.Role("iamForLambda", assume_role_policy="canBeAString")
test_lambda = aws.lambda_.Function("testLambda",
    code=pulumi.FileArchive("lambda_function_payload.zip"),
    role=iam_for_lambda.arn,
    handler="index.test",
    runtime=aws.lambda_.Runtime.NODE_JS12D_X,
    environment={
        "variables": {
            "foo": "bar",
        },
    })
