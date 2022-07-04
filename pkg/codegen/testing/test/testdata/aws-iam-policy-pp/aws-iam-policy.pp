// Create a policy with multiple Condition keys
resource policy "aws:iam/policy:Policy" {
	__logicalName = "policy"
	path = "/"
	description = "My test policy"
    policy = toJSON({
		Version = "2012-10-17"
		Statement = [{
			Effect = "Allow"
			Principal = "*"
			Action = [ "s3:GetObject" ]
			Resource = [ "arn:aws:s3:::some-aws-bucket/*" ]
            Condition = {
				Foo = {
					Bar: ["iamuser-admin", "iamuser2-admin"]
				},
				Baz: {
					Qux: ["iamuser3-admin"]
				}
			}
		}]
	})
}
