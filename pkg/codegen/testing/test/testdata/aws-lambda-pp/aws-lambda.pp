resource iamForLambda "aws:iam:Role" {
    assumeRolePolicy = "canBeAString"
}

resource testLambda "aws:lambda:Function" {
    code = fileArchive("lambda_function_payload.zip")
    role = iamForLambda.arn
    handler = "index.test"
    runtime = "nodejs12.x"
    environment = {
        variables = {
          "foo" = "bar"
        }
    }
}
