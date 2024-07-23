import pulumi
import pulumi_aws as aws

mybucket = aws.s3.Bucket("mybucket", website={
    "index_document": "index.html",
})
indexhtml = aws.s3.BucketObject("indexhtml",
    bucket=mybucket.id,
    source=pulumi.StringAsset("<h1>Hello, world!</h1>"),
    acl="public-read",
    content_type="text/html")
pulumi.export("bucketEndpoint", mybucket.website_endpoint.apply(lambda website_endpoint: f"http://{website_endpoint}"))
