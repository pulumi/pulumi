import pulumi
import pulumi_aws as aws
import pulumi_external_type_ref as external_type_ref

main = external_type_ref.ExampleComponent("main", external_input=aws.s3.BucketWebsiteArgs(
    index_document="index.html",
))
