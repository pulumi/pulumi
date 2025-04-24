resource "role" "aws-native:iam:Role" {
    roleName = "ScriptIAMRole"
    assumeRolePolicyDocument = {
        Version = "2012-10-17",
        Statement = [
            {
                Effect = "Allow",
                Action = "sts:AssumeRole",
                Principal = {
                    Service = ["cloudformation.amazonaws.com", "gamelift.amazonaws.com"]
                }
            }
        ]
    }
}