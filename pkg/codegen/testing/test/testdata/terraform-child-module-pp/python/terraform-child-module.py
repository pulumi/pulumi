import pulumi
import modules.aws_s3_static_website_bucket as aws_s3_static_website_bucket
import pulumi_aws as aws

class IndexArgs:
    def __init__(
        self,
    ):
        pass

class Index(ComponentResource):
    def __init__(name: string, args: IndexArgs, opts: ResourceOptions = None):
        super().__init__("pkg:Index:component", name, {}, opts)

        # Local Terraform Child Module
        s3_bucket = aws.s3.Bucket("s3Bucket", bucket="s3BucketId")
        child_module = aws_s3_static_website_bucket.Index("childModule", custom_module_parameter=s3_bucket.id)
