import pulumi
import pulumi_aws as aws

logs = aws.s3.Bucket("logs")
bucket = aws.s3.Bucket("bucket", loggings=[aws.s3.BucketLoggingArgs(
    target_bucket=logs.bucket,
)])
pulumi.export("targetBucket", bucket.loggings[0].target_bucket)
