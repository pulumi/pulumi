import pulumi
import pulumi_aws as aws
import pulumi_pulumi as pulumi

provider = pulumi.providers.Aws("provider", region="us-west-2")
bucket1 = aws.s3.Bucket("bucket1", opts=ResourceOptions(provider=provider,
    dependsOn=[provider],
    protect=True,
    ignoreChanges=[
        "bucket",
        "lifecycleRules[0]",
    ]))
