import pulumi
import pulumi_aws as aws

bucket = []
for range in [{"value": i} for i in range(0, 10)]:
    bucket.append(aws.s3.Bucket(f"bucket-{range['value']}", website=aws.s3.BucketWebsiteArgs(
        index_document=f"index-{range['value']}.html",
    )))
