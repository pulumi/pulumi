import pulumi
import pulumi_aws as aws

iam_for_lambda = aws.iam.Role("iamForLambda")
test_lambda = aws.lambda_.Function("testLambda",
    code=pulumi.FileArchive("lambda_function_payload.zip"),
    role=iam_for_lambda.arn,
    handler="index.test",
    runtime="nodejs12.x",
    environment=aws.lambda..FunctionEnvironmentArgs(
        variables={
            "foo": "bar",
        },
    ))
