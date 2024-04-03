// Create a policy with multiple Condition keys
resource policy "aws:iam/policy:Policy" {
	__logicalName = "policy"
	path = "/"
	description = "My test policy"
  policy = toJSON({
    "Version" = "2012-10-17",
    "Statement" = [{
      "Effect" = "Allow",
      "Action" = "lambda:*",
      "Resource" = "arn:aws:lambda:*:*:function:*",
      "Condition" = {
        "StringEquals" = {
          "aws:RequestTag/Team" = [
            "iamuser-admin",
            "iamuser2-admin"
          ]
        },
        "ForAllValues:StringEquals" = {
          "aws:TagKeys" = ["Team"]
        }
      }
    }]
	})
}

output policyName { value = policy.name }
